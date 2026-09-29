package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

const (
	ImageProtocolOpenAI  = "openai"
	ImageProtocolNovelAI = "novelai"

	novelAIImageMaxResponseBytes = 50 << 20
)

// NovelAIForwardResult contains the native response and the accounting data
// needed by the shared image lifecycle.
type NovelAIForwardResult struct {
	Body            []byte
	ResponseHeaders http.Header
	RequestID       string
	Model           string
	UpstreamModel   string
	ImageCount      int
	ImageSize       string
	Duration        time.Duration
}

// GetImageProtocol selects the upstream image protocol for an API-key account.
// Existing accounts intentionally default to OpenAI for backwards compatibility.
func (a *Account) GetImageProtocol() string {
	if a == nil || a.Credentials == nil {
		return ImageProtocolOpenAI
	}
	protocol, _ := a.Credentials["image_protocol"].(string)
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case ImageProtocolNovelAI:
		return ImageProtocolNovelAI
	default:
		return ImageProtocolOpenAI
	}
}

func buildNovelAIImageURL(base string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("NovelAI base URL is empty")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid NovelAI base URL")
	}
	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/ai/generate-image", "/ai", "/v1"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + "/ai/generate-image"
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (s *OpenAIGatewayService) buildNovelAIImagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) (*http.Request, error) {
	if account == nil {
		return nil, fmt.Errorf("NovelAI account is required")
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		return nil, fmt.Errorf("NovelAI account requires an explicit base_url")
	}
	validated, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL, err := buildNovelAIImageURL(validated)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	authHeaders, err := s.buildOpenAIAuthenticationHeaders(ctx, account, token)
	if err != nil {
		return nil, fmt.Errorf("build NovelAI authentication headers: %w", err)
	}
	for key, values := range authHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if !openaiPassthroughAllowedHeaders[strings.ToLower(key)] {
				continue
			}
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	req.Header.Set("Content-Type", "application/json")
	if userAgent := account.GetOpenAIUserAgent(); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	account.ApplyHeaderOverrides(req.Header)
	return req, nil
}

// ForwardNovelAI sends an already-authenticated native request to the selected
// NovelAI account. Scheduling and failover remain in the handler layer.
func (s *OpenAIGatewayService) ForwardNovelAI(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	request *NovelAIImageRequest,
	channelMappedModel string,
) (*NovelAIForwardResult, error) {
	if request == nil {
		return nil, fmt.Errorf("NovelAI request is required")
	}
	if account == nil || account.GetImageProtocol() != ImageProtocolNovelAI {
		return nil, fmt.Errorf("account is not configured for NovelAI protocol")
	}
	upstreamModel := strings.TrimSpace(channelMappedModel)
	if upstreamModel == "" {
		upstreamModel = account.GetMappedModel(request.Model)
	}
	if upstreamModel == "" {
		upstreamModel = request.Model
	}
	body := append([]byte(nil), request.RawBody...)
	if upstreamModel != request.Model {
		var err error
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, fmt.Errorf("rewrite NovelAI request model: %w", err)
		}
	}
	start := time.Now()
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	token, _, err := s.GetAccessToken(upstreamCtx, account)
	if err != nil {
		return nil, err
	}
	upstreamReq, err := s.buildNovelAIImagesRequest(upstreamCtx, c, account, body, token)
	if err != nil {
		return nil, err
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIUpstream(upstreamReq, proxyURL, account)
	if err != nil {
		return nil, fmt.Errorf("NovelAI upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, novelAIImageMaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read NovelAI upstream response: %w", err)
	}
	if len(responseBody) > novelAIImageMaxResponseBytes {
		return nil, fmt.Errorf("NovelAI upstream response is too large")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, &OpenAIImagesUpstreamError{
			StatusCode:        resp.StatusCode,
			Message:           sanitizeUpstreamErrorMessage(message),
			UpstreamRequestID: resp.Header.Get("x-request-id"),
		}
	}
	return &NovelAIForwardResult{
		Body:            responseBody,
		ResponseHeaders: resp.Header.Clone(),
		RequestID:       resp.Header.Get("x-request-id"),
		Model:           request.Model,
		UpstreamModel:   upstreamModel,
		ImageCount:      request.Samples,
		ImageSize:       request.Size(),
		Duration:        time.Since(start),
	}, nil
}
