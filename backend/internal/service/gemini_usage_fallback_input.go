package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/tiktoken-go/tokenizer"
)

const (
	geminiInputCountMaxRequestBytes  = 4 << 20
	geminiInputCountMaxResponseBytes = 64 << 10
	geminiInputCountTimeout          = 2 * time.Second
	geminiInputContentOverhead       = 3
	geminiInputPartOverhead          = 1
)

var ErrGeminiInputCountUnavailable = errors.New("upstream input token count unavailable")

// GeminiInputUsageFallback holds a scalar local estimate and a short-lived,
// credential-free count request. CountRequest must never be persisted.
type GeminiInputUsageFallback struct {
	Tokens                          int
	Known                           bool
	CalibrationKey                  string
	CountRequest                    *geminiInputCountRequest
	LocalTokensExcludeCachedContent bool
}

type geminiInputCountRequest struct {
	URL          string
	Body         []byte
	IdentityHash string
	AccountID    int64
	AuthHeader   string
}

func prepareGeminiInputUsageFallback(req *http.Request, accountID int64, model string) *GeminiInputUsageFallback {
	fallback := &GeminiInputUsageFallback{}
	if req == nil || req.GetBody == nil {
		return fallback
	}
	body, err := req.GetBody()
	if err != nil || body == nil {
		return fallback
	}
	raw, err := io.ReadAll(io.LimitReader(body, geminiInputCountMaxRequestBytes+1))
	_ = body.Close()
	if err != nil || len(raw) > geminiInputCountMaxRequestBytes {
		return fallback
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(raw, &payload) != nil || payload == nil {
		return fallback
	}
	wrapped := false
	if nested, exists := payload["request"]; exists {
		payload = nil
		if json.Unmarshal(nested, &payload) != nil || payload == nil {
			return fallback
		}
		wrapped = true
	}
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return fallback
	}
	counter := geminiInputSemanticCounter{codec: codec, textOnly: true}
	counter.content(payload["systemInstruction"], false)
	counter.content(payload["system_instruction"], false)
	var contents []json.RawMessage
	if rawContents, exists := payload["contents"]; exists {
		if json.Unmarshal(rawContents, &contents) != nil {
			return fallback
		}
		for _, content := range contents {
			counter.content(content, true)
		}
	}
	if tools := payload["tools"]; len(tools) > 0 && string(tools) != "null" && string(tools) != "[]" {
		counter.tools = true
		counter.semanticJSON(tools)
	}
	var generation map[string]json.RawMessage
	if json.Unmarshal(payload["generationConfig"], &generation) == nil {
		if len(generation["responseSchema"]) > 0 || len(generation["responseJsonSchema"]) > 0 {
			counter.tools = true
		}
		counter.semanticJSON(generation["responseSchema"])
		counter.semanticJSON(generation["responseJsonSchema"])
	}
	if counter.err != nil {
		return fallback
	}
	fallback.Known, fallback.Tokens = true, counter.tokens
	identity := geminiUsageLedgerRequestIdentity(req)
	hasExternalCache := len(payload["cachedContent"]) > 0 || len(payload["cached_content"]) > 0
	fallback.LocalTokensExcludeCachedContent = hasExternalCache
	if counter.textOnly && !hasExternalCache && identity != "" && accountID > 0 {
		shape, language := "plain", "latin"
		if counter.tools {
			shape = "tools"
		}
		if counter.runes > 0 && counter.cjkRunes*5 >= counter.runes {
			language = "cjk"
		}
		// Logarithmic length buckets prevent short-message overhead from
		// calibrating a very long prompt. Only a digest survives the request.
		bucket := 0
		for length := counter.tokens; length >= 128; length /= 4 {
			bucket++
		}
		digest := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s\x00%d", accountID, identity, model, shape, language, bucket)))
		fallback.CalibrationKey = hex.EncodeToString(digest[:])
	}
	if !wrapped && identity != "" {
		fallback.CountRequest = prepareGeminiInputCountRequest(req.URL, payload, identity, accountID, model, hasExternalCache)
		if fallback.CountRequest != nil {
			fallback.CountRequest.AuthHeader = "x-goog-api-key"
			if req.Header.Get("x-goog-api-key") == "" {
				fallback.CountRequest.AuthHeader = "Authorization"
			}
		}
	}
	return fallback
}

