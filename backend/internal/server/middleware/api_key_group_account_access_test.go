//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthBindsGroupAccountPolicyBeforeSimpleMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		for _, runMode := range []string{config.RunModeSimple, config.RunModeStandard} {
			name := runMode
			if google {
				name += "/google"
			}
			t.Run(name, func(t *testing.T) {
				groupID := int64(42)
				key := &service.APIKey{
					ID: 100, Key: "test-policy-key", UserID: 7, Status: service.StatusActive, GroupID: &groupID,
					User:  &service.User{ID: 7, Status: service.StatusActive, Balance: 100, GroupAccountExclusions: service.GroupAccountExclusions{groupID: {101}}},
					Group: &service.Group{ID: groupID, Status: service.StatusActive, Platform: service.PlatformGemini, SubscriptionType: service.SubscriptionTypeStandard, Hydrated: true},
				}
				repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
				cfg := &config.Config{RunMode: runMode}
				svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
				router := gin.New()
				if google {
					router.Use(APIKeyAuthGoogle(svc, cfg))
				} else {
					router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
				}
				router.GET("/t", func(c *gin.Context) {
					require.False(t, service.IsGroupAccountAllowedForUser(c.Request.Context(), &groupID, 101))
					require.True(t, service.IsGroupAccountAllowedForUser(c.Request.Context(), &groupID, 102))
					c.Status(http.StatusOK)
				})
				request := httptest.NewRequest(http.MethodGet, "/t", nil)
				request.Header.Set("Authorization", "Bearer "+key.Key)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			})
		}
	}
}
