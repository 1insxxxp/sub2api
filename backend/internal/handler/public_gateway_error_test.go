package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicGatewayErrorMiddleware(t *testing.T) {
	t.Run("HTTP error", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1beta/models/x", func(c *gin.Context) {
			c.Header("Location", "https://upstream.invalid")
			c.JSON(400, gin.H{"error": gin.H{"code": 400, "status": "INVALID_ARGUMENT", "message": "antigravity auth missing project_id", "details": "hidden"}})
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1beta/models/x", nil))
		require.Equal(t, 502, out.Code)
		require.Empty(t, out.Header().Get("Location"))
		require.NotContains(t, out.Body.String(), "antigravity")
		require.Contains(t, out.Body.String(), "INTERNAL")
	})
	t.Run("unknown 400", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1/chat/completions", func(c *gin.Context) {
			c.JSON(400, gin.H{"error": gin.H{"message": "unknown failure at private.example"}})
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil))
		require.Equal(t, 400, out.Code)
		require.JSONEq(t, `{"error":{"type":"invalid_request_error","code":"invalid_request","message":"参数异常"}}`, out.Body.String())
	})
	t.Run("split stream error", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1/responses", func(c *gin.Context) {
			c.Header("Content-Type", "text/event-stream")
			_, _ = c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
			c.Writer.Flush()
			_, _ = c.Writer.WriteString("event: error\r\ndata: {\"error\":{\"message\":\"antigravity ")
			c.Writer.Flush()
			_, _ = c.Writer.WriteString("auth missing project_id\"}}\r\n\r\n")
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
		require.Equal(t, 200, out.Code)
		require.Contains(t, out.Body.String(), "hello")
		require.Contains(t, out.Body.String(), "service_unavailable")
		require.NotContains(t, out.Body.String(), "antigravity")
	})
}
