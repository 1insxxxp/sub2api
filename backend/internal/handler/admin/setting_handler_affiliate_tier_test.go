//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_UpdateSettings_AffiliateTierMapsAllFields(t *testing.T) {
	repo, rec := updateAffiliateTierSettings(t, map[string]any{
		"affiliate_rebate_rate":          9,
		"affiliate_qualification_amount": 75,
		"affiliate_bronze_invitees":      5,
		"affiliate_bronze_rate":          11,
		"affiliate_silver_invitees":      15,
		"affiliate_silver_rate":          14,
		"affiliate_gold_invitees":        40,
		"affiliate_gold_rate":            18,
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "75.00000000", repo.lastUpdates[service.SettingKeyAffiliateQualificationAmount])
	require.Equal(t, "5", repo.lastUpdates[service.SettingKeyAffiliateBronzeInvitees])
	require.Equal(t, "11.00000000", repo.lastUpdates[service.SettingKeyAffiliateBronzeRate])
	require.Equal(t, "15", repo.lastUpdates[service.SettingKeyAffiliateSilverInvitees])
	require.Equal(t, "14.00000000", repo.lastUpdates[service.SettingKeyAffiliateSilverRate])
	require.Equal(t, "40", repo.lastUpdates[service.SettingKeyAffiliateGoldInvitees])
	require.Equal(t, "18.00000000", repo.lastUpdates[service.SettingKeyAffiliateGoldRate])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, 75.0, data[service.SettingKeyAffiliateQualificationAmount])
	require.Equal(t, 5.0, data[service.SettingKeyAffiliateBronzeInvitees])
	require.Equal(t, 11.0, data[service.SettingKeyAffiliateBronzeRate])
	require.Equal(t, 15.0, data[service.SettingKeyAffiliateSilverInvitees])
	require.Equal(t, 14.0, data[service.SettingKeyAffiliateSilverRate])
	require.Equal(t, 40.0, data[service.SettingKeyAffiliateGoldInvitees])
	require.Equal(t, 18.0, data[service.SettingKeyAffiliateGoldRate])
}

func TestSettingHandler_UpdateSettings_AffiliateTierReturnsStableBadRequest(t *testing.T) {
	repo, rec := updateAffiliateTierSettings(t, map[string]any{
		"affiliate_bronze_invitees": 10,
		"affiliate_silver_invitees": 10,
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.lastUpdates)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "INVALID_AFFILIATE_TIER_CONFIG", resp.Reason)
}

func TestDiffSettings_AffiliateTierIncludesAllFields(t *testing.T) {
	before := &service.SystemSettings{}
	after := &service.SystemSettings{
		AffiliateQualificationAmount: 75,
		AffiliateBronzeInvitees:      5,
		AffiliateBronzeRate:          11,
		AffiliateSilverInvitees:      15,
		AffiliateSilverRate:          14,
		AffiliateGoldInvitees:        40,
		AffiliateGoldRate:            18,
	}

	changed := diffSettings(before, after, nil, nil, UpdateSettingsRequest{})

	require.ElementsMatch(t, []string{
		service.SettingKeyAffiliateQualificationAmount,
		service.SettingKeyAffiliateBronzeInvitees,
		service.SettingKeyAffiliateBronzeRate,
		service.SettingKeyAffiliateSilverInvitees,
		service.SettingKeyAffiliateSilverRate,
		service.SettingKeyAffiliateGoldInvitees,
		service.SettingKeyAffiliateGoldRate,
	}, changed)
}

func updateAffiliateTierSettings(t *testing.T, body map[string]any) (*settingHandlerRepoStub, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(c)
	return repo, rec
}
