package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportHandlerStub struct {
	allowed     bool
	err         error
	calls       int
	actor       int64
	target      int64
	account     string
	enabled     bool
	searchCalls int
	grantCalls  int
	query       string
}

func (s *supportHandlerStub) CanAccess(context.Context, int64) (bool, error) { return s.allowed, s.err }
func (s *supportHandlerStub) GetAccess(_ context.Context, actor, target int64) (bool, error) {
	s.calls++
	s.actor, s.target = actor, target
	return s.enabled, s.err
}
func (s *supportHandlerStub) SetAccess(_ context.Context, actor, target int64, enabled bool) error {
	s.calls++
	s.actor, s.target, s.enabled = actor, target, enabled
	return s.err
}
func (s *supportHandlerStub) Lookup(_ context.Context, actor int64, account string) (*service.UserSupportResult, error) {
	s.calls++
	s.actor, s.account = actor, account
	return &service.UserSupportResult{}, s.err
}
func (s *supportHandlerStub) Search(_ context.Context, actor int64, query string) ([]service.UserSupportSuggestion, error) {
	s.calls++
	s.searchCalls++
	s.actor, s.query = actor, query
	return []service.UserSupportSuggestion{{ID: 12, Email: "customer@example.com"}}, s.err
}
func (s *supportHandlerStub) Grant(_ context.Context, actor, target int64) (*service.UserSupportGrantResult, error) {
	s.calls++
	s.grantCalls++
	s.actor, s.target = actor, target
	return &service.UserSupportGrantResult{User: service.UserSupportUser{ID: target}, Trial: service.UserSupportTrial{Status: "granted", Amount: 15}}, s.err
}

func supportHandlerRouter(stub *supportHandlerStub, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if role != "" {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 73})
			c.Set(string(middleware.ContextKeyUserRole), role)
			c.Next()
		})
	}
	h := &UserSupportHandler{service: stub}
	r.GET("/access", h.GetMyAccess)
	r.POST("/lookup", h.Lookup)
	if actions, ok := any(h).(interface {
		Search(*gin.Context)
		Grant(*gin.Context)
	}); ok {
		r.POST("/search", actions.Search)
		r.POST("/grant", actions.Grant)
	}
	r.GET("/users/:id/access", h.GetUserAccess)
	r.PUT("/users/:id/access", h.SetUserAccess)
	return r
}

