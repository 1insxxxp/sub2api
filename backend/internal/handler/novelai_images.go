package handler

import (
	"bytes"
	"io"
	"net/http"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// NovelAIImages accepts NovelAI's native JSON contract and hands a small
// OpenAI-shaped routing view to the shared image scheduler. The original body
// is carried in the request context and is forwarded unchanged to accounts
// configured with the NovelAI image protocol.
// POST /ai/generate-image
func (h *OpenAIGatewayHandler) NovelAIImages(c *gin.Context) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	request, err := service.ParseNovelAIImageRequestBody(body, c.GetHeader("Content-Type"))
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	routingBody, err := service.BuildOpenAIImageRoutingBody(request)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	originalBody := c.Request.Body
	originalLength := c.Request.ContentLength
	originalType := c.GetHeader("Content-Type")
	c.Request.Body = io.NopCloser(bytes.NewReader(routingBody))
	c.Request.ContentLength = int64(len(routingBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request = c.Request.WithContext(service.WithNovelAIImageRequest(c.Request.Context(), request))
	defer func() {
		c.Request.Body = originalBody
		c.Request.ContentLength = originalLength
		c.Request.Header.Set("Content-Type", originalType)
	}()

	h.Images(c)
}
