package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type modelProbeForwardResult struct {
	Outcome      *service.ResponseOutcome
	FirstTokenMs *int
	MediaSuccess bool
}

// forwardModelProbe owns only one selected account's production forwarding.
// The caller owns admission, failover, the bounded sink and the overall deadline.
// No customer identity, usage recording or account selection belongs here.
func (h *GatewayHandler) forwardModelProbe(c *gin.Context, r *modelProbeRoute, a *service.Account) (*modelProbeForwardResult, error) {
	if h == nil || c == nil || c.Request == nil || r == nil || a == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	if err := c.Request.Context().Err(); err != nil {
		return nil, err
	}
	mapped := a.GetMappedModel(r.forwardModel)
	if !modelProbeForwardSupportsMode(mapped) {
		return nil, service.ErrModelProbeUnsupported
	}
	if probeImageDenied(r.billingGroup, r.publicModel, r.routeModel, r.forwardModel, mapped) {
		return nil, service.ErrModelProbeUnsupported
	}
	endpoint := modelProbeForwardEndpoint(r, a, mapped)
	if endpoint == service.CompositeRouteEndpointImages || endpoint == modelProbeVideoEndpoint {
		return h.forwardModelProbeMedia(c, r, a, endpoint, mapped)
	}
	body, path, err := modelProbeTextBody(endpoint, r.forwardModel, mapped)
	if err != nil {
		return nil, err
	}
	modelProbeInstallRequest(c, path, body)
	return h.forwardModelProbeText(c, r, a, endpoint, body)
}

const modelProbeVideoEndpoint = "/v1/videos/generations"

func modelProbeForwardEndpoint(r *modelProbeRoute, a *service.Account, mapped string) string {
	if r.endpoint != "" && r.endpoint != service.CompositeRouteEndpointAny {
		return r.endpoint
	}
	if a.Platform == service.PlatformGrok && modelProbeGrokVideo(mapped) {
		return modelProbeVideoEndpoint
	}
	if endpoint := service.ModelProbeNativeEndpoint(a, r.forwardModel); endpoint != "" {
		return endpoint
	}
	// Adaptive and model-routed providers resolve their actual upstream protocol
	// inside the production OpenAI adapter, including the provider's model catalog.
	if domain.UsesOpenAIGateway(a.Platform) {
		return service.CompositeRouteEndpointResponses
	}
	return service.CompositeRouteEndpointMessages
}

// Keep the dedicated-mode boundary aligned with account_model_probe_mode.go.
// Match the final account target so public/channel aliases cannot hide a mode.
func modelProbeForwardSupportsMode(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndexByte(model, '/'); slash >= 0 {
		model = model[slash+1:]
	}
	for _, prefix := range []string{"text-embedding-", "gemini-embedding-", "embedding-", "text-moderation-", "omni-moderation-", "tts-", "whisper-", "grok-voice", "grok-tts", "grok-stt", "grok-web-search", "dall-e-", "imagen-", "veo-", "nai-diffusion-", "sora-"} {
		if strings.HasPrefix(model, prefix) {
			return false
		}
	}
	for _, family := range []string{"realtime", "-transcribe", "-tts", "-stt", "-audio"} {
		if strings.Contains(model, family) {
			return false
		}
	}
	return true
}

func modelProbeGrokVideo(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "grok-imagine-video") || strings.HasPrefix(model, "grok-video")
}

func modelProbeGeminiImage(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "gemini") && strings.Contains(model, "image")
}

func modelProbeTextBody(endpoint, model, mapped string) ([]byte, string, error) {
	const prompt = "Reply with OK."
	payload := map[string]any{"model": model, "stream": true}
	var path string
	switch endpoint {
	case service.CompositeRouteEndpointResponses:
		path = "/v1/responses"
		payload["input"] = prompt
		payload["instructions"] = "Answer briefly."
		payload["max_output_tokens"] = 1024
	case service.CompositeRouteEndpointChatCompletions, service.CompositeRouteEndpointMessages:
		path = "/v1/messages"
		payload["messages"] = []any{map[string]any{"role": "user", "content": prompt}}
		payload["max_tokens"] = 1024
		if endpoint == service.CompositeRouteEndpointChatCompletions {
			path = "/v1/chat/completions"
			payload["stream_options"] = map[string]any{"include_usage": true}
		}
	case service.CompositeRouteEndpointGemini:
		path = "/v1beta/models/" + url.PathEscape(model) + ":streamGenerateContent"
		payload = map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}}}}}
		if modelProbeGeminiImage(mapped) {
			path = "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
			payload["contents"] = []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "Draw a small blue circle on a white background."}}}}
			payload["generationConfig"] = map[string]any{"responseModalities": []string{"TEXT", "IMAGE"}, "imageConfig": map[string]any{"aspectRatio": "1:1"}}
		}
	default:
		return nil, "", service.ErrModelProbeUnsupported
	}
	body, err := json.Marshal(payload)
	return body, path, err
}

