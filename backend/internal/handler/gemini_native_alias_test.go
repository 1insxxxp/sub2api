//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	geminiNativeAlias       = "按量vet/gemini-4-argon"
	geminiNativeAliasTarget = "gemini-4-argon"
)

// This transport has no network implementation: even a malformed path cannot
// reach a provider. Capture the actual request built by the real Gemini adapter.
type geminiNativeAliasUpstream struct {
	service.HTTPUpstream
	requests []*http.Request
	bodies   [][]byte
	accounts []int64
}

func (u *geminiNativeAliasUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.requests = append(u.requests, req.Clone(context.Background()))
	u.bodies = append(u.bodies, body)
	u.accounts = append(u.accounts, accountID)
	header := http.Header{"Content-Type": {"application/json"}, "X-Goog-Request-Id": {"native-alias-request"}}
	response := `{"candidates":[{"content":{"role":"model","parts":[{"text":"local test answer"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":7,"totalTokenCount":19}}`
	if strings.HasSuffix(req.URL.Path, ":countTokens") {
		response = `{"totalTokens":12}`
	} else if strings.HasSuffix(req.URL.Path, ":streamGenerateContent") {
		header.Set("Content-Type", "text/event-stream")
		response = "data: " + response + "\n\n"
	}
	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(response))}, nil
}

type geminiNativeAliasCustomRepo struct {
	service.UserCustomGroupRepository
	groupID, userID int64
	route           service.UserCustomGroupModel
}

func (r *geminiNativeAliasCustomRepo) ResolveModel(_ context.Context, groupID, userID int64, model string) (*service.UserCustomGroupModel, error) {
	if groupID != r.groupID || userID != r.userID || model != r.route.PublicModel {
		return nil, service.ErrUserCustomGroupNotFound
	}
	route := r.route
	return &route, nil
}

type geminiNativeAliasFixture struct {
	router   *gin.Engine
	upstream *geminiNativeAliasUpstream
	usage    *geminiMeasuredUsageRepo
	wallet   *geminiPartialBillingRepo
	key      *service.APIKey
	account  *service.Account
}

