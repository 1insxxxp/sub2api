//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicForwardEmbeddedErrorPreservesModelScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, account, c, _, parsed := newEmbeddedErrorForwardFixture(t, true, `{"type":"error","error":{"type":"api_error","status_code":502,"message":"temporary provider failure"}}`)
	repo := &nonJSONTempUnschedAccountRepo{}
	svc.rateLimitService = NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account.Credentials["temp_unschedulable_enabled"] = true
	account.Credentials["temp_unschedulable_rules"] = []any{map[string]any{
		"error_code": float64(http.StatusBadGateway), "keywords": []any{"temporary provider failure"}, "duration_minutes": float64(10),
	}}

	result, err := svc.Forward(c.Request.Context(), c, account, parsed)

	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Zero(t, repo.tempUnschedCalls, "a model error must not cool down the whole account")
	require.Equal(t, 1, repo.modelRateLimitCalls)
	require.Equal(t, parsed.Model, repo.modelScope)
}