func modelProbeInstallRequest(c *gin.Context, path string, body []byte) {
	// Preserve the parent's per-attempt context and independent session headers.
	req := c.Request.Clone(c.Request.Context())
	req.Method = http.MethodPost
	req.URL.Path, req.URL.RawPath, req.URL.RawQuery = path, "", ""
	req.RequestURI = path
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	c.Request = req
}

func (h *GatewayHandler) forwardModelProbeText(c *gin.Context, r *modelProbeRoute, a *service.Account, endpoint string, body []byte) (*modelProbeForwardResult, error) {
	if domain.UsesOpenAIGateway(a.Platform) {
		return h.forwardModelProbeOpenAI(c, a, endpoint, body)
	}
	ctx := c.Request.Context()
	var result *service.ForwardResult
	var err error
	if endpoint == service.CompositeRouteEndpointGemini {
		stream := !strings.HasSuffix(c.Request.URL.Path, ":generateContent")
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		if a.Platform == service.PlatformAntigravity && a.Type != service.AccountTypeAPIKey {
			if h.antigravityGatewayService == nil {
				return nil, service.ErrUserSupportUnavailable
			}
			result, err = h.antigravityGatewayService.ForwardGemini(ctx, c, a, r.forwardModel, action, stream, body, false)
		} else {
			if h.geminiCompatService == nil {
				return nil, service.ErrUserSupportUnavailable
			}
			result, err = h.geminiCompatService.ForwardNative(ctx, c, a, r.forwardModel, action, stream, body)
		}
	} else if a.Platform == service.PlatformGemini {
		if h.geminiCompatService == nil {
			return nil, service.ErrUserSupportUnavailable
		}
		switch endpoint {
		case service.CompositeRouteEndpointMessages:
			result, err = h.geminiCompatService.Forward(ctx, c, a, body)
		case service.CompositeRouteEndpointChatCompletions:
			result, err = h.geminiCompatService.ForwardAsChatCompletions(ctx, c, a, body)
		default:
			return nil, service.ErrModelProbeUnsupported
		}
	} else {
		result, err = h.forwardModelProbeClaude(c, r, a, endpoint, body)
	}
	if result == nil {
		return nil, err
	}
	return &modelProbeForwardResult{Outcome: result.Outcome, FirstTokenMs: result.FirstTokenMs}, err
}

func (h *GatewayHandler) forwardModelProbeClaude(c *gin.Context, r *modelProbeRoute, a *service.Account, endpoint string, body []byte) (*service.ForwardResult, error) {
	ctx := c.Request.Context()
	if a.Platform == service.PlatformAntigravity && a.Type != service.AccountTypeAPIKey {
		if h.antigravityGatewayService == nil {
			return nil, service.ErrUserSupportUnavailable
		}
		switch endpoint {
		case service.CompositeRouteEndpointMessages:
			return h.antigravityGatewayService.Forward(ctx, c, a, body, false)
		case service.CompositeRouteEndpointChatCompletions:
			return h.antigravityGatewayService.ForwardAsChatCompletions(ctx, c, a, body, nil)
		case service.CompositeRouteEndpointResponses:
			return h.antigravityGatewayService.ForwardAsResponses(ctx, c, a, body, nil)
		}
	}
	if h.gatewayService == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	parsed, err := service.ParseGatewayRequest(service.NewRequestBodyRef(body), endpoint)
	if err != nil {
		return nil, err
	}
	parsed.GroupID = &r.group.ID
	switch endpoint {
	case service.CompositeRouteEndpointMessages:
		return h.gatewayService.Forward(ctx, c, a, parsed)
	case service.CompositeRouteEndpointChatCompletions:
		return h.gatewayService.ForwardAsChatCompletions(ctx, c, a, body, parsed)
	case service.CompositeRouteEndpointResponses:
		return h.gatewayService.ForwardAsResponses(ctx, c, a, body, parsed)
	default:
		return nil, service.ErrModelProbeUnsupported
	}
}

