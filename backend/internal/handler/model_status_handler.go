package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type modelStatusReporter interface {
	Report(context.Context) (*service.ModelStatusReport, error)
}

type ModelStatusHandler struct {
	reporter modelStatusReporter
}

func NewModelStatusHandler(svc *service.ModelStatusService) *ModelStatusHandler {
	return &ModelStatusHandler{reporter: svc}
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
