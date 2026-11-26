package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const publicGatewayWriterKey = "public_gateway_error_writer"

// PublicGatewayErrorMiddleware wraps the writer before the ops logger observes
// it, so the logger retains the original error and the wire gets a new body.
func PublicGatewayErrorMiddleware(afterIngress bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Writer
		writer := &publicGatewayWriter{ResponseWriter: original, context: c, enabled: !afterIngress}
		c.Writer = writer
		c.Set(publicGatewayWriterKey, writer)
		defer func() { c.Writer = original }()
		c.Next()
		writer.finish()
	}
}

type publicGatewayWriter struct {
	gin.ResponseWriter
	context    *gin.Context
	mu         sync.Mutex
	pending    string
	wroteError bool
	enabled    bool
}

// SSE accepts CR, LF and CRLF, including mixed line endings.
func publicSSEFrameEnd(buffer string) int {
	for i := 0; i < len(buffer); {
		if buffer[i] != '\r' && buffer[i] != '\n' {
			i++
			continue
		}
		next := i + 1
		if buffer[i] == '\r' && next < len(buffer) && buffer[next] == '\n' {
			next++
		}
		if next >= len(buffer) {
			return 0
		}
		switch buffer[next] {
		case '\n':
			return next + 1
		case '\r':
			if next+1 == len(buffer) {
				return 0
			}
			if buffer[next+1] == '\n' {
				return next + 2
			}
			return next + 1
		}
		i = next
	}
	return 0
}

// Gateways arm the public writer after trusted ingress validation has finished.
func PublicGatewayErrorsAfterIngress() gin.HandlerFunc {
	return func(c *gin.Context) {
		if value, ok := c.Get(publicGatewayWriterKey); ok {
			if writer, ok := value.(*publicGatewayWriter); ok {
				writer.enabled = true
			}
		}
		c.Next()
	}
}

func (w *publicGatewayWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *publicGatewayWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.enabled {
		return w.ResponseWriter.Write(body)
	}
	if w.Status() >= 400 || w.wroteError {
		return len(body), w.writeError(body)
	}
	if !w.isStream() {
		if !w.Written() && strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			if safe, ok := publicJSONError(body); ok {
				if gjson.GetBytes(body, "status").String() != "failed" {
					w.WriteHeader(http.StatusBadGateway)
					return len(body), w.writeError(body)
				}
				w.Header().Del("Content-Length")
				_, err := w.ResponseWriter.Write(safe)
				return len(body), err
			}
		}
		return w.ResponseWriter.Write(body)
	}
	// Buffer only the unfinished frame. Gateway writers may split event and data
	// across writes or flush between them.
	if !w.Written() {
		w.Header().Del("Content-Length")
	}
	w.pending += string(body)
	for {
		boundary := publicSSEFrameEnd(w.pending)
		if boundary == 0 {
			break
		}
		frame := w.pending[:boundary]
		w.pending = w.pending[boundary:]
		if err := w.writeFrame(frame); err != nil {
			return 0, err
		}
	}
	if len(w.pending) > 32<<20 {
		w.pending = ""
		w.wroteError = true
		return 0, w.writeFrame("event: error\ndata: {}\n\n")
	}
	return len(body), nil
}

func (w *publicGatewayWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.enabled && w.Status() >= 400 && !w.Written() {
		_ = w.writeError(nil)
	}
	w.ResponseWriter.Flush()
}

func (w *publicGatewayWriter) WriteHeaderNow() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.enabled && w.Status() >= 400 && !w.Written() {
		_ = w.writeError(nil)
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *publicGatewayWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.enabled && w.Status() >= 400 && !w.Written() {
		_ = w.writeError(nil)
		return
	}
	if strings.HasSuffix(w.pending, "\r") {
		w.pending += "\n"
	}
	if w.pending != "" {
		_ = w.writeFrame(w.pending)
		w.pending = ""
	}
}

func (w *publicGatewayWriter) isStream() bool {
	return strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream")
}