// routeKind exercises the handler boundary after user-custom resolution, an
// ordinary source-group alias, or a direct account alias. The actual routing,
// forwarding, pricing and usage recording remain production implementations.
func newGeminiNativeAliasFixture(t *testing.T, routeKind, publicModel, sourceModel, mappedModel string) *geminiNativeAliasFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID, customGroupID := int64(9850), int64(9854)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformGemini,
		Status: service.StatusActive, RateMultiplier: 1}
	if routeKind == "group" {
		group.ModelAliases = []domain.ModelAlias{{Alias: publicModel, UpstreamModel: sourceModel}}
	}
	account := &service.Account{
		ID: 9851, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "local-alias-test-only", "base_url": "https://native-alias.example.invalid",
			"model_mapping": map[string]any{sourceModel: mappedModel}},
		AccountGroups: []service.AccountGroup{{AccountID: 9851, GroupID: groupID}},
	}
	key := &service.APIKey{ID: 9852, UserID: 9853, GroupID: &groupID, Group: group, Status: service.StatusActive,
		User: &service.User{ID: 9853, Concurrency: 10, Balance: 100}}
	if routeKind == "custom" {
		key.CustomGroupID = &customGroupID
		key.GroupID, key.Group = nil, nil
	}
	upstream := &geminiNativeAliasUpstream{}
	usage := &geminiMeasuredUsageRepo{}
	wallet := &geminiPartialBillingRepo{}
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	cfg.Pricing.DataDir = t.TempDir()
	cfg.Pricing.UpdateIntervalHours = 100000
	prices := map[string]any{geminiNativeAliasTarget: map[string]any{
		"litellm_provider": "gemini", "mode": "chat", "input_cost_per_token": 0.000001, "output_cost_per_token": 0.00001,
	}}
	priceJSON, err := json.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Pricing.DataDir, "model_pricing.json"), priceJSON, 0o600))
	pricing := service.NewPricingService(cfg, nil)
	require.NoError(t, pricing.Initialize())
	t.Cleanup(pricing.Stop)
	userRepo := &cachedBillingUserRepo{balance: 100}
	billing := service.NewBillingCacheService(nil, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	groups := &fakeGroupRepo{group: group}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	gateway := service.NewGatewayService(
		nil, groups, usage, wallet, userRepo, nil, nil, nil, cfg,
		snapshot, nil, service.NewBillingService(cfg, pricing), nil, billing, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil,
		&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil,
	)
	handler := &GatewayHandler{
		gatewayService: gateway, billingCacheService: billing, cfg: cfg, maxAccountSwitchesGemini: 0,
		geminiCompatService: service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, upstream, nil, cfg, nil),
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
	}
	customRepo := &geminiNativeAliasCustomRepo{groupID: customGroupID, userID: key.UserID, route: service.UserCustomGroupModel{
		PublicModel: publicModel, SourceModel: sourceModel, SourceGroupID: groupID,
	}}
	resolver := service.ProvideAPIKeyService(nil, nil, groups, customRepo, nil, nil, nil, nil, cfg, nil, nil)
	router := gin.New()
	router.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		requestKey := key
		ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
		if routeKind == "custom" {
			model, action, parseErr := parseGeminiModelAction(strings.TrimPrefix(c.Param("modelAction"), "/"))
			require.NoError(t, parseErr)
			resolution, resolveErr := resolver.ResolveCustomGroupModel(ctx, key, model)
			require.NoError(t, resolveErr)
			requestKey = resolution.APIKey
			// Mirror the custom-group middleware contract without importing routes
			// (which imports handler): it rewrites the catch-all param and context.
			c.Params = gin.Params{{Key: "modelAction", Value: "/" + resolution.SourceModel + ":" + action}}
			ctx = service.WithCustomGroupModelResolution(ctx, resolution.PublicModel, resolution.SourceModel)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(string(middleware.ContextKeyAPIKey), requestKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
		handler.GeminiV1BetaModels(c)
	})
	return &geminiNativeAliasFixture{router: router, upstream: upstream, usage: usage, wallet: wallet, key: key, account: account}
}

