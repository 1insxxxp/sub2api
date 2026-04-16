package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := int64(16)
	router := gin.New()
	router.Use(middleware.RequestBodyLimit(limit))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": buildBodyTooLargeMessage(maxErr.Limit),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "read_failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	payload := bytes.Repeat([]byte("a"), int(limit+1))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), buildBodyTooLargeMessage(limit))
}

type timeoutReadError struct{}

func (timeoutReadError) Error() string   { return "read timeout" }
func (timeoutReadError) Timeout() bool   { return true }
func (timeoutReadError) Temporary() bool { return false }

func TestClassifyRequestBodyReadError(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		status, errType, message := classifyRequestBodyReadError(context.Canceled)
		require.Equal(t, http.StatusBadRequest, status)
		require.Equal(t, "invalid_request_error", errType)
		require.Equal(t, "Client canceled request while sending request body", message)
	})

	t.Run("unexpected eof", func(t *testing.T) {
		status, errType, message := classifyRequestBodyReadError(io.ErrUnexpectedEOF)
		require.Equal(t, http.StatusBadRequest, status)
		require.Equal(t, "invalid_request_error", errType)
		require.Equal(t, "Request body stream interrupted", message)
	})

	t.Run("read timeout", func(t *testing.T) {
		status, errType, message := classifyRequestBodyReadError(timeoutReadError{})
		require.Equal(t, http.StatusRequestTimeout, status)
		require.Equal(t, "invalid_request_error", errType)
		require.Equal(t, "Request body read timed out", message)
	})
}
