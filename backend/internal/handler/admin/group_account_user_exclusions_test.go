package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupAccountUserExclusionsDenyNonSuperAdmins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"", service.RoleUser, service.RoleSubAdmin} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			router := gin.New()
			if role != "" {
				router.Use(func(c *gin.Context) {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
					c.Set(string(middleware.ContextKeyUserRole), role)
					c.Next()
				})
			}
			h := &GroupHandler{}
			router.GET("/groups/:id/account-user-exclusions", h.ListAccountUserExclusions)
			router.POST("/groups/:id/account-user-exclusions", h.AddAccountUserExclusion)
			router.DELETE("/groups/:id/account-user-exclusions/:exclusion_id", h.RemoveAccountUserExclusion)
			path := "/groups/10/account-user-exclusions"
			if method == http.MethodDelete {
				path += "/5"
			}
			req := httptest.NewRequest(method, path, strings.NewReader(`{"account_id":21,"user_id":7}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			status := http.StatusForbidden
			if role == "" {
				status = http.StatusUnauthorized
			}
			require.Equal(t, status, rec.Code, role+" "+method)
			require.NotContains(t, rec.Body.String(), "user_email")
		}
	}
}

type groupAccountExclusionHandlerRepo struct {
	calls                                   int
	groupID, accountID, userID, exclusionID int64
	params                                  pagination.PaginationParams
}

func (r *groupAccountExclusionHandlerRepo) List(_ context.Context, groupID int64, params pagination.PaginationParams) ([]service.GroupAccountUserExclusion, pagination.PaginationResult, error) {
	r.calls++
	r.groupID = groupID
	r.params = params
	return []service.GroupAccountUserExclusion{{ID: 5, GroupID: groupID, AccountID: 21, AccountName: "Private account", UserID: 7, UserEmail: "user@example.com", Username: "Customer", Notes: "Private note", CreatedAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)}}, pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize, Total: 1}, nil
}
func (r *groupAccountExclusionHandlerRepo) Add(_ context.Context, g, a, u int64) (*service.GroupAccountUserExclusion, error) {
	r.calls++
	r.groupID = g
	r.accountID = a
	r.userID = u
	return &service.GroupAccountUserExclusion{ID: 5, GroupID: g, AccountID: a, UserID: u}, nil
}
func (r *groupAccountExclusionHandlerRepo) Remove(_ context.Context, g, id int64) (int64, error) {
	r.calls++
	r.groupID = g
	r.exclusionID = id
	return 7, nil
}

func TestGroupAccountUserExclusionAdminAPIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &groupAccountExclusionHandlerRepo{}
	h := &GroupHandler{accountUserExclusions: service.NewGroupAccountUserExclusionService(repo, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	router.GET("/groups/:id/account-user-exclusions", h.ListAccountUserExclusions)
	router.POST("/groups/:id/account-user-exclusions", h.AddAccountUserExclusion)
	router.DELETE("/groups/:id/account-user-exclusions/:exclusion_id", h.RemoveAccountUserExclusion)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/groups/10/account-user-exclusions?page=2&page_size=5", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data struct {
			Items    []service.GroupAccountUserExclusion `json:"items"`
			Total    int64                               `json:"total"`
			Page     int                                 `json:"page"`
			PageSize int                                 `json:"page_size"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Data.Page)
	require.Equal(t, 5, body.Data.PageSize)
	require.EqualValues(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	require.Equal(t, "Customer", body.Data.Items[0].Username)
	require.Equal(t, "Private note", body.Data.Items[0].Notes)
	require.Equal(t, "Private account", body.Data.Items[0].AccountName)
	req := httptest.NewRequest(http.MethodPost, "/groups/10/account-user-exclusions", strings.NewReader(`{"account_id":21,"user_id":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 21, repo.accountID)
	require.EqualValues(t, 7, repo.userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/groups/10/account-user-exclusions/5", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 10, repo.groupID)
	require.EqualValues(t, 5, repo.exclusionID)
	calls := repo.calls
	for _, path := range []string{"/groups/0/account-user-exclusions", "/groups/no/account-user-exclusions"} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	for _, payload := range []string{`{"account_id":0,"user_id":7}`, `{"account_id":21,"user_id":-1}`, `{"account_id":21}`} {
		req = httptest.NewRequest(http.MethodPost, "/groups/10/account-user-exclusions", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	require.Equal(t, calls, repo.calls, "invalid inputs cannot touch private storage")
}
