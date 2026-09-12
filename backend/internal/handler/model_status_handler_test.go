//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelStatusPNGStub struct {
	body        []byte
	status      int
	contentType string
	query       string
}

func (s *modelStatusPNGStub) Fetch(_ context.Context, query string) ([]byte, string, int, error) {
	s.query = query
	return s.body, s.contentType, s.status, nil
}

func TestModelStatusHandlerPNG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	renderer := &modelStatusPNGStub{body: []byte("png"), contentType: "image/png", status: http.StatusOK}
	h := &ModelStatusHandler{renderer: renderer}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-status/png?search=gemini%20pro", nil)
	h.GetPNG(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	b, _ := io.ReadAll(w.Body)
	require.Equal(t, []byte("png"), b)
	require.Equal(t, "search=gemini%20pro", renderer.query)
}

func TestHTTPModelStatusPNGRendererForwardsOnlySearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Pro稳定", r.URL.Query().Get("search"))
		require.Empty(t, r.URL.Query().Get("url"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer srv.Close()
	r := httpModelStatusPNGRenderer{url: srv.URL + "?search=old", client: srv.Client()}
	body, _, status, err := r.Fetch(context.Background(), "search=Pro%E7%A8%B3%E5%AE%9A&url=http://untrusted")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, []byte("png"), body)
}

func TestModelStatusHandlerPNGUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelStatusHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-status/png", nil)
	h.GetPNG(c)
	require.Equal(t, http.StatusNotImplemented, w.Code)
}

type modelStatusReporterStub struct {
	report *service.ModelStatusReport
	err    error
}

func (s modelStatusReporterStub) Report(context.Context) (*service.ModelStatusReport, error) {
	return s.report, s.err
}

func TestModelStatusHandlerAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelStatusHandler{reporter: modelStatusReporterStub{report: &service.ModelStatusReport{}}}
	r := gin.New()
	r.GET("/api/v1/model-status", h.Get)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/model-status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"code":0`)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestModelStatusHandlerUnavailableDoesNotLeakErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelStatusHandler{reporter: modelStatusReporterStub{err: errors.New("database password and private account")}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-status", nil)
	h.Get(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "password")
	require.NotContains(t, w.Body.String(), "private account")
	require.NotContains(t, w.Body.String(), `"total":0`)
	require.Equal(t, "15", w.Header().Get("Retry-After"))
}