func supportHTTP(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func TestUserSupportHandlerRoleAndRevocation(t *testing.T) {
	for _, role := range []string{"", "user", "sub_admin"} {
		s := &supportHandlerStub{}
		rec := supportHTTP(supportHandlerRouter(s, role), "POST", "/lookup", `{"account":"12"}`)
		want := http.StatusForbidden
		if role == "" {
			want = http.StatusUnauthorized
		}
		require.Equal(t, want, rec.Code, role)
		require.Zero(t, s.calls)
		require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	}
	s := &supportHandlerStub{allowed: true}
	r := supportHandlerRouter(s, "sub_admin")
	require.Equal(t, 200, supportHTTP(r, "POST", "/lookup", `{"account":" 12 "}`).Code)
	require.Equal(t, int64(73), s.actor)
	require.Equal(t, "12", s.account)
	s.allowed = false
	require.Equal(t, 403, supportHTTP(r, "POST", "/lookup", `{"account":"12"}`).Code)
	require.Equal(t, 1, s.calls)
}

func TestUserSupportHandlerOrdinaryUserCannotUseExplicitCapability(t *testing.T) {
	stub := &supportHandlerStub{allowed: true}
	router := supportHandlerRouter(stub, service.RoleUser)
	for _, request := range []struct{ method, path, body string }{
		{"GET", "/access", ""},
		{"POST", "/lookup", `{"account":"12"}`},
	} {
		recorder := supportHTTP(router, request.method, request.path, request.body)
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
	require.Zero(t, stub.calls)
}

func TestUserSupportHandlerRejectsUnspecifiedInputs(t *testing.T) {
	for _, body := range []string{
		``, `null`, `{}`, `{"account":""}`, `{"account":3}`,
		`{"account":"12","amount":300}`, `{"account":"12","page_size":500}`,
		`{"account":"12"} {"account":"13"}`, `{"account":"` + strings.Repeat("a", 3000) + `"}`,
	} {
		s := &supportHandlerStub{allowed: true}
		rec := supportHTTP(supportHandlerRouter(s, "admin"), "POST", "/lookup", body)
		require.Equal(t, 400, rec.Code, body[:min(len(body), 100)])
		require.Zero(t, s.calls)
	}
	s := &supportHandlerStub{allowed: true}
	require.Equal(t, 400, supportHTTP(supportHandlerRouter(s, "admin"), "POST", "/lookup?page_size=100", `{"account":"12"}`).Code)
	require.Zero(t, s.calls)
}

func TestUserSupportHandlerOnlyMainAdminManagesAccess(t *testing.T) {
	for _, role := range []string{"user", "sub_admin"} {
		s := &supportHandlerStub{allowed: true}
		for _, method := range []string{"GET", "PUT"} {
			require.Equal(t, 403, supportHTTP(supportHandlerRouter(s, role), method, "/users/12/access", `{"enabled":true}`).Code)
		}
		require.Zero(t, s.calls)
	}
	s := &supportHandlerStub{}
	r := supportHandlerRouter(s, "admin")
	require.Equal(t, 200, supportHTTP(r, "PUT", "/users/12/access", `{"enabled":true}`).Code)
	require.Equal(t, int64(73), s.actor)
	require.Equal(t, int64(12), s.target)
	require.True(t, s.enabled)
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":true,"role":"admin"}`} {
		require.Equal(t, 400, supportHTTP(r, "PUT", "/users/12/access", body).Code)
	}
	require.Equal(t, 400, supportHTTP(r, "GET", "/users/invalid/access", "").Code)
}

func TestUserSupportHandlerNeverReflectsInternalErrors(t *testing.T) {
	secret := "https://private-origin.invalid:8443/a?key=sk-canary; 10.4.3.2 [2001:db8::1] account=secret"
	for _, method := range []string{"GET", "POST", "PUT"} {
		s := &supportHandlerStub{allowed: true, err: errors.New(secret)}
		path, body := "/access", ""
		if method == "POST" {
			path, body = "/lookup", `{"account":"12"}`
		}
		if method == "PUT" {
			path, body = "/users/12/access", `{"enabled":true}`
		}
		rec := supportHTTP(supportHandlerRouter(s, "admin"), method, path, body)
		require.Equal(t, 503, rec.Code)
		for _, canary := range []string{"private-origin", "8443", "sk-canary", "10.4.3.2", "2001:db8", "account=secret", "metadata"} {
			require.NotContains(t, rec.Body.String(), canary)
		}
		require.Contains(t, rec.Body.String(), "SUPPORT_UNAVAILABLE")
	}
}

func TestUserSupportHandlerSeparatesSuggestionsLookupAndExplicitGrant(t *testing.T) {
	s := &supportHandlerStub{allowed: true}
	r := supportHandlerRouter(s, service.RoleSubAdmin)
	search := supportHTTP(r, "POST", "/search", `{"query":" customer "}`)
	require.Equal(t, 200, search.Code, search.Body.String())
	require.Contains(t, search.Body.String(), `"items"`)
	require.Contains(t, search.Header().Get("Cache-Control"), "no-store")
	require.Equal(t, "customer", s.query)
	require.Equal(t, 1, s.searchCalls)
	require.Zero(t, s.grantCalls)
	require.Equal(t, 200, supportHTTP(r, "POST", "/lookup", `{"account":"12"}`).Code)
	require.Zero(t, s.grantCalls)
	require.Equal(t, 200, supportHTTP(r, "POST", "/grant", `{"user_id":12}`).Code)
	require.Equal(t, int64(73), s.actor)
	require.Equal(t, int64(12), s.target)
	require.Equal(t, 1, s.grantCalls)
	for _, body := range []string{`null`, `{}`, `{"user_id":0}`, `{"user_id":-1}`, `{"user_id":"12"}`, `{"account":"12"}`, `{"user_id":12,"amount":999}`, `{"user_id":12} {"user_id":13}`} {
		require.Equal(t, 400, supportHTTP(r, "POST", "/grant", body).Code, body)
	}
	for _, body := range []string{`null`, `{}`, `{"query":""}`, `{"query":2}`, `{"query":"a","page_size":200}`, `{"query":"a"} {}`} {
		require.Equal(t, 400, supportHTTP(r, "POST", "/search", body).Code, body)
	}
	require.Equal(t, 400, supportHTTP(r, "POST", "/grant?amount=300", `{"user_id":12}`).Code)
	require.Equal(t, 1, s.grantCalls)
	s.allowed = false
	require.Equal(t, 403, supportHTTP(r, "POST", "/grant", `{"user_id":12}`).Code)
	require.Equal(t, 403, supportHTTP(r, "POST", "/search", `{"query":"customer"}`).Code)
	require.Equal(t, 1, s.grantCalls)
	require.Equal(t, 1, s.searchCalls)
}

func TestUserSupportHandlerSearchAndGrantNeverPromoteOrdinaryUsers(t *testing.T) {
	for _, role := range []string{"", service.RoleUser} {
		s := &supportHandlerStub{allowed: true}
		r := supportHandlerRouter(s, role)
		for _, action := range []struct{ path, body string }{{"/search", `{"query":"customer"}`}, {"/grant", `{"user_id":12}`}} {
			rec := supportHTTP(r, "POST", action.path, action.body)
			want := 403
			if role == "" {
				want = 401
			}
			require.Equal(t, want, rec.Code)
		}
		require.Zero(t, s.calls)
	}
}
