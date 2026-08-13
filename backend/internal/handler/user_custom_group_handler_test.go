package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type customGroupHandlerRepoStub struct {
	service.UserCustomGroupRepository
	boundCount       int
	forceDeleteCount int
	forceCalls       int
}

type customGroupCandidateUserRepoStub struct{ service.UserRepository }

func (customGroupCandidateUserRepoStub) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 9}, nil
}

type customGroupCandidateGroupRepoStub struct{ service.GroupRepository }

func (customGroupCandidateGroupRepoStub) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{{ID: 11, Name: "Empty source", Platform: service.PlatformAnthropic, Status: service.StatusActive}}, nil
}

type customGroupCandidateSubscriptionRepoStub struct {
	service.UserSubscriptionRepository
}

func (customGroupCandidateSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return nil, nil
}

type customGroupCandidateAccountRepoStub struct{ service.AccountRepository }

func (customGroupCandidateAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return nil, nil
}

func (s *customGroupHandlerRepoStub) GetOwned(context.Context, int64, int64) (*service.UserCustomGroup, error) {
	return &service.UserCustomGroup{ID: 21, UserID: 9}, nil
}

func (s *customGroupHandlerRepoStub) CountBoundAPIKeys(context.Context, int64) (int, error) {
	return s.boundCount, nil
}

func (s *customGroupHandlerRepoStub) Delete(context.Context, int64, int64) error { return nil }

func (s *customGroupHandlerRepoStub) DeleteAndUnbindAPIKeys(context.Context, int64, int64) (int, error) {
	s.forceCalls++
	return s.forceDeleteCount, nil
}

func performCustomGroupDelete(t *testing.T, repo *customGroupHandlerRepoStub, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/custom-groups/21"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: "21"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
	h := NewUserCustomGroupHandler(service.NewUserCustomGroupService(repo, nil, nil, nil, nil), nil, nil)
	h.Delete(c)
	return w
}

func TestUserCustomGroupHandlerDeleteReturnsBoundKeyCount(t *testing.T) {
	w := performCustomGroupDelete(t, &customGroupHandlerRepoStub{boundCount: 2}, "")

	require.Equal(t, http.StatusConflict, w.Code)
	var body struct {
		Reason   string            `json:"reason"`
		Metadata map[string]string `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "CUSTOM_GROUP_IN_USE", body.Reason)
	require.Equal(t, "2", body.Metadata["bound_api_key_count"])
}

func TestUserCustomGroupHandlerDeleteForceReturnsUnboundCount(t *testing.T) {
	repo := &customGroupHandlerRepoStub{boundCount: 2, forceDeleteCount: 2}
	w := performCustomGroupDelete(t, repo, "?force=true")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.forceCalls)
	var body struct {
		Data struct {
			Deleted            bool `json:"deleted"`
			UnboundAPIKeyCount int  `json:"unbound_api_key_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Data.Deleted)
	require.Equal(t, 2, body.Data.UnboundAPIKeyCount)
}

func TestUserCustomGroupHandlerDeleteRejectsInvalidForce(t *testing.T) {
	repo := &customGroupHandlerRepoStub{}
	w := performCustomGroupDelete(t, repo, "?force=definitely")

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, repo.forceCalls)
}

func TestUserCustomGroupHandlerCandidatesSerializesEmptyModelsAsArray(t *testing.T) {
	apiKeys := service.NewAPIKeyService(
		nil,
		customGroupCandidateUserRepoStub{},
		customGroupCandidateGroupRepoStub{},
		customGroupCandidateSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
	gateway := service.NewGatewayService(
		customGroupCandidateAccountRepoStub{}, nil, nil, nil, nil, nil, nil, nil,
		&config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := NewUserCustomGroupHandler(nil, apiKeys, gateway)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/custom-groups/candidates", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
	h.Candidates(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data []struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.NotNil(t, body.Data[0].Models)
	require.Empty(t, body.Data[0].Models)
}
