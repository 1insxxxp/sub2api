//go:build unit

package routes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const publicGroupSyncRouteSecret = "public-group-sync-route-secret"

type publicGroupSyncRouteGroupRepo struct{}

func (publicGroupSyncRouteGroupRepo) Create(context.Context, *service.Group) error { return nil }
func (publicGroupSyncRouteGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return nil, nil
}
func (publicGroupSyncRouteGroupRepo) GetByIDLite(context.Context, int64) (*service.Group, error) {
	return nil, nil
}
func (publicGroupSyncRouteGroupRepo) Update(context.Context, *service.Group) error { return nil }
func (publicGroupSyncRouteGroupRepo) Delete(context.Context, int64) error          { return nil }
func (publicGroupSyncRouteGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (publicGroupSyncRouteGroupRepo) List(context.Context, pagination.PaginationParams) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (publicGroupSyncRouteGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (publicGroupSyncRouteGroupRepo) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{{ID: 1, Name: "public", Status: service.StatusActive}}, nil
}
func (publicGroupSyncRouteGroupRepo) ListActiveByPlatform(context.Context, string) ([]service.Group, error) {
	return nil, nil
}
func (publicGroupSyncRouteGroupRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
func (publicGroupSyncRouteGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (publicGroupSyncRouteGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (publicGroupSyncRouteGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (publicGroupSyncRouteGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error {
	return nil
}
func (publicGroupSyncRouteGroupRepo) UpdateSortOrders(context.Context, []service.GroupSortOrderUpdate) error {
	return nil
}

type publicGroupSyncRouteChannelRepo struct{}

func (publicGroupSyncRouteChannelRepo) Create(context.Context, *service.Channel) error { return nil }
func (publicGroupSyncRouteChannelRepo) GetByID(context.Context, int64) (*service.Channel, error) {
	return nil, nil
}
func (publicGroupSyncRouteChannelRepo) Update(context.Context, *service.Channel) error { return nil }
func (publicGroupSyncRouteChannelRepo) Delete(context.Context, int64) error            { return nil }
func (publicGroupSyncRouteChannelRepo) List(context.Context, pagination.PaginationParams, string, string) ([]service.Channel, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (publicGroupSyncRouteChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	return []service.Channel{{ID: 1, Status: service.StatusActive, GroupIDs: []int64{1}}}, nil
}
func (publicGroupSyncRouteChannelRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
func (publicGroupSyncRouteChannelRepo) ExistsByNameExcluding(context.Context, string, int64) (bool, error) {
	return false, nil
}
func (publicGroupSyncRouteChannelRepo) GetGroupIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (publicGroupSyncRouteChannelRepo) SetGroupIDs(context.Context, int64, []int64) error { return nil }
func (publicGroupSyncRouteChannelRepo) GetChannelIDByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (publicGroupSyncRouteChannelRepo) GetGroupsInOtherChannels(context.Context, int64, []int64) ([]int64, error) {
	return nil, nil
}
func (publicGroupSyncRouteChannelRepo) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (publicGroupSyncRouteChannelRepo) ListModelPricing(context.Context, int64) ([]service.ChannelModelPricing, error) {
	return nil, nil
}
func (publicGroupSyncRouteChannelRepo) CreateModelPricing(context.Context, *service.ChannelModelPricing) error {
	return nil
}
func (publicGroupSyncRouteChannelRepo) UpdateModelPricing(context.Context, *service.ChannelModelPricing) error {
	return nil
}
func (publicGroupSyncRouteChannelRepo) DeleteModelPricing(context.Context, int64) error { return nil }
func (publicGroupSyncRouteChannelRepo) ReplaceModelPricing(context.Context, int64, []service.ChannelModelPricing) error {
	return nil
}

func newPublicGroupSyncRouteRouter(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{PublicGroupSync: config.PublicGroupSyncConfig{Secret: secret}}
	svc := service.NewPublicGroupSyncService(publicGroupSyncRouteGroupRepo{}, publicGroupSyncRouteChannelRepo{})
	h := handler.NewPublicGroupSyncHandler(svc, cfg)
	router := gin.New()
	RegisterPublicGroupSyncRoute(router.Group("/api"), &handler.Handlers{PublicGroupSync: h})
	return router
}

func publicGroupSyncSignature(secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "\n"))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestPublicGroupSyncRouteServesSignedSnapshotAndRejectsInvalidSignature(t *testing.T) {
	router := newPublicGroupSyncRouteRouter(publicGroupSyncRouteSecret)
	timestamp := time.Now().Unix()
	path := "/api/internal/public-group-sync/snapshot"

	valid := httptest.NewRequest(http.MethodGet, path, nil)
	valid.Header.Set("X-Public-Group-Sync-Timestamp", strconv.FormatInt(timestamp, 10))
	valid.Header.Set("X-Public-Group-Sync-Signature", publicGroupSyncSignature(publicGroupSyncRouteSecret, timestamp))
	validRecorder := httptest.NewRecorder()
	router.ServeHTTP(validRecorder, valid)
	require.Equal(t, http.StatusOK, validRecorder.Code, validRecorder.Body.String())
	require.Contains(t, validRecorder.Body.String(), `"version":1`)
	require.Contains(t, validRecorder.Body.String(), `"group_id":1`)

	invalid := httptest.NewRequest(http.MethodGet, path, nil)
	invalid.Header.Set("X-Public-Group-Sync-Timestamp", strconv.FormatInt(timestamp, 10))
	invalid.Header.Set("X-Public-Group-Sync-Signature", publicGroupSyncSignature("wrong-secret", timestamp))
	invalidRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidRecorder, invalid)
	require.Equal(t, http.StatusUnauthorized, invalidRecorder.Code)
}

func TestPublicGroupSyncRouteTrimsConfiguredSecret(t *testing.T) {
	router := newPublicGroupSyncRouteRouter("  " + publicGroupSyncRouteSecret + "  ")
	timestamp := time.Now().Unix()
	req := httptest.NewRequest(http.MethodGet, "/api/internal/public-group-sync/snapshot", nil)
	req.Header.Set("X-Public-Group-Sync-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Public-Group-Sync-Signature", publicGroupSyncSignature(publicGroupSyncRouteSecret, timestamp))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}
