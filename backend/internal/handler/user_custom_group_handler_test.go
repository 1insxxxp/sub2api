package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
