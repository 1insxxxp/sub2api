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
}

func (s modelStatusPNGStub) Fetch(context.Context) ([]byte, string, int, error) {
	return s.body, s.contentType, s.status, nil
}

func TestModelStatusHandlerPNG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelStatusHandler{renderer: modelStatusPNGStub{body: []byte("png"), contentType: "image/png", status: http.StatusOK}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-status/png", nil)
	h.GetPNG(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	b, _ := io.ReadAll(w.Body)
	require.Equal(t, []byte("png"), b)
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
