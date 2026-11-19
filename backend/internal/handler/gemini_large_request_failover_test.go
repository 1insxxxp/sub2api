//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type largeGeminiFailoverUpstream struct {
	service.HTTPUpstream
	accounts []int64
	statuses []int
}

func (u *largeGeminiFailoverUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	status := u.statuses[min(len(u.accounts), len(u.statuses)-1)]
	u.accounts = append(u.accounts, accountID)
	body := `{"error":{"message":"unavailable","status":"UNAVAILABLE"}}`
	if status == http.StatusNotFound {
		body = `{"error":{"message":"endpoint not found","status":"NOT_FOUND"}}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestGeminiLargeRequestHandlerTraversesAccounts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []int
	}{
		{"overload", []int{503}},
		{"missing then overload", []int{404, 503}},
		{"missing", []int{404}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: 9800, Hydrated: true, Platform: service.PlatformGemini, Status: service.StatusActive}
			var accounts []*service.Account
			for i := 1; i <= 6; i++ {
				id := int64(i)
				accounts = append(accounts, &service.Account{ID: id, Platform: group.Platform, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Priority: i, Concurrency: 1,
					Credentials:   map[string]any{"api_key": "test", "pool_mode": true, "pool_mode_retry_count": 8},
					AccountGroups: []service.AccountGroup{{GroupID: group.ID, AccountID: id}},
				})
			}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Gateway.MaxSameAccountRetriesGemini = 8
			snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
			upstream := &largeGeminiFailoverUpstream{statuses: tc.statuses}
			gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil, cfg,
				snapshot, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, &service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil)
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			h := &GatewayHandler{gatewayService: gateway, cfg: cfg, billingCacheService: billing, maxAccountSwitchesGemini: 8,
				geminiCompatService: service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, upstream, nil, cfg, nil),
				concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0)}
			key := &service.APIKey{ID: 9801, UserID: 9802, GroupID: &group.ID, Group: group, Status: service.StatusActive, User: &service.User{ID: 9802, Concurrency: 10, Balance: 100}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			action := "/gemini-2.5-flash:generateContent"
			c.Params = gin.Params{{Key: "modelAction", Value: action}}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models"+action, strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)).WithContext(ctx)
			c.Set(string(middleware.ContextKeyAPIKey), key)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
			// Exercise the post-admission path directly, independently of ingress size accounting.
			service.BeginGeminiLargeRequestBudget(c)
			h.GeminiV1BetaModels(c)
			require.Equal(t, []int64{1, 2, 3, 4}, upstream.accounts, rec.Body.String())
			require.GreaterOrEqual(t, rec.Code, 400)
		})
	}
}
