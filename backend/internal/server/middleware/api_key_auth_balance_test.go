package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authBalanceUserRepo struct {
	service.UserRepository
	getByID func(context.Context, int64) (*service.User, error)
}

func (r authBalanceUserRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	return r.getByID(ctx, id)
}

func authZeroBalanceUserRepo() service.UserRepository {
	return authBalanceUserRepo{getByID: func(_ context.Context, id int64) (*service.User, error) {
		return &service.User{ID: id, Status: service.StatusActive}, nil
	}}
}

func TestAPIKeyAuthBalanceRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		format := "standard"
		if google {
			format = "google"
		}
		t.Run(format, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				cached      float64
				fresh       float64
				freshStatus string
				err         error
				status      int
				message     string
				wantDBReads int
				keyStatus   string
				quota       float64
				quotaUsed   float64
				expired     bool
				revokeGroup bool
				excludedID  int64
			}{
				{name: "recharged_after_cached_zero", fresh: 0.005, status: http.StatusOK, wantDBReads: 1},
				{name: "recharged_after_cached_debt", cached: -1, fresh: 0.005, status: http.StatusOK, wantDBReads: 1},
				{name: "recharged_preserves_account_exclusions", fresh: 0.005, status: http.StatusOK, wantDBReads: 1, excludedID: 9001},
				{name: "zero_confirmed", status: http.StatusForbidden, message: "Insufficient account balance", wantDBReads: 1},
				{name: "debt_confirmed", fresh: -1, status: http.StatusForbidden, message: "Insufficient account balance", wantDBReads: 1},
				{name: "database_unavailable", err: errors.New("database down"), status: http.StatusServiceUnavailable, message: "Billing service temporarily unavailable", wantDBReads: 1},
				{name: "fresh_user_disabled", fresh: 1, freshStatus: service.StatusDisabled, status: http.StatusUnauthorized, message: "User account is not active", wantDBReads: 1},
				{name: "key_disabled", fresh: 1, keyStatus: service.StatusDisabled, status: http.StatusUnauthorized, message: "API key is disabled"},
				{name: "key_expired", fresh: 1, expired: true, status: http.StatusForbidden, message: "API key 已过期"},
				{name: "key_quota_exhausted", fresh: 1, quota: 1, quotaUsed: 1, status: http.StatusTooManyRequests, message: "额度已用完"},
				{name: "fresh_group_revocation", fresh: 1, revokeGroup: true, status: http.StatusForbidden, message: "不再允许当前用户使用", wantDBReads: 1},
				{name: "healthy_cache_avoids_database", cached: 1, status: http.StatusOK},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cachedUser := &service.User{ID: 7, Status: service.StatusActive, Balance: tc.cached, Role: service.RoleUser, Concurrency: 3}
					group := &service.Group{ID: 42, Status: service.StatusActive, IsExclusive: tc.revokeGroup}
					cachedUser.AllowedGroups = []int64{group.ID}
					if tc.excludedID > 0 {
						cachedUser.GroupAccountExclusions = service.GroupAccountExclusions{group.ID: {tc.excludedID}}
					}
					key := &service.APIKey{ID: 101, UserID: cachedUser.ID, Key: "balance-recovery-test", Status: service.StatusActive, User: cachedUser, GroupID: &group.ID, Group: group, Quota: tc.quota, QuotaUsed: tc.quotaUsed}
					if tc.keyStatus != "" {
						key.Status = tc.keyStatus
					}
					if tc.expired {
						past := time.Now().Add(-time.Minute)
						key.ExpiresAt = &past
					}
					reads := 0
					freshStatus := service.StatusActive
					if tc.freshStatus != "" {
						freshStatus = tc.freshStatus
					}
					freshUser := &service.User{ID: cachedUser.ID, Status: freshStatus, Balance: tc.fresh, Role: service.RoleUser, Concurrency: 3}
					userRepo := authBalanceUserRepo{getByID: func(_ context.Context, id int64) (*service.User, error) {
						reads++
						require.Equal(t, cachedUser.ID, id)
						return freshUser, tc.err
					}}
					cfg := &config.Config{RunMode: config.RunModeStandard}
					repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
					svc := service.NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, cfg)
					r := gin.New()
					if google {
						r.Use(APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg))
					} else {
						r.Use(apiKeyAuthWithSubscription(svc, nil, cfg))
					}
					var resultKey *service.APIKey
					var excludedAccountAllowed bool
					r.POST("/test", func(c *gin.Context) {
						var ok bool
						resultKey, ok = c.MustGet(string(ContextKeyAPIKey)).(*service.APIKey)
						require.True(t, ok)
						excludedAccountAllowed = service.IsGroupAccountAllowedForUser(c.Request.Context(), &group.ID, tc.excludedID)
						c.Status(http.StatusOK)
					})
					req := httptest.NewRequest(http.MethodPost, "/test", nil)
					req.Header.Set("x-api-key", key.Key)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					require.Equal(t, tc.status, w.Code, w.Body.String())
					require.Equal(t, tc.wantDBReads, reads)
					if tc.message != "" {
						require.Contains(t, w.Body.String(), tc.message)
					}
					if tc.status == http.StatusOK && tc.wantDBReads > 0 {
						require.Equal(t, tc.fresh, resultKey.User.Balance)
						require.NotSame(t, key, resultKey)
						require.NotSame(t, freshUser, resultKey.User)
					}
					if tc.excludedID > 0 {
						require.Nil(t, freshUser.GroupAccountExclusions, "GetByID does not hydrate account exclusions")
						require.Equal(t, cachedUser.GroupAccountExclusions, resultKey.User.GroupAccountExclusions)
						require.False(t, excludedAccountAllowed, "balance recovery must preserve routing restrictions in context")
					}
					require.Equal(t, tc.cached, cachedUser.Balance, "recovery must not mutate a shared cached user")
					require.Same(t, cachedUser, key.User)
				})
			}
		})
	}
}
