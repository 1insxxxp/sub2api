//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchAffiliateRepositoryStub struct {
	service.AffiliateRepository
	items  []service.AffiliateInviterSummary
	total  int64
	filter service.AffiliateRecordFilter
}

func (s *workbenchAffiliateRepositoryStub) ListAffiliateInviterSummaries(
	_ context.Context,
	filter service.AffiliateRecordFilter,
	_ float64,
) ([]service.AffiliateInviterSummary, int64, error) {
	s.filter = filter
	return s.items, s.total, nil
}

func TestAffiliateHandlerListWorkbenchLeaderboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lastInvitedAt := time.Date(2026, time.August, 29, 9, 30, 0, 0, time.UTC)
	leader := service.AffiliateInviterSummary{
		InviterID:             42,
		InviterEmail:          "leader@example.com",
		InviterUsername:       "leader",
		InviterAvatarURL:      "https://cdn.example.com/leader.png",
		AffCode:               "MUST_NOT_LEAK",
		InvitedCount:          31,
		QualifiedInviteeCount: 18,
		TotalRebate:           12.34,
		AvailableQuota:        99.99,
		TransferredAmount:     20,
		RebateRecordCount:     7,
		LastInvitedAt:         &lastInvitedAt,
	}
	repo := &workbenchAffiliateRepositoryStub{
		items: []service.AffiliateInviterSummary{leader},
		total: 1,
	}
	handler := NewAffiliateHandler(service.NewAffiliateService(repo, nil, nil, nil), nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/workbench/affiliates/leaderboard", nil)

	handler.ListWorkbenchLeaderboard(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, repo.filter.Page)
	require.Equal(t, 20, repo.filter.PageSize)
	require.Equal(t, "invited_count", repo.filter.SortBy)
	require.True(t, repo.filter.SortDesc)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Zero(t, payload.Code)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, map[string]any{
		"inviter_id":              float64(42),
		"inviter_email":           "leader@example.com",
		"inviter_username":        "leader",
		"inviter_avatar_url":      "https://cdn.example.com/leader.png",
		"invited_count":           float64(31),
		"qualified_invitee_count": float64(18),
		"total_rebate":            12.34,
		"last_invited_at":         lastInvitedAt.Format(time.RFC3339),
	}, payload.Data.Items[0])
}
