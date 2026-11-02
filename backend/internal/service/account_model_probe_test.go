//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelProbeRequiresSuccessfulTerminalEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, stream, status, code string
		err                        error
	}{
		{name: "complete", stream: "data: {\"type\":\"content\",\"text\":\"private response\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n", status: "success"},
		{name: "empty", status: "failed", code: "probe_incomplete"},
		{name: "EOF after content", stream: "data: {\"type\":\"content\",\"text\":\"hi\"}\n\n", status: "failed", code: "probe_incomplete"},
		{name: "unterminated completion", stream: "data: {\"type\":\"test_complete\",\"success\":true}", status: "failed", code: "probe_incomplete"},
		{name: "unsuccessful completion", stream: "data: {\"type\":\"test_complete\",\"success\":false}\n\n", status: "failed", code: "probe_failed"},
		{name: "missing success", stream: "data: {\"type\":\"test_complete\"}\n\n", status: "failed", code: "probe_failed"},
		{name: "later unsuccessful completion", stream: "data: {\"type\":\"test_complete\",\"success\":true}\n\ndata: {\"type\":\"test_complete\",\"success\":false}\n\n", status: "failed", code: "probe_failed"},
		{name: "error before completion", stream: "data: {\"type\":\"error\",\"error\":\"https://secret.invalid sk-secret\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n", status: "failed", code: "probe_failed"},
		{name: "error after completion", stream: "data: {\"type\":\"test_complete\",\"success\":true}\n\ndata: {\"type\":\"error\",\"error\":\"private\"}\n\n", status: "failed", code: "probe_failed"},
		{name: "malformed before completion", stream: "data: {no json}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n", status: "failed", code: "probe_failed"},
		{name: "callback failure", stream: "data: {\"type\":\"test_complete\",\"success\":true}\n\n", err: errors.New("private upstream error"), status: "failed", code: "probe_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := runStrictModelProbe(context.Background(), func(c *gin.Context) error {
				_, _ = c.Writer.Write([]byte(tc.stream))
				return tc.err
			})
			require.NoError(t, err)
			require.Equal(t, ModelProbeExecution{Status: tc.status, ErrorCode: tc.code}, result)
		})
	}
}

func TestModelProbeCaptureMemoryRemainsBounded(t *testing.T) {
	w := &modelProbeCapture{}
	_, _ = w.Write([]byte("data: {\"type\":\"image\",\"image_url\":\""))
	for i := 0; i < 1024; i++ {
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
		require.LessOrEqual(t, len(w.line), modelProbeEventLimit)
		require.LessOrEqual(t, len(w.data), modelProbeEventLimit)
	}
	_, _ = w.Write([]byte("\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n"))
	require.True(t, w.complete)
	require.False(t, w.failed)
}

func TestModelProbeContextWinsOverSuccessfulOutput(t *testing.T) {
	for _, tc := range []struct {
		name, status, code string
		timeout            bool
	}{
		{name: "cancelled", status: "cancelled", code: "probe_cancelled"},
		{name: "deadline", status: "timeout", code: "probe_timeout", timeout: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if tc.timeout {
				ctx, cancel = context.WithTimeout(context.Background(), -1)
			}
			defer cancel()
			called := false
			result, err := runStrictModelProbe(ctx, func(c *gin.Context) error {
				called = true
				cancel()
				_, _ = c.Writer.WriteString("data: {\"type\":\"test_complete\",\"success\":true}\n\n")
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, ModelProbeExecution{Status: tc.status, ErrorCode: tc.code}, result)
			require.Equal(t, !tc.timeout, called)
		})
	}
}

func TestModelProbeCaptureBoundsLargeMediaAndKeepsFollowingCompletion(t *testing.T) {
	for _, kind := range []string{"content", "image", "audio", "video"} {
		t.Run(kind, func(t *testing.T) {
			result, err := runStrictModelProbe(context.Background(), func(c *gin.Context) error {
				_, _ = fmt.Fprintf(c.Writer, "data: {\"type\":%q,\"text\":\"", kind)
				for i := 0; i < 4096; i++ {
					_, _ = c.Writer.WriteString(strings.Repeat("x", 1024))
				}
				_, _ = c.Writer.WriteString("\"}\r\n\r\ndata: {\"type\":\"test_complete\",\"success\":true}\r\n\r\n")
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, ModelProbeExecution{Status: "success"}, result)
		})
	}
}

func TestModelProbeOversizedErrorCannotTurnIntoSuccess(t *testing.T) {
	for _, prefix := range []string{"{\"type\":\"error\",\"error\":\"", "{\"other\":\""} {
		result, err := runStrictModelProbe(context.Background(), func(c *gin.Context) error {
			_, _ = c.Writer.WriteString("data: " + prefix + strings.Repeat("x", 200000) + "\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n")
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, ModelProbeExecution{Status: "failed", ErrorCode: "probe_failed"}, result)
	}
}

func TestModelProbeCaptureSupportsChunkedMultilineAndHeartbeat(t *testing.T) {
	stream := ": heartbeat\r\n\r\nevent: probe\r\ndata: {\"type\":\"test_complete\",\r\ndata: \"success\":true}\r\n\r\n"
	result, err := runStrictModelProbe(context.Background(), func(c *gin.Context) error {
		for i := range []byte(stream) {
			_, _ = c.Writer.Write([]byte{stream[i]})
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, ModelProbeExecution{Status: "success"}, result)
}

func TestModelProbeHTTPFailureCannotTurnIntoSuccess(t *testing.T) {
	result, err := runStrictModelProbe(context.Background(), func(c *gin.Context) error {
		c.Status(502)
		_, _ = c.Writer.WriteString("data: {\"type\":\"test_complete\",\"success\":true}\n\n")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, ModelProbeExecution{Status: "failed", ErrorCode: "probe_failed"}, result)
}