func (s *GatewayService) CountGeminiInputFallback(ctx context.Context, input *RecordUsageInput) (int, bool, error) {
	if input == nil || input.Result == nil || input.Result.GeminiUsageFallback == nil || input.Result.GeminiUsageFallback.Input == nil {
		return 0, false, nil
	}
	fallback := input.Result.GeminiUsageFallback
	prepared := fallback.Input.CountRequest
	fallback.Input.CountRequest = nil // Release on success, failure, and every skip.
	account := input.Account
	if prepared == nil || account == nil || account.Platform != PlatformGemini || account.Type != AccountTypeAPIKey ||
		account.ID != prepared.AccountID || fallback.UsageFields&1 != 0 {
		return 0, false, nil
	}
	apiKey := account.GetCredential("api_key")
	baseURL := account.GetGeminiBaseURL(geminicli.AIStudioBaseURL)
	identity := GeminiUsageLedgerIdentity(baseURL, apiKey)
	if identity == "" || identity != prepared.IdentityHash || GeminiUsageLedgerIdentity(prepared.URL, apiKey) != identity {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	if s == nil || s.cfg == nil || s.httpUpstream == nil || len(prepared.Body) > geminiInputCountMaxRequestBytes {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	if _, err := s.validateUpstreamBaseURL(prepared.URL); err != nil {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return 0, false, ErrGeminiInputCountUnavailable
		}
		proxyURL = account.Proxy.URL()
	}
	budget := usageFollowupBudget(ctx, geminiInputCountTimeout)
	if budget <= 0 {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	baseContext := context.Background()
	if ctx != nil {
		baseContext = context.WithoutCancel(ctx)
	}
	requestCtx, cancel := context.WithTimeout(baseContext, budget)
	defer cancel()
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(requestCtx), http.MethodPost, prepared.URL, bytes.NewReader(prepared.Body))
	if err != nil {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	if prepared.AuthHeader == "Authorization" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		req.Header.Set("x-goog-api-key", apiKey)
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return 0, false, ErrGeminiInputCountUnavailable
	}
	if resp == nil || resp.Body == nil {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || (resp.Request != nil && (resp.Request.URL == nil || resp.Request.URL.String() != prepared.URL)) {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, geminiInputCountMaxResponseBytes+1))
	if err != nil || len(raw) > geminiInputCountMaxResponseBytes {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	fields, err := geminiLedgerObject(raw)
	if err != nil {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	tokens, err := geminiLedgerTokenCount(fields["totalTokens"])
	if err != nil {
		return 0, false, ErrGeminiInputCountUnavailable
	}
	return tokens, true, nil
}

func prepareGeminiInputCountRequest(originalURL *url.URL, payload map[string]json.RawMessage, identity string, accountID int64, model string, hasExternalCache bool) *geminiInputCountRequest {
	if originalURL == nil || originalURL.User != nil || accountID <= 0 {
		return nil
	}
	u := *originalURL
	action := ""
	for _, candidate := range []string{":streamGenerateContent", ":generateContent"} {
		if strings.HasSuffix(u.Path, candidate) {
			action = candidate
			break
		}
	}
	if action == "" {
		return nil
	}
	modelIndex := strings.LastIndex(u.Path, "/models/")
	if modelIndex < 0 || strings.TrimSuffix(u.Path[modelIndex+len("/models/"):], action) != strings.TrimPrefix(model, "models/") {
		return nil
	}
	vertex := strings.Contains(u.Path, "/publishers/google/models/")
	if !vertex && !strings.Contains(u.Path, "/v1beta/models/") && !strings.Contains(u.Path, "/v1/models/") {
		return nil
	}
	u.Path = strings.TrimSuffix(u.Path, action) + ":countTokens"
	if u.RawPath != "" {
		u.RawPath = strings.TrimSuffix(u.RawPath, action) + ":countTokens"
	}
	u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = "", "", "", false
	var body []byte
	var err error
	if vertex {
		if hasExternalCache {
			return nil // This endpoint does not document cachedContent counting.
		}
		counted := make(map[string]json.RawMessage)
		for _, key := range []string{"contents", "tools", "systemInstruction", "generationConfig"} {
			if value, ok := payload[key]; ok {
				counted[key] = value
			}
		}
		body, err = json.Marshal(counted)
	} else {
		counted := make(map[string]json.RawMessage, len(payload)+1)
		for key, value := range payload {
			counted[key] = value
		}
		counted["model"], _ = json.Marshal("models/" + strings.TrimPrefix(model, "models/"))
		body, err = json.Marshal(map[string]any{"generateContentRequest": counted})
	}
	if err != nil || len(body) > geminiInputCountMaxRequestBytes {
		return nil
	}
	return &geminiInputCountRequest{URL: u.String(), Body: body, IdentityHash: identity, AccountID: accountID}
}

type geminiInputSemanticCounter struct {
	codec                   tokenizer.Codec
	tokens, runes, cjkRunes int
	textOnly, tools         bool
	err                     error
}

func (c *geminiInputSemanticCounter) text(text string) {
	if text == "" || c.err != nil {
		return
	}
	var count int
	count, c.err = c.codec.Count(text)
	c.tokens += count
	for _, r := range text {
		c.runes++
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			c.cjkRunes++
		}
	}
}

func (c *geminiInputSemanticCounter) semanticJSON(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	// Compact structural JSON rather than encoding the whole transport envelope.
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		c.err = ErrGeminiInputCountUnavailable
		return
	}
	compacted, err := json.Marshal(value)
	if err != nil {
		c.err = err
		return
	}
	c.text(string(compacted))
}

func (c *geminiInputSemanticCounter) content(raw json.RawMessage, hasRole bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		c.text(plain)
		return
	}
	var content struct {
		Parts []map[string]json.RawMessage `json:"parts"`
	}
	if json.Unmarshal(raw, &content) != nil {
		c.err = ErrGeminiInputCountUnavailable
		return
	}
	if hasRole && len(content.Parts) > 0 {
		c.tokens += geminiInputContentOverhead
	}
	for _, part := range content.Parts {
		semantic := false
		var text string
		if value, exists := part["text"]; exists {
			if json.Unmarshal(value, &text) != nil {
				c.err = ErrGeminiInputCountUnavailable
				return
			}
			c.text(text)
			semantic = text != ""
		}
		for _, field := range []string{"functionCall", "functionResponse", "executableCode", "codeExecutionResult"} {
			if value, exists := part[field]; exists {
				c.semanticJSON(value)
				semantic, c.tools = true, true
			}
		}
		for _, field := range []string{"inlineData", "fileData", "inline_data", "file_data", "videoMetadata"} {
			if _, exists := part[field]; exists {
				c.textOnly = false
			}
		}
		// File URLs, base64 data, and opaque thought signatures deliberately do
		// not become ordinary text tokens. Plain text and function JSON still do.
		if semantic {
			c.tokens += geminiInputPartOverhead
		}
	}
}
