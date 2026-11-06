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
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type claude55CountPolicyUpstream struct{ body []byte }

func (u *claude55CountPolicyUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.body, _ = io.ReadAll(req.Body)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"input_tokens":18}`))}, nil
}

func (u *claude55CountPolicyUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func TestClaude55CountTokensHandlerEnforcesDerivedEffortPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, extra, ceiling, want string
		deny                       bool
		mappings                   []service.ReasoningEffortMapping
	}{
		{name: "derived downgrade", ceiling: "medium", want: "medium"},
		{name: "derived deny", ceiling: "medium", deny: true},
		{name: "explicit downgrade", extra: `,"output_config":{"effort":"max"}`, ceiling: "medium", want: "medium"},
		{name: "explicit deny", extra: `,"output_config":{"effort":"max"}`, ceiling: "medium", deny: true},
		{name: "explicit mapping once", extra: `,"output_config":{"effort":"max"}`, want: "high", mappings: []service.ReasoningEffortMapping{{From: "max", To: "high"}, {From: "high", To: "low"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(9800)
			group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, MaxReasoningEffort: tc.ceiling, ReasoningEffortMappings: tc.mappings}
			if tc.deny {
				group.MaxReasoningEffortOverLimit = service.ReasoningEffortOverLimitDeny
			}
			account := &service.Account{ID: 9801, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "fixture", "base_url": "https://api.anthropic.com", "model_mapping": map[string]any{"public-claude": "claude-opus-5-5"}},
				Extra:       map[string]any{"anthropic_passthrough": true}, AccountGroups: []service.AccountGroup{{AccountID: 9801, GroupID: groupID}}}
			upstream := &claude55CountPolicyUpstream{}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
			gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil, cfg,
				snapshot, nil, nil, &service.RateLimitService{}, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil,
				&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil)
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			h := &GatewayHandler{gatewayService: gateway, billingCacheService: billing, cfg: cfg}
			apiKey := &service.APIKey{ID: 9850, UserID: 9851, GroupID: &groupID, Group: group, Status: service.StatusActive, User: &service.User{ID: 9851, Balance: 100}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := fmt.Sprintf(`{"model":"public-claude","messages":[{"role":"user","content":"hello"}],"thinking":{"type":"enabled","budget_tokens":32768}%s}`, tc.extra)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(body)).WithContext(context.WithValue(context.Background(), ctxkey.Group, group))
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
			h.CountTokens(c)
			if tc.deny {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Empty(t, upstream.body)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tc.want, gjson.GetBytes(upstream.body, "output_config.effort").String())
		})
	}
}
