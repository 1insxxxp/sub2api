package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func ledgerFixture(row map[string]any) []byte {
	raw, _ := json.Marshal(map[string]any{"success": true, "data": []any{row}})
	return raw
}

func ledgerFixtureRow() map[string]any {
	return map[string]any{
		"id": 447, "type": 2, "request_id": "provider-request-1", "upstream_request_id": "next-hop-request",
		"model_name": "gemini-test", "prompt_tokens": 21916, "completion_tokens": 4055,
		"other": `{"cache_tokens":0,"cache_input_includes_cache":true,"stream_status":{"status":"error","end_reason":"client_gone"}}`,
	}
}

func TestGeminiUsageLedgerMatchesVerifiedProviderShape(t *testing.T) {
	usage, err := lookupGeminiLedgerUsage(ledgerFixture(ledgerFixtureRow()), "provider-request-1", "gemini-test")
	require.NoError(t, err)
	require.Equal(t, &ClaudeUsage{InputTokens: 21916, OutputTokens: 4055}, usage)
	for _, other := range []string{`{"cache_tokens":700,"cache_input_includes_cache":true}`, `{"cache_tokens":700,"cache_input_includes_cache":false}`} {
		row := ledgerFixtureRow()
		row["other"] = other
		usage, err = lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
		require.NoError(t, err)
		require.Equal(t, 700, usage.CacheReadInputTokens)
		if strings.Contains(other, "true") {
			require.Equal(t, 21216, usage.InputTokens)
		} else {
			require.Equal(t, 21916, usage.InputTokens)
		}
		require.Equal(t, 4055, usage.OutputTokens, "ledger output already includes provider reasoning; do not add it again")
	}
}

func TestGeminiUsageLedgerUsesOnlyDirectConsumptionRequestIdentity(t *testing.T) {
	for _, id := range []string{"missing-request", "next-hop-request", "447"} {
		usage, err := lookupGeminiLedgerUsage(ledgerFixture(ledgerFixtureRow()), id, "gemini-test")
		require.NoError(t, err)
		require.Nil(t, usage)
	}
	for _, typ := range []int{1, 3, 4, 5, 6} {
		row := ledgerFixtureRow()
		row["type"] = typ
		usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
		require.NoError(t, err)
		require.Nil(t, usage)
	}
	row := ledgerFixtureRow()
	row["model_name"] = "different-model"
	usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
	require.Nil(t, usage)
	require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse)
}

func TestGeminiUsageLedgerRejectsUnreportedOrMalformedCounts(t *testing.T) {
	for _, field := range []string{"prompt_tokens", "completion_tokens"} {
		for _, bad := range []any{nil, "10", -1, 1.5, true, int64(2147483648)} {
			t.Run(fmt.Sprintf("%s=%v", field, bad), func(t *testing.T) {
				row := ledgerFixtureRow()
				row[field] = bad
				usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
				require.Nil(t, usage)
				require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse)
			})
		}
		row := ledgerFixtureRow()
		delete(row, field)
		usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
		require.Nil(t, usage)
		require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse)
	}
	row := ledgerFixtureRow()
	row["prompt_tokens"], row["completion_tokens"] = 0, 0
	usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
	require.NoError(t, err)
	require.Equal(t, &ClaudeUsage{}, usage, "explicit upstream zero is a reported measurement")
}

func TestGeminiUsageLedgerRejectsAmbiguousAndInconsistentCache(t *testing.T) {
	row := ledgerFixtureRow()
	raw, _ := json.Marshal(map[string]any{"success": true, "data": []any{row, row}})
	usage, err := lookupGeminiLedgerUsage(raw, "provider-request-1", "gemini-test")
	require.Nil(t, usage)
	require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse)
	for _, other := range []any{
		`{"cache_tokens":1}`, `{"cache_tokens":1,"cache_input_includes_cache":"true"}`,
		`{"cache_tokens":1,"cache_input_includes_cache":null}`, `{"cache_tokens":21917,"cache_input_includes_cache":true}`,
		`{"cache_tokens":-1}`, `{"cache_tokens":null}`, `{"cache_tokens":"0"}`, `{"cache_tokens":0.5}`,
		`{"cache_tokens":2147483648}`, `{broken}`, `[]`, 10,
	} {
		row := ledgerFixtureRow()
		row["other"] = other
		usage, err := lookupGeminiLedgerUsage(ledgerFixture(row), "provider-request-1", "gemini-test")
		require.Nil(t, usage, "%v", other)
		require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse, "%v", other)
	}
}

func TestGeminiUsageLedgerRejectsEnvelopeAndDuplicateFieldsWithoutLeakingPayload(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"success":false,"data":[]}`, `{"success":true,"data":null}`, `{"success":true,"data":{}}`,
		`{"success":true,"success":false,"data":[]}`,
		`{"success":true,"data":[{"type":2,"request_id":"provider-request-1","model_name":"gemini-test","prompt_tokens":100,"prompt_tokens":1,"completion_tokens":2}]}`,
		`{"success":true,"data":[]} trailing-secret`,
	} {
		usage, err := lookupGeminiLedgerUsage([]byte(raw), "provider-request-1", "gemini-test")
		require.Nil(t, usage)
		require.ErrorIs(t, err, ErrGeminiLedgerInvalidResponse)
		require.NotContains(t, err.Error(), raw)
		require.NotContains(t, err.Error(), "provider-request-1")
		require.NotContains(t, err.Error(), "trailing-secret")
	}
	usage, err := lookupGeminiLedgerUsage([]byte(`{"success":true,"data":[]}`), "provider-request-1", "gemini-test")
	require.NoError(t, err)
	require.Nil(t, usage)
}
