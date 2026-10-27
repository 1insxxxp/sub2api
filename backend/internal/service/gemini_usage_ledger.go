package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

// ErrGeminiLedgerInvalidResponse deliberately excludes provider response contents.
var ErrGeminiLedgerInvalidResponse = errors.New("invalid upstream usage ledger")

// lookupGeminiLedgerUsage accepts only a single, exact direct-provider request
// identity. The display row ID, downstream provider ID and timestamps are not
// stable evidence that a ledger row belongs to this request.
func lookupGeminiLedgerUsage(raw []byte, requestID, expectedModel string) (*ClaudeUsage, error) {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(expectedModel) == "" {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	envelope, err := geminiLedgerObject(raw)
	if err != nil || !bytes.Equal(bytes.TrimSpace(envelope["success"]), []byte("true")) {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	var matched *ClaudeUsage
	for _, rawRow := range rows {
		row, err := geminiLedgerObject(rawRow)
		if err != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		var directID string
		if json.Unmarshal(row["request_id"], &directID) != nil || directID != requestID {
			continue
		}
		kind, err := geminiLedgerTokenCount(row["type"])
		if err != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		if kind != 2 { // New API consumption log
			continue
		}
		var model string
		if json.Unmarshal(row["model_name"], &model) != nil || model != expectedModel || matched != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		prompt, err := geminiLedgerTokenCount(row["prompt_tokens"])
		if err != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		completion, err := geminiLedgerTokenCount(row["completion_tokens"])
		if err != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		cache, includesCache, err := geminiLedgerCache(row["other"])
		if err != nil || (includesCache && cache > prompt) {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		if includesCache {
			prompt -= cache
		}
		// completion_tokens is the provider's final ledger count. Adding thoughts
		// or locally counted output would count them twice.
		matched = &ClaudeUsage{InputTokens: prompt, OutputTokens: completion, CacheReadInputTokens: cache}
	}
	return matched, nil
}

func geminiLedgerCache(raw json.RawMessage) (int, bool, error) {
	if len(raw) == 0 {
		return 0, false, nil
	}
	var other string
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &other) != nil {
		return 0, false, ErrGeminiLedgerInvalidResponse
	}
	if strings.TrimSpace(other) == "" {
		return 0, false, nil
	}
	fields, err := geminiLedgerObject([]byte(other))
	if err != nil {
		return 0, false, ErrGeminiLedgerInvalidResponse
	}
	cacheRaw, exists := fields["cache_tokens"]
	if !exists {
		return 0, false, nil
	}
	cache, err := geminiLedgerTokenCount(cacheRaw)
	if err != nil {
		return 0, false, ErrGeminiLedgerInvalidResponse
	}
	if cache == 0 {
		return 0, false, nil
	}
	includes := bytes.TrimSpace(fields["cache_input_includes_cache"])
	if bytes.Equal(includes, []byte("true")) {
		return cache, true, nil
	}
	if bytes.Equal(includes, []byte("false")) {
		return cache, false, nil
	}
	return 0, false, ErrGeminiLedgerInvalidResponse
}

func geminiLedgerTokenCount(raw json.RawMessage) (int, error) {
	// Counts must fit the persisted PostgreSQL INTEGER columns. ParseUint also
	// rejects null, strings, fractional/exponent numbers and signed negatives.
	value, err := strconv.ParseUint(string(bytes.TrimSpace(raw)), 10, 31)
	if err != nil {
		return 0, ErrGeminiLedgerInvalidResponse
	}
	return int(value), nil
}

// Do not silently accept duplicate financial fields using encoding/json's
// normal last-value-wins behavior.
func geminiLedgerObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		if _, exists := fields[name]; exists {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, ErrGeminiLedgerInvalidResponse
		}
		fields[name] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrGeminiLedgerInvalidResponse
	}
	return fields, nil
}