func (h *GatewayHandler) forwardModelProbeOpenAI(c *gin.Context, a *service.Account, endpoint string, body []byte) (*modelProbeForwardResult, error) {
	if h.openAIGatewayService == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	var result *service.OpenAIForwardResult
	var err error
	switch endpoint {
	case service.CompositeRouteEndpointResponses:
		result, err = h.openAIGatewayService.Forward(c.Request.Context(), c, a, body)
	case service.CompositeRouteEndpointChatCompletions:
		result, err = h.openAIGatewayService.ForwardAsChatCompletions(c.Request.Context(), c, a, body, "", "")
	case service.CompositeRouteEndpointMessages:
		result, err = h.openAIGatewayService.ForwardAsAnthropic(c.Request.Context(), c, a, body, "", "")
	default:
		return nil, service.ErrModelProbeUnsupported
	}
	return modelProbeOpenAIResult(result), err
}

func modelProbeOpenAIResult(result *service.OpenAIForwardResult) *modelProbeForwardResult {
	if result == nil {
		return nil
	}
	return &modelProbeForwardResult{Outcome: result.Outcome, FirstTokenMs: result.FirstTokenMs}
}

func (h *GatewayHandler) forwardModelProbeMedia(c *gin.Context, r *modelProbeRoute, a *service.Account, endpoint, mapped string) (*modelProbeForwardResult, error) {
	if h.openAIGatewayService == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	payload := map[string]any{"model": r.forwardModel, "prompt": "Draw a small blue circle on a white background.", "n": 1}
	if endpoint == modelProbeVideoEndpoint {
		payload = map[string]any{"model": r.forwardModel, "prompt": "A blue circle moving slowly on a white background.", "duration": 6, "aspect_ratio": "16:9", "resolution": "480p"}
	} else if a.Platform != service.PlatformGrok {
		// Native images validate the request model before account mapping. Resolve
		// aliases once, then use a request-local identity mapping for the target.
		// Never mutate the scheduler's account or allow a chained target mapping.
		a = modelProbeResolvedImageAccount(a, mapped)
		payload["model"] = mapped
		if size := a.OpenAIImageTestSize(); size != "" {
			payload["size"] = size
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	path := "/v1/images/generations"
	if endpoint == modelProbeVideoEndpoint {
		path = modelProbeVideoEndpoint
	}
	modelProbeInstallRequest(c, path, body)
	if a.Platform == service.PlatformGrok {
		if endpoint == modelProbeVideoEndpoint {
			return h.forwardModelProbeVideo(c, a, body)
		}
		result, err := h.openAIGatewayService.ForwardGrokMedia(c.Request.Context(), c, a, service.GrokMediaEndpointImagesGenerations, "", body, "application/json")
		probe := modelProbeOpenAIResult(result)
		if probe != nil {
			// Grok's media adapter rejects responses without actual image outputs.
			probe.MediaSuccess = err == nil && result.ImageCount > 0
		}
		return probe, err
	}
	ctx := service.WithOpenAIImagesEndpoint(service.WithOpenAIImageGenerationIntent(c.Request.Context()))
	c.Request = c.Request.WithContext(ctx)
	parsed, err := h.openAIGatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		return nil, err
	}
	result, err := h.openAIGatewayService.ForwardImages(ctx, c, a, body, parsed, mapped)
	probe := modelProbeOpenAIResult(result)
	if probe != nil && err == nil {
		if result.Outcome != nil {
			probe.MediaSuccess = result.Outcome.StreamCompleted && result.Outcome.HasMedia
		} else if a.Type == service.AccountTypeOAuth || a.Type == service.AccountTypeSetupToken {
			// OAuth's non-streaming images adapters require a final image result
			// before returning nil error, but do not currently attach an outcome.
			probe.MediaSuccess = result.ImageCount > 0
		}
	}
	return probe, err
}

func modelProbeResolvedImageAccount(a *service.Account, mapped string) *service.Account {
	clone := *a
	clone.Credentials = make(map[string]any, len(a.Credentials))
	for key, value := range a.Credentials {
		clone.Credentials[key] = value
	}
	clone.Credentials["model_mapping"] = map[string]any{mapped: mapped}
	return &clone
}

func (h *GatewayHandler) forwardModelProbeVideo(c *gin.Context, a *service.Account, body []byte) (*modelProbeForwardResult, error) {
	result, err := h.openAIGatewayService.ForwardGrokMedia(c.Request.Context(), c, a, service.GrokMediaEndpointVideosGenerations, "", body, "application/json")
	if err != nil {
		return modelProbeOpenAIResult(result), err
	}
	// An accepted async request must never be replayed, including if the task ID
	// is missing, polling fails with a typed failover error, or the deadline wins.
	service.MarkResponseCommitted(c)
	if result == nil || strings.TrimSpace(result.ResponseID) == "" {
		return nil, errors.New("model probe video task accepted without an identifier")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	return h.pollModelProbeVideo(ctx, c, a, result.ResponseID)
}

// Polling observes the already accepted task on this account; it never retries
// creation or returns success merely because the create request was accepted.
func (h *GatewayHandler) pollModelProbeVideo(ctx context.Context, c *gin.Context, a *service.Account, taskID string) (*modelProbeForwardResult, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		capture := &modelProbeStatusCapture{ResponseWriter: c.Writer}
		c.Writer = capture
		_, err := h.openAIGatewayService.ForwardGrokMedia(ctx, c, a, service.GrokMediaEndpointVideoStatus, taskID, nil, "")
		c.Writer = capture.ResponseWriter
		if err != nil {
			return nil, err
		}
		if capture.overflow || !gjson.ValidBytes(capture.body) {
			return nil, errors.New("model probe video status is invalid or too large")
		}
		switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(capture.body, "status").String())) {
		case "done", "completed", "succeeded", "success":
			return h.finishModelProbeVideo(ctx, c, a, taskID, capture.body)
		case "failed", "error", "canceled", "cancelled", "expired":
			return nil, errors.New("model probe video task did not complete")
		}
		timer := time.NewTimer(3 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (h *GatewayHandler) finishModelProbeVideo(ctx context.Context, c *gin.Context, a *service.Account, taskID string, body []byte) (*modelProbeForwardResult, error) {
	for _, field := range []string{"video.url", "url", "video_url", "download_url"} {
		value := strings.TrimSpace(gjson.GetBytes(body, field).String())
		// ForwardGrokMedia replaces an upstream URL with the current task's
		// authenticated content path before writing the status response.
		contentPath := "/videos/" + url.PathEscape(strings.Trim(taskID, "/")) + "/content"
		if value == "/v1"+contentPath || value == contentPath || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "data:") {
			return &modelProbeForwardResult{MediaSuccess: true}, nil
		}
	}
	// Some compatible providers expose completed videos only via /content.
	counter := &modelProbeContentCounter{ResponseWriter: c.Writer}
	c.Writer = counter
	_, err := h.openAIGatewayService.ForwardGrokMedia(ctx, c, a, service.GrokMediaEndpointVideoContent, taskID, nil, "")
	c.Writer = counter.ResponseWriter
	if err != nil {
		return nil, err
	}
	contentType := strings.ToLower(c.Writer.Header().Get("Content-Type"))
	mediaType := strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "application/octet-stream")
	return &modelProbeForwardResult{MediaSuccess: counter.written > 0 && counter.status >= 200 && counter.status < 300 && mediaType}, nil
}

