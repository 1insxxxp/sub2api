package middleware

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/wireusage"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// InputProtectionPricingKey identifies the tariff owner after a handler-local
// group retry. Authentication and the opt-in captured at ingress stay separate.
const InputProtectionPricingKey = "gateway.input_protection_pricing_key"

// InstallInputTokenProtectionWriter consults the wire tariff when the first
// byte of a usage object is written, including in fragmented JSON or SSE.
func InstallInputTokenProtectionWriter(c *gin.Context, projectUsage func() bool) {
	if c == nil || c.Request == nil || c.Writer == nil {
		return
	}
	policy := service.InputTokenProtectionFromContext(c.Request.Context())
	if policy == nil || policy.Pending || policy.Limit < 0 {
		return
	}
	if _, ok := c.Writer.(*inputProtectionWriter); ok {
		return
	}
	messages := strings.HasSuffix(strings.TrimRight(c.Request.URL.Path, "/"), "/messages")
	projection := wireusage.New(policy.Cap, messages)
	projection.ProjectStreamUsage = projectUsage
	c.Writer = &inputProtectionWriter{ResponseWriter: c.Writer, projection: projection}
}

type inputProtectionWriter struct {
	gin.ResponseWriter
	projection *wireusage.Projector
	detected   string
}

func (w *inputProtectionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *inputProtectionWriter) media(status int) string {
	if status < 200 || status >= 300 {
		return ""
	}
	header := w.Header()
	if encoding := header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return ""
	}
	media, _, _ := strings.Cut(header.Get("Content-Type"), ";")
	switch strings.ToLower(strings.TrimSpace(media)) {
	case "text/event-stream":
		return "sse"
	case "", "text/plain":
		return "detect"
	case "application/json":
		return "json"
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(media)), "+json") {
		return "json"
	}
	return ""
}

func (w *inputProtectionWriter) prepare(status int) string {
	media := w.media(status)
	if media != "" && !w.Written() {
		w.Header().Del("Content-Length")
	}
	return media
}
func (w *inputProtectionWriter) WriteHeader(code int) {
	w.prepare(code)
	w.ResponseWriter.WriteHeader(code)
}
func (w *inputProtectionWriter) WriteHeaderNow() {
	w.prepare(w.Status())
	w.ResponseWriter.WriteHeaderNow()
}
func (w *inputProtectionWriter) Flush() {
	w.prepare(w.Status())
	w.ResponseWriter.Flush()
}
func (w *inputProtectionWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *inputProtectionWriter) Write(b []byte) (int, error) {
	media := w.prepare(w.Status())
	if media == "detect" {
		if w.detected == "" {
			start := bytes.TrimSpace(b)
			if len(start) == 0 {
				return w.ResponseWriter.Write(b)
			}
			switch start[0] {
			case '{', '[':
				w.detected = "json"
			case ':', 'd', 'e', 'i', 'r':
				w.detected = "sse"
			default:
				w.detected = "other"
			}
		}
		media = w.detected
	}
	switch media {
	case "json":
		return w.projection.WriteJSON(w.ResponseWriter, b)
	case "sse":
		return w.projection.WriteSSE(w.ResponseWriter, b)
	default:
		return w.ResponseWriter.Write(b)
	}
}
