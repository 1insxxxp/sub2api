package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCheckinBrowserCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://example.com/checkin", nil)
	hash, found, err := checkinBrowserIdentity(c)
	require.NoError(t, err)
	require.False(t, found)
	require.Len(t, hash, 64)
	cookie := recorder.Result().Cookies()[0]
	require.True(t, cookie.HttpOnly)
	require.True(t, cookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://example.com/checkin", nil)
	c.Request.AddCookie(cookie)
	again, found, err := checkinBrowserIdentity(c)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, hash, again)
}

func TestCheckinBrowserRejectsInventedCookieAndReusesIssuedValue(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "https://example.com", nil)
	c.Request.AddCookie(&http.Cookie{Name: checkinBrowserCookie, Value: strings.Repeat("21", 32)})
	hash, found, err := checkinBrowserIdentity(c)
	require.NoError(t, err)
	require.False(t, found)
	again, found, err := checkinBrowserIdentity(c)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, hash, again)
	require.Len(t, rec.Result().Cookies(), 1)
}

func TestCheckinBrowserIdentityAcrossInstances(t *testing.T) {
	getCookie := func(secret string, cookie *http.Cookie) (string, bool, *http.Cookie) {
		t.Helper()
		var hash string
		var present bool
		router := gin.New()
		router.Use(BrowserCookieSigner(secret))
		router.GET("/", func(c *gin.Context) {
			var err error
			hash, present, err = checkinBrowserIdentity(c)
			require.NoError(t, err)
			c.Status(http.StatusOK)
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://test.invalid/", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		router.ServeHTTP(rec, req)
		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			return hash, present, nil
		}
		return hash, present, cookies[0]
	}
	a, present, cookie := getCookie("cluster-secret", nil)
	require.False(t, present)
	require.NotNil(t, cookie)
	b, present, _ := getCookie("cluster-secret", cookie)
	require.True(t, present)
	require.Equal(t, a, b)
	c, present, _ := getCookie("changed-secret", cookie)
	require.False(t, present)
	require.NotEqual(t, a, c)
}
