//go:build unit

package handler

import (
	"bytes"
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

type missingModelUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	models     []string
}

func (u *missingModelUpstream) DoWithTLS(req *http.Request, _ string, accountID int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.accountIDs = append(u.accountIDs, accountID)
	u.models = append(u.models, gjson.GetBytes(body, "model").String())
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"X-Request-Id": {fmt.Sprintf("attempt-%d", accountID)}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"Model \"claude-opus-4-6\" is not supported by any configured account in this group"}}`,
		)),
	}, nil
}

func TestGatewayModelNotFoundFailoverSelectsNextAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []struct {
		path string
		body string
		call func(*GatewayHandler, *gin.Context)
	}{
		{"/v1/chat/completions", `{"model":"public-opus","messages":[{"role":"user","content":"hello"}],"stream":true}`, (*GatewayHandler).ChatCompletions},
		{"/v1/responses", `{"model":"public-opus","input":"hello","stream":true}`, (*GatewayHandler).Responses},
	} {
		for _, switches := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/switches=%d", endpoint.path, switches), func(t *testing.T) {
				groupID := int64(9400)
				group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive}
				accounts := []*service.Account{}
				for i := 1; i <= 3; i++ {
					accountID := int64(9400 + i)
					accounts = append(accounts, &service.Account{
						ID: accountID, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
						Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: i,
						Credentials: map[string]any{
							"api_key": "test-key", "base_url": "https://api.anthropic.com",
							"model_mapping": map[string]any{"public-opus": "claude-opus-4-6"},
							"pool_mode":     true, "pool_mode_retry_status_codes": []any{float64(http.StatusNotFound)},
						},
						AccountGroups: []service.AccountGroup{{AccountID: accountID, GroupID: groupID}},
					})
				}
				upstream := &missingModelUpstream{}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
				gateway := service.NewGatewayService(
					nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil, cfg,
					snapshot, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil,
					&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil,
				)
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				h := &GatewayHandler{
					gatewayService: gateway, billingCacheService: billing, cfg: cfg,
					concurrencyHelper:  NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
					maxAccountSwitches: switches,
				}
				apiKey := &service.APIKey{
					ID: 9500, UserID: 9501, GroupID: &groupID, Group: group, Status: service.StatusActive,
					User: &service.User{ID: 9501, Concurrency: 10, Balance: 100},
				}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				ctx := context.WithValue(context.Background(), ctxkey.Group, group)
				c.Request = httptest.NewRequest(http.MethodPost, endpoint.path, bytes.NewBufferString(endpoint.body)).WithContext(ctx)
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

				endpoint.call(h, c)

				require.Equal(t, []int64{9401, 9402}[:switches+1], upstream.accountIDs, "each failed account must be excluded, without bypassing the switch limit")
				for _, model := range upstream.models {
					require.Equal(t, "claude-opus-4-6", model)
				}
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Contains(t, recorder.Body.String(), "All available accounts exhausted")
				events := c.MustGet(service.OpsUpstreamErrorsKey).([]*service.OpsUpstreamErrorEvent)
				require.Len(t, events, switches+1)
				for i, event := range events {
					require.Equal(t, upstream.accountIDs[i], event.AccountID)
					require.Equal(t, http.StatusNotFound, event.UpstreamStatusCode)
				}
				phase, limited, owner, source := classifyOpsErrorLog(c, "server_error", "All available accounts exhausted", "", recorder.Code)
				require.Equal(t, "upstream", phase)
				require.False(t, limited)
				require.Equal(t, "provider", owner)
				require.Equal(t, "upstream_http", source)
			})
		}
	}
}
