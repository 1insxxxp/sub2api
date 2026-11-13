//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayPrecharge_RejectsInsufficientFundsBeforeUpstream(t *testing.T) {
	for _, protocol := range []string{"chat", "messages", "responses", "gemini"} {
		for _, mode := range []service.BillingMode{service.BillingModePerRequest, service.BillingModeToken} {
			t.Run(protocol+"/"+string(mode), func(t *testing.T) {
				recorder, upstream, cache := runGatewayPrecharge(t, protocol, mode, 0.01)
				require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), "余额不足")
				require.Contains(t, recorder.Body.String(), "当前余额 US$0.01")
				require.Contains(t, recorder.Body.String(), "本次预扣额度")
				require.Zero(t, upstream.calls)
				require.Zero(t, cache.count())
				balance, err := cache.GetUserBalance(context.Background(), 9903)
				require.NoError(t, err)
				require.Equal(t, 0.01, balance)
				if mode == service.BillingModePerRequest {
					require.Contains(t, recorder.Body.String(), "本次预扣额度 US$0.50")
				}
			})
		}
	}
}

func TestGatewayPrecharge_ExactFixedPriceCanCallUpstream(t *testing.T) {
	for _, protocol := range []string{"chat", "messages", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			recorder, upstream, cache := runGatewayPrecharge(t, protocol, service.BillingModePerRequest, 0.5)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, 1, upstream.calls)
			require.Zero(t, cache.count(), "settled request releases its precharge")
			balance, err := cache.GetUserBalance(context.Background(), 9903)
			require.NoError(t, err)
			require.Zero(t, balance, "the fixed fee is settled exactly once")
		})
	}
}

func runGatewayPrecharge(t *testing.T, protocol string, mode service.BillingMode, balance float64, metadata ...string) (*httptest.ResponseRecorder, *compatStreamUsageUpstream, *handlerInflightCache) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, DefaultMaxTokens: 1000, FailClosedOnUnpriced: true}
	groupID := int64(9900)
	price, inputPrice, outputPrice := 0.5, 0.00001, 0.00005
	model := "claude-sonnet-4-5"
	platform := service.PlatformAnthropic
	if protocol == "gemini" {
		model = "gemini-2.5-flash"
		platform = service.PlatformGemini
	}
	group := &service.Group{ID: groupID, Hydrated: true, Platform: platform, Status: service.StatusActive,
		RateMultiplier: 1, ModelPricing: []service.ChannelModelPricing{{Models: []string{model}, BillingMode: mode,
			PerRequestPrice: &price, InputPrice: &inputPrice, OutputPrice: &outputPrice}},
	}
	account := &service.Account{ID: 9901, Platform: platform, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials:   map[string]any{"api_key": "local-precharge-test"},
		AccountGroups: []service.AccountGroup{{AccountID: 9901, GroupID: groupID}},
	}
	cache := newHandlerInflightCache(balance)
	users := &cachedBillingUserRepo{balance: balance}
	billing := service.NewBillingCacheService(cache, users, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	upstream := &compatStreamUsageUpstream{response: &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body:   io.NopCloser(strings.NewReader(compatStreamUsagePartialStream + compatStreamUsageCompletion)),
	}}
	usage := &partialChatUsageRepo{}
	wallet := &compatStreamUsageBillingRepo{}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	tokenBilling := service.NewBillingService(cfg, nil)
	pricing := service.NewModelPricingResolver(nil, tokenBilling)
	gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usage, wallet, users, nil, nil, nil, cfg,
		snapshot, nil, tokenBilling, nil, billing, nil, upstream, &service.DeferredService{},
		nil, nil, nil, nil, nil, nil, nil, pricing, nil, nil, nil)
	h := &GatewayHandler{gatewayService: gateway, billingCacheService: billing, cfg: cfg,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0)}
	apiKey := &service.APIKey{ID: 9902, UserID: 9903, GroupID: &groupID, Group: group, Status: service.StatusActive,
		User: &service.User{ID: 9903, Status: service.StatusActive, Concurrency: 1, Balance: balance}}
	body := `{"model":"` + model + `","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":1000}`
	path := "/v1/chat/completions"
	if protocol == "messages" {
		path = "/v1/messages"
	} else if protocol == "responses" {
		path = "/v1/responses"
		body = `{"model":"` + model + `","input":"hello","stream":true,"max_output_tokens":1000}`
	} else if protocol == "gemini" {
		path = "/v1beta/models/" + model + ":streamGenerateContent?alt=sse"
		body = `{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"generationConfig":{"maxOutputTokens":1000}}`
	}
	if len(metadata) > 0 {
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &payload))
		payload["metadata"] = metadata[0]
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		body = string(encoded)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 1})
	switch protocol {
	case "messages":
		h.Messages(c)
	case "responses":
		h.Responses(c)
	case "gemini":
		c.Params = gin.Params{{Key: "modelAction", Value: "/" + model + ":streamGenerateContent"}}
		h.GeminiV1BetaModels(c)
	default:
		h.ChatCompletions(c)
	}
	return recorder, upstream, cache
}

func TestGatewayPrecharge_MetadataDoesNotBlockTokenRequests(t *testing.T) {
	for _, protocol := range []string{"chat", "messages", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			response, upstream, _ := runGatewayPrecharge(t, protocol, service.BillingModeToken, 0.1, strings.Repeat("metadata", 300000))
			require.Equal(t, 1, upstream.calls, response.Body.String())
			require.NotContains(t, response.Body.String(), "余额不足")
		})
	}
}