// Gin's original writer has already committed the create status. Observe the
// content response's own status and byte count instead, without buffering it.
type modelProbeContentCounter struct {
	gin.ResponseWriter
	status  int
	written int64
}

func (w *modelProbeContentCounter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *modelProbeContentCounter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.written += int64(n)
	return n, err
}

func (w *modelProbeContentCounter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.written += int64(n)
	return n, err
}

// Only status JSON is observed here. Image/video contents remain in the parent's
// bounded sink; this observer never buffers more than one 64 KiB status response.
type modelProbeStatusCapture struct {
	gin.ResponseWriter
	body     []byte
	overflow bool
}

func (w *modelProbeStatusCapture) Write(p []byte) (int, error) {
	const limit = 64 << 10
	remaining := limit - len(w.body)
	if len(p) > remaining {
		w.overflow = true
	}
	w.body = append(w.body, p[:min(len(p), remaining)]...)
	return w.ResponseWriter.Write(p)
}

func (w *modelProbeStatusCapture) WriteString(s string) (int, error) {
	const limit = 64 << 10
	remaining := limit - len(w.body)
	if len(s) > remaining {
		w.overflow = true
	}
	w.body = append(w.body, s[:min(len(s), remaining)]...)
	return w.ResponseWriter.WriteString(s)
}
