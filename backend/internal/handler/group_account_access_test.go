//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGeminiCachedContentRejectsExcludedOwnerAccount(t *testing.T) {
	repo := &geminiCachedContentHandlerRepoStub{}
	record := seedGeminiCachedContent(repo, "privatecache", 7, time.Now().Add(time.Hour))
	h := newGeminiCachedContentHandlerForTest(repo)
	key := geminiCachedContentTestAPIKey(service.PlatformGemini)
	c, response := newGeminiCachedContentTestContext(http.MethodPost, "/", key)
	c.Request = c.Request.WithContext(service.WithGroupAccountExclusions(context.Background(), key.UserID, service.GroupAccountExclusions{record.GroupID: {record.AccountID}}))
	_, _, ok := h.resolveGeminiCachedContentBinding(c, zap.NewNop(), key, record.Model, []byte(`{"cachedContent":"`+record.PublicName()+`"}`))
	require.False(t, ok)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.NotContains(t, response.Body.String(), "900")
	require.NotContains(t, response.Body.String(), "excluded")
	// The exclusion prevents new upstream use while owners retain access to
	// existing cache metadata and cleanup, whose ownership checks are unchanged.
	_, ok = h.resolveGeminiCachedContentOrWrite(c, zap.NewNop(), key, record.PublicName())
	require.True(t, ok)
}

func TestGeminiCachedContentPatchRejectsExcludedOwnerAccount(t *testing.T) {
	repo := &geminiCachedContentHandlerRepoStub{}
	record := seedGeminiCachedContent(repo, "privatecache", 7, time.Now().Add(time.Hour))
	h := newGeminiCachedContentHandlerForTest(repo)
	key := geminiCachedContentTestAPIKey(service.PlatformGemini)
	c, response := newGeminiCachedContentTestContext(http.MethodPatch, "/v1beta/"+record.PublicName(), key)
	c.AddParam("cacheID", record.PublicName())
	c.Request.Body = io.NopCloser(strings.NewReader(`{"ttl":"7200s"}`))
	c.Request = c.Request.WithContext(service.WithGroupAccountExclusions(c.Request.Context(), key.UserID, service.GroupAccountExclusions{record.GroupID: {record.AccountID}}))
	h.GeminiCachedContentsPatch(c)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), geminiCachedContentUnavailableMessage)
	require.NotContains(t, response.Body.String(), "900")
	require.NotContains(t, response.Body.String(), "excluded")
}

func TestOpenAIResponsesWebSocket_GroupAccountExclusionAppliesToNextTurn(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			repo := newWSGroupPricingAPIKeyRepoStub(3)
			keyService := newWSGroupPricingAPIKeyService(repo)
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload: wsGroupPricingFirst, secondPayload: wsGroupPricingSecond,
				ingressMode: mode, apiKeyService: keyService, apiKeyCredential: wsGroupPricingKey,
				afterFirstUpstreamRequest: func(*service.ChannelService) error {
					return repo.adminUpdate(keyService, func(key *service.APIKey, _ *service.Group) {
						key.User.GroupAccountExclusions = service.GroupAccountExclusions{wsGroupPricingGroupID: {9901}}
					})
				},
				secondTurnCloseExpected: true, closeStatus: coderws.StatusTryAgainLater,
				closeReason: "no longer eligible for this connection",
			})
		})
	}
}

func TestOpenAIWSTurnAccessRefreshAllowsUngroupedSimpleKey(t *testing.T) {
	conn := &service.APIKey{ID: 5, Key: "sk-ungrouped", User: &service.User{ID: 9}}
	latest := *conn
	latest.User = &service.User{ID: 9, GroupAccountExclusions: service.GroupAccountExclusions{10: {101}}}
	got, err := refreshOpenAIWSTurnAPIKey(context.Background(), wsTurnAPIKeyLookupFunc(func(context.Context, string) (*service.APIKey, error) {
		return &latest, nil
	}), conn)
	require.NoError(t, err)
	require.Nil(t, got.GroupID)
	require.Nil(t, got.Group)
	require.Equal(t, latest.User.GroupAccountExclusions, got.User.GroupAccountExclusions)
}

func TestOpenAIWSTurnAccessRefreshKeepsResolvedSystemCustomSource(t *testing.T) {
	billingID, sourceID := int64(99), int64(10)
	conn := &service.APIKey{ID: 5, Key: "sk-custom", GroupID: &sourceID, Group: &service.Group{ID: sourceID, Platform: service.PlatformOpenAI}, User: &service.User{ID: 9}}
	latest := *conn
	latest.GroupID = &billingID
	latest.Group = &service.Group{ID: billingID, Platform: service.PlatformComposite}
	latest.User = &service.User{ID: 9, GroupAccountExclusions: service.GroupAccountExclusions{sourceID: {101}}}
	ctx := service.WithSystemCustomGroupResolution(context.Background(), service.SystemCustomGroupResolution{
		BillingGroupID: billingID, SourceGroupID: sourceID, SourcePlatform: service.PlatformOpenAI,
		PublicModel: "alias", SourceModel: "gpt-5.4", AllowedAccounts: service.NewSystemCustomGroupAccountAllowlist([]int64{101}),
	})
	got, err := refreshOpenAIWSTurnAPIKey(ctx, wsTurnAPIKeyLookupFunc(func(context.Context, string) (*service.APIKey, error) { return &latest, nil }), conn)
	require.NoError(t, err)
	require.Equal(t, sourceID, *got.GroupID)
	require.Equal(t, latest.User.GroupAccountExclusions, got.User.GroupAccountExclusions)
}
