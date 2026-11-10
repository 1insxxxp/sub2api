//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type chatReasoningPolicyUpstream struct {
	service.HTTPUpstream
	body  []byte
	calls int
}

func (u *chatReasoningPolicyUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	u.body, _ = io.ReadAll(req.Body)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(compatStreamUsagePartialStream + compatStreamUsageCompletion))}, nil
}

func TestGatewayChatCompletionsAnthropicReasoningPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, controls, ceiling, want string
		deny, composite               bool
		mappings                      []service.ReasoningEffortMapping
	}{
		{name: "native ceiling", controls: `"output_config":{"effort":"max"},`, ceiling: "low", want: "low"},
		{name: "legacy ceiling", controls: `"reasoning_effort":"max",`, ceiling: "low", want: "low"},
		{name: "native over limit denied", controls: `"output_config":{"effort":"max"},`, ceiling: "low", deny: true},
		{name: "legacy over limit denied", controls: `"reasoning_effort":"max",`, ceiling: "low", deny: true},
		{name: "native scoped deny", controls: `"output_config":{"effort":"max"},`, deny: true, mappings: []service.ReasoningEffortMapping{{From: "max", To: service.ReasoningEffortMappingDeny, Model: "story-alias", MatchType: domain.ReasoningEffortMatchExact}}},
		{name: "mapping applied once", controls: `"output_config":{"effort":"max"},`, want: "high", mappings: []service.ReasoningEffortMapping{{From: "max", To: "high"}, {From: "high", To: "low"}}},
		{name: "scope uses public model", controls: `"output_config":{"effort":"max"},`, want: "max", mappings: []service.ReasoningEffortMapping{{From: "max", To: "low", Model: "claude-opus-4-6", MatchType: domain.ReasoningEffortMatchExact}}},
		{name: "unrestricted native", controls: `"output_config":{"effort":"medium"},`, want: "medium"},
		{name: "unrestricted legacy", controls: `"reasoning_effort":"high",`, want: "high"},
		{name: "composite anthropic ceiling", controls: `"output_config":{"effort":"max"},`, ceiling: "low", want: "low", composite: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: 9930, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1, MaxReasoningEffort: tc.ceiling, ReasoningEffortMappings: tc.mappings,
				ModelAliases: []domain.ModelAlias{{Alias: "story-alias", UpstreamModel: "claude-opus-4-6"}}}
			if tc.deny {
				group.MaxReasoningEffortOverLimit = service.ReasoningEffortOverLimitDeny
			}
			if tc.composite {
				group.Platform = service.PlatformComposite
			}
			body := fmt.Sprintf(`{"model":"story-alias","max_tokens":8192,%s"messages":[{"role":"user","content":"Continue."}]}`, tc.controls)
			rec, c, upstream := runChatReasoningPolicyHandler(t, group, body)
			if tc.deny {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Zero(t, upstream.calls)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, 1, upstream.calls)
			require.Equal(t, tc.want, gjson.GetBytes(upstream.body, "output_config.effort").String())
			requested := service.RequestedReasoningEffortFromContext(c.Request.Context())
			require.NotNil(t, requested)
			if strings.Contains(tc.controls, "max") {
				require.Equal(t, "max", *requested)
			}
		})
	}
}

func runChatReasoningPolicyHandler(t *testing.T, group *service.Group, body string) (*httptest.ResponseRecorder, *gin.Context, *chatReasoningPolicyUpstream) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	account := &service.Account{ID: 9931, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "fixture", "model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"}}, AccountGroups: []service.AccountGroup{{AccountID: 9931, GroupID: group.ID}}}
	upstream := &chatReasoningPolicyUpstream{}
	usage := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
	billingRepo := &compatStreamUsageBillingRepo{}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usage, billingRepo, nil, nil, nil, nil, cfg,
		snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil, &service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil)
	billing := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := &GatewayHandler{gatewayService: gateway, billingCacheService: billing, cfg: cfg, maxAccountSwitches: 1,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0)}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)).WithContext(context.WithValue(context.Background(), ctxkey.Group, group))
	key := &service.APIKey{ID: 9932, UserID: 9933, GroupID: &group.ID, Group: group, User: &service.User{ID: 9933, Balance: 100}}
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
	h.ChatCompletions(c)
	return rec, c, upstream
}
