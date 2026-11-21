package middleware

import (
	"bytes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInputProtectionHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, path, media, encoding, raw, want string
		status                                 int
		p                                      *service.InputTokenProtection
	}{
		{"pending", "/v1/messages", "application/json", "", `{"usage":{"input_tokens":12,"cache_read_input_tokens":8,"output_tokens":4}}`, `{"usage":{"input_tokens":12,"cache_read_input_tokens":8,"output_tokens":4}}`, 200, &service.InputTokenProtection{Pending: true}},
		{"per request", "/v1/messages", "application/json", "", `{"usage":{"input_tokens":12}}`, `{"usage":{"input_tokens":12}}`, 200, nil},
		{"error", "/v1/responses", "application/json", "", `{"usage":{"input_tokens":12}}`, `{"usage":{"input_tokens":12}}`, 429, &service.InputTokenProtection{Limit: 1}},
		{"encoded", "/v1/responses", "application/json", "gzip", "opaque", "opaque", 200, &service.InputTokenProtection{Limit: 1}},
		{"messages", "/v1/messages", "application/json", "", `{"usage":{"input_tokens":5012,"cache_read_input_tokens":8,"output_tokens":4}}`, `{"usage":{"input_tokens":10,"cache_read_input_tokens":0,"output_tokens":4}}`, 200, &service.InputTokenProtection{Limit: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", tc.path, nil)
			c.Request = c.Request.WithContext(service.WithInputTokenProtection(c.Request.Context(), tc.p))
			original := c.Writer
			InstallInputTokenProtectionWriter(c, nil)
			wrapped := c.Writer
			InstallInputTokenProtectionWriter(c, nil)
			require.Same(t, wrapped, c.Writer)
			if tc.p == nil || tc.p.Pending {
				require.Same(t, original, c.Writer)
			}
			c.Header("Content-Type", tc.media)
			c.Header("Content-Length", "80")
			if tc.encoding != "" {
				c.Header("Content-Encoding", tc.encoding)
			}
			c.Status(tc.status)
			c.Writer.WriteHeaderNow()
			raw := []byte(tc.raw)
			n, err := c.Writer.Write(raw)
			require.NoError(t, err)
			require.Equal(t, len(raw), n)
			require.Equal(t, tc.raw, string(raw))
			require.Equal(t, tc.want, rec.Body.String())
			if tc.p != nil && !tc.p.Pending && tc.encoding == "" && tc.status == 200 {
				require.Empty(t, rec.Result().Header.Get("Content-Length"))
			}
		})
	}
}

func TestInputProtectionFlushAndWriteString(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request = c.Request.WithContext(service.WithInputTokenProtection(c.Request.Context(), &service.InputTokenProtection{Limit: 1}))
	InstallInputTokenProtectionWriter(c, nil)
	c.Header("Content-Type", "text/event-stream")
	prefix := ": ping\n\ndata: "
	_, err := c.Writer.WriteString(prefix)
	require.NoError(t, err)
	c.Writer.Flush()
	require.True(t, rec.Flushed)
	require.Equal(t, prefix, rec.Body.String())
	b := []byte(`{"response":{"usage":{"input_tokens":12345,"output_tokens":2,"total_tokens":12347}}}` + "\n\n")
	src := bytes.Clone(b)
	n, err := c.Writer.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	require.Equal(t, src, b)
	require.Contains(t, rec.Body.String(), `"total_tokens":3`)
}