func (f *geminiNativeAliasFixture) post(modelPath, action string) *httptest.ResponseRecorder {
	path := "/v1beta/models/" + modelPath + ":" + action
	if action == "streamGenerateContent" {
		path += "?alt=sse"
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	return recorder
}

func TestGeminiNativeAliasRoutesAndBilling(t *testing.T) {
	for _, routeKind := range []string{"direct", "custom", "group"} {
		for _, action := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			for _, encoded := range []bool{false, true} {
				name := routeKind + "/" + action + "/literal"
				if encoded {
					name = routeKind + "/" + action + "/encoded"
				}
				t.Run(name, func(t *testing.T) {
					publicModel := geminiNativeAlias
					if routeKind == "group" {
						publicModel = "Gemini优惠"
					}
					fixture := newGeminiNativeAliasFixture(t, routeKind, publicModel, geminiNativeAlias, geminiNativeAliasTarget)
					modelPath := publicModel
					if encoded {
						modelPath = url.PathEscape(publicModel)
					}
					response := fixture.post(modelPath, action)
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					require.Len(t, fixture.upstream.requests, 1, "only the mapped local fake upstream may be called")
					upstreamRequest := fixture.upstream.requests[0]
					require.Equal(t, "/v1beta/models/"+geminiNativeAliasTarget+":"+action, upstreamRequest.URL.Path)
					require.NotContains(t, upstreamRequest.URL.EscapedPath(), "%")
					require.Equal(t, "native-alias.example.invalid", upstreamRequest.URL.Host)
					require.Equal(t, []int64{fixture.account.ID}, fixture.upstream.accounts)
					require.JSONEq(t, `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`, string(fixture.upstream.bodies[0]))
					require.Empty(t, fixture.usage.pending)
					if action == "countTokens" {
						require.JSONEq(t, `{"totalTokens":12}`, response.Body.String())
						for _, command := range fixture.wallet.commands {
							require.Zero(t, command.BalanceCost, "token counting is not paid inference")
							require.Zero(t, command.APIKeyQuotaCost)
							require.Zero(t, command.AccountQuotaCost)
						}
						for _, usage := range fixture.usage.logs {
							require.Zero(t, usage.ActualCost)
							require.Zero(t, usage.InputTokens)
							require.Zero(t, usage.OutputTokens)
						}
						return
					}
					require.Contains(t, response.Body.String(), "local test answer")
					require.Len(t, fixture.wallet.commands, 1)
					require.Len(t, fixture.usage.logs, 1)
					usage := fixture.usage.logs[0]
					require.Equal(t, publicModel, usage.Model)
					require.Equal(t, publicModel, usage.RequestedModel)
					require.NotNil(t, usage.UpstreamModel)
					require.Equal(t, geminiNativeAliasTarget, *usage.UpstreamModel)
					require.Equal(t, fixture.account.ID, usage.AccountID)
					require.Equal(t, fixture.key.UserID, usage.UserID)
					require.Equal(t, fixture.key.ID, usage.APIKeyID)
					require.NotNil(t, usage.GroupID)
					require.Equal(t, fixture.account.AccountGroups[0].GroupID, *usage.GroupID)
					require.Equal(t, fixture.key.CustomGroupID, usage.CustomGroupID)
					require.Equal(t, 12, usage.InputTokens)
					require.Equal(t, 7, usage.OutputTokens)
					command := fixture.wallet.commands[0]
					require.Equal(t, fixture.account.ID, command.AccountID)
					require.Equal(t, fixture.key.UserID, command.UserID)
					require.Equal(t, fixture.key.ID, command.APIKeyID)
					require.InDelta(t, 12*0.000001+7*0.00001, command.BalanceCost, 0.00000001)
					require.InDelta(t, command.BalanceCost, usage.ActualCost, 0.00000001)
				})
			}
		}
	}
}

func TestGeminiNativeAliasRejectsUnsafeRoutesBeforeUpstream(t *testing.T) {
	for _, model := range []string{
		"vet/../gemini-4-argon", "vet/./gemini-4-argon", "vet//gemini-4-argon",
		"vet\\gemini-4-argon", "vet/gemini-4-argon?key=other", "vet/gemini-4-argon#fragment",
		"vet/gemini-4-argon\n", "vet/%2Fgemini-4-argon", "vet/%2e%2e/gemini-4-argon",
	} {
		t.Run(model, func(t *testing.T) {
			// Even an exact account mapping must not legitimize a route containing
			// traversal or URL metacharacters. PathEscape also covers double encoding.
			fixture := newGeminiNativeAliasFixture(t, "direct", model, model, geminiNativeAliasTarget)
			response := fixture.post(url.PathEscape(model), "generateContent")
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.Empty(t, fixture.upstream.requests)
			require.Empty(t, fixture.wallet.commands)
			require.Empty(t, fixture.usage.logs)
		})
	}
	for _, target := range []string{geminiNativeAlias, "../gemini-4-argon", "gemini-4-argon%2FgenerateContent"} {
		t.Run("unsafe mapped target/"+target, func(t *testing.T) {
			fixture := newGeminiNativeAliasFixture(t, "custom", geminiNativeAlias, geminiNativeAlias, target)
			response := fixture.post(url.PathEscape(geminiNativeAlias), "generateContent")
			require.GreaterOrEqual(t, response.Code, http.StatusBadRequest, response.Body.String())
			require.Empty(t, fixture.upstream.requests)
			require.Empty(t, fixture.wallet.commands)
			require.Empty(t, fixture.usage.logs)
		})
	}
}
