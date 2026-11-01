package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminitokens"
)

const (
	GeminiInputCountingExtraKey                = "gemini_input_counting"
	GeminiInputCountingVertexLikeDiscountV1    = "vertex_like_discount_v1"
	geminiInputCountingWrapperTokens           = 31
	geminiInputCountingAdditionalContentTokens = 2
)

// GeminiInputCountingEvidence is an estimate of the final request. It deliberately
// retains no request, prompt, URL, credentials or provider response content.
type GeminiInputCountingEvidence struct {
	AccountID            int64
	Policy               string
	CandidateInputTokens int
}

// GeminiInputCountingConfigProjection validates and copies only the supported
// scalar configuration for scheduler snapshots. Unknown settings stay disabled.
func GeminiInputCountingConfigProjection(value any) map[string]any {
	fields, ok := value.(map[string]any)
	if !ok || fields["mode"] != GeminiInputCountingVertexLikeDiscountV1 {
		return nil
	}
	var models []string
	switch values := fields["models"].(type) {
	case []string:
		models = append([]string(nil), values...)
	case []any:
		for _, item := range values {
			model, ok := item.(string)
			if !ok {
				return nil
			}
			models = append(models, model)
		}
	default:
		return nil
	}
	if len(models) == 0 || len(models) > 32 {
		return nil
	}
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		if !utf8.ValidString(model) || strings.TrimSpace(model) != model || !isSafeGeminiAIStudioModelPathSegment(model) || seen[model] {
			return nil
		}
		seen[model] = true
	}
	return map[string]any{"mode": GeminiInputCountingVertexLikeDiscountV1, "models": models}
}

func accountGeminiInputCountingEnabled(account *Account, model string) bool {
	if account == nil || account.ID <= 0 || account.Platform != PlatformGemini || account.Type != AccountTypeAPIKey {
		return false
	}
	policy := GeminiInputCountingConfigProjection(account.Extra[GeminiInputCountingExtraKey])
	if policy == nil {
		return false
	}
	models, ok := policy["models"].([]string)
	if !ok {
		return false
	}
	for _, configured := range models {
		if configured == model {
			return true
		}
	}
	return false
}

func prepareGeminiInputCountingPolicy(req *http.Request, account *Account, model string) *GeminiInputCountingEvidence {
	if !accountGeminiInputCountingEnabled(account, model) || req == nil || req.Method != http.MethodPost || req.URL == nil || req.GetBody == nil {
		return nil
	}
	if !strings.HasSuffix(req.URL.Path, "/models/"+model+":generateContent") && !strings.HasSuffix(req.URL.Path, "/models/"+model+":streamGenerateContent") {
		return nil
	}
	body, err := req.GetBody()
	if err != nil || body == nil {
		return nil
	}
	defer func() {
		// This read-only replay body has no writes to flush; closing it cannot
		// change the request evidence already read below.
		_ = body.Close()
	}()
	raw, err := io.ReadAll(io.LimitReader(body, geminitokens.MaxTextBytes+1))
	if err != nil || len(raw) > geminitokens.MaxTextBytes || !utf8.Valid(raw) {
		return nil
	}
	fields, err := geminiLedgerObject(raw)
	if err != nil {
		return nil
	}
	for key := range fields {
		switch key {
		case "contents", "systemInstruction", "generationConfig", "safetySettings":
		default:
			return nil
		}
	}
	if generation, exists := fields["generationConfig"]; exists && !geminiInputCountingTextGeneration(generation) {
		return nil
	}
	var contents []json.RawMessage
	if json.Unmarshal(fields["contents"], &contents) != nil || len(contents) == 0 {
		return nil
	}
	texts := make([]string, 0, len(contents)+1)
	if system, exists := fields["systemInstruction"]; exists {
		parts, ok := geminiInputCountingTextContent(system, true)
		if !ok {
			return nil
		}
		texts = append(texts, parts...)
	}
	for _, content := range contents {
		parts, ok := geminiInputCountingTextContent(content, false)
		if !ok {
			return nil
		}
		texts = append(texts, parts...)
	}
	tokens, err := geminitokens.CountTexts(texts)
	if err != nil {
		return nil
	}
	return &GeminiInputCountingEvidence{AccountID: account.ID, Policy: GeminiInputCountingVertexLikeDiscountV1,
		CandidateInputTokens: tokens + geminiInputCountingWrapperTokens + geminiInputCountingAdditionalContentTokens*(len(contents)-1)}
}

func geminiInputCountingTextContent(raw json.RawMessage, system bool) ([]string, bool) {
	content, err := geminiLedgerObject(raw)
	if err != nil {
		return nil, false
	}
	for key := range content {
		if key != "role" && key != "parts" {
			return nil, false
		}
	}
	if value, exists := content["role"]; exists {
		var role string
		if json.Unmarshal(value, &role) != nil || (system && role != "" && role != "system" && role != "user") || (!system && role != "" && role != "user" && role != "model") {
			return nil, false
		}
	}
	var parts []json.RawMessage
	if json.Unmarshal(content["parts"], &parts) != nil || len(parts) == 0 {
		return nil, false
	}
	texts := make([]string, 0, len(parts))
	for _, rawPart := range parts {
		part, err := geminiLedgerObject(rawPart)
		if err != nil || len(part) != 1 {
			return nil, false
		}
		var text string
		if rawText, exists := part["text"]; !exists || string(rawText) == "null" || json.Unmarshal(rawText, &text) != nil {
			return nil, false
		}
		texts = append(texts, text)
	}
	return texts, true
}

func geminiInputCountingTextGeneration(raw json.RawMessage) bool {
	fields, err := geminiLedgerObject(raw)
	if err != nil {
		return false
	}
	for key, value := range fields {
		switch key {
		case "temperature", "topP", "topK", "maxOutputTokens", "candidateCount", "seed", "presencePenalty", "frequencyPenalty", "stopSequences", "thinkingConfig":
		case "responseMimeType":
			var mime string
			if json.Unmarshal(value, &mime) != nil || mime != "text/plain" {
				return false
			}
		case "responseModalities":
			var modalities []string
			if json.Unmarshal(value, &modalities) != nil {
				return false
			}
			for _, modality := range modalities {
				if modality != "TEXT" {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// geminiCustomerInputTokens projects customer input without mutating the direct
// upstream measurement. Missing/unknown fields and all cache paths fail closed.
func geminiCustomerInputTokens(result *ForwardResult) (int, bool) {
	if result == nil {
		return 0, false
	}
	raw := result.Usage.InputTokens
	evidence := result.GeminiInputCounting
	if evidence == nil || evidence.AccountID <= 0 || evidence.Policy != GeminiInputCountingVertexLikeDiscountV1 ||
		evidence.CandidateInputTokens <= 0 || evidence.CandidateInputTokens >= raw || raw <= 0 ||
		result.ImageEndpoint || result.ImageCount != 0 || result.Usage.ImageOutputTokens != 0 || result.AudioUsage != nil || (result.Outcome != nil && result.Outcome.HasMedia) ||
		result.InputTokensSource != UsageTokenSourceUpstream || result.GeminiUsageFallback == nil || result.GeminiUsageFallback.UsageFields&8 == 0 ||
		result.Usage.CacheReadInputTokens != 0 || result.Usage.CacheCreationInputTokens != 0 || result.Usage.CacheCreation5mTokens != 0 || result.Usage.CacheCreation1hTokens != 0 {
		return raw, false
	}
	return evidence.CandidateInputTokens, true
}