func (w *publicGatewayWriter) writeFrame(frame string) error {
	event, payload := parseOpsSSEFrameEnvelope([]byte(frame))
	upstream, _ := w.context.Get(service.OpsUpstreamStatusCodeKey)
	status, _ := upstream.(int)
	if safe, ok := service.PublicGatewayErrorEvent(string(event), payload, status); ok {
		frame = "data: " + string(safe) + "\n\n"
		if string(event) == "error" || string(event) == "response.failed" {
			frame = "event: " + string(event) + "\n" + frame
		}
	}
	_, err := w.ResponseWriter.WriteString(frame)
	return err
}

// Asynchronous task APIs can report failure with HTTP 200. Preserve the
// task envelope while replacing its untrusted diagnostic fields.
func publicJSONError(body []byte) ([]byte, bool) {
	if !gjson.ValidBytes(body) {
		return nil, false
	}
	root := gjson.ParseBytes(body)
	errorValue := root.Get("error")
	if root.Get("status").String() != "failed" && (!errorValue.Exists() || errorValue.Type == gjson.Null) {
		return nil, false
	}
	diagnostic := errorValue.Get("code").String() + " " + errorValue.Get("message").String()
	for _, key := range []string{"error_message", "failure_reason", "message", "detail"} {
		diagnostic += " " + root.Get(key).String()
	}
	public := service.ClassifyPublicGatewayError(0, diagnostic)
	safe, _ := json.Marshal(gin.H{
		"status": "failed",
		"error":  gin.H{"code": public.Code, "message": public.Message},
	})
	return safe, true
}

func (w *publicGatewayWriter) writeError(body []byte) error {
	if w.wroteError {
		return nil
	}
	w.wroteError = true
	upstream, _ := w.context.Get(service.OpsUpstreamStatusCodeKey)
	upstreamStatus, _ := upstream.(int)
	diagnostic := service.ExtractUpstreamErrorMessage(body)
	code := gjson.GetBytes(body, "error.code").String()
	if code == "" {
		code = gjson.GetBytes(body, "code").String()
	}
	errType := gjson.GetBytes(body, "error.type").String()
	if diagnostic == "" {
		diagnostic = string(body)
	}
	upstreamMessage, _ := w.context.Get(service.OpsUpstreamErrorMessageKey)
	if message, ok := upstreamMessage.(string); ok {
		diagnostic += " " + message
	}
	status := upstreamStatus
	if status == 0 {
		status = w.Status()
	}
	public := service.ClassifyPublicGatewayError(status, code+" "+errType+" "+diagnostic)
	if upstreamStatus == 0 {
		if local, ok := service.PublicLocalGatewayError(w.Status(), code, diagnostic); ok {
			public = local
		}
	}
	header := w.Header()
	retryAfter := header.Get("Retry-After")
	requestID, _ := w.context.Request.Context().Value(ctxkey.RequestID).(string)

	for key := range header {
		switch http.CanonicalHeaderKey(key) {
		case "Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Expose-Headers", "Vary", "Strict-Transport-Security", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Content-Security-Policy", "Permissions-Policy":
		default:
			header.Del(key)
		}
	}
	if requestID != "" {
		header.Set("X-Request-ID", requestID)
	}
	if (public.Status == 429 || public.Status == 503) && numericRetryAfter.MatchString(retryAfter) {
		header.Set("Retry-After", retryAfter)
	}
	header.Set("Content-Type", "application/json; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	var envelope any = gin.H{"error": gin.H{"type": public.Type(), "code": public.Code, "message": public.Message}}
	path := w.context.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1beta/") || strings.HasPrefix(path, "/antigravity/v1beta/"):
		envelope = gin.H{"error": gin.H{"code": public.Status, "status": googleapi.HTTPStatusToGoogleStatus(public.Status), "message": public.Message}}
	case strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/messages/count_tokens"):
		envelope = gin.H{"type": "error", "error": gin.H{"type": public.Type(), "message": public.Message}}
	}
	safe, _ := json.Marshal(envelope)
	w.WriteHeader(public.Status)
	_, err := w.ResponseWriter.Write(safe)
	return err
}

var numericRetryAfter = regexp.MustCompile(`^[0-9]{1,9}$`)
