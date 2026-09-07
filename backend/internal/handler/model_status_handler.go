package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type modelStatusReporter interface {
	Report(context.Context) (*service.ModelStatusReport, error)
}

type ModelStatusHandler struct {
	reporter modelStatusReporter
	renderer modelStatusPNGRenderrer
}

type modelStatusPNGRenderrer interface {
	Fetch(context.Context) ([]byte, string, int, error)
}

type httpModelStatusPNGRenderrer struct {
	url    string
	client *http.Client
}

func (r httpModelStatusPNGRenderrer) Fetch(ctx context.Context) ([]byte, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, "", 0, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Header.Get("Content-Type"), resp.StatusCode, nil
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/png") {
		return nil, resp.Header.Get("Content-Type"), resp.StatusCode, nil
	}
	const maxPNGBytes = 10 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPNGBytes+1))
	if err != nil {
		return nil, "", 0, err
	}
	if len(b) > maxPNGBytes {
		return nil, "", http.StatusRequestEntityTooLarge, nil
	}
	return b, "image/png", resp.StatusCode, nil
}

func NewModelStatusHandler(svc *service.ModelStatusService) *ModelStatusHandler {
	h := &ModelStatusHandler{reporter: svc}
	if url := strings.TrimSpace(os.Getenv("MODEL_STATUS_PNG_RENDERER_URL")); url != "" {
		h.renderer = httpModelStatusPNGRenderrer{url: url, client: &http.Client{Timeout: 15 * time.Second}}
	}
	return h
}

func (h *ModelStatusHandler) GetPNG(c *gin.Context) {
	if h.renderer == nil {
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusNotImplemented)
		c.Writer.WriteHeaderNow()
		return
	}
	b, contentType, status, err := h.renderer.Fetch(c.Request.Context())
	if err != nil || status != http.StatusOK || contentType != "image/png" || len(b) == 0 {
		slog.WarnContext(c.Request.Context(), "model_status_png_failed", "error", err, "status", status)
		c.Header("Retry-After", "15")
		c.Status(http.StatusServiceUnavailable)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusOK, "image/png", b)
}

func (h *ModelStatusHandler) Get(c *gin.Context) {
	// The service shares a short cache; intermediaries must not retain public
	// catalog entries after their visibility changes.
	c.Header("Cache-Control", "no-store")
	report, err := h.reporter.Report(c.Request.Context())
	if err != nil {
		slog.WarnContext(c.Request.Context(), "model_status_report_failed", "error", err)
		c.Header("Retry-After", "15")
		response.Error(c, http.StatusServiceUnavailable, "Model status data is temporarily unavailable")
		return
	}
	response.Success(c, report)
}
