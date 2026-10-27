//go:build unit

package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiUsageLedgerCapturesActualRequestIdentityWithoutAccountHeaderSetting(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	account := &Account{ID: 701, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	attempt := BeginGeminiUpstreamAttempt(c, account, "gemini-ledger", true)
	req, err := http.NewRequest(http.MethodPost, "https://ledger.example.invalid/v1beta/models/gemini-ledger:streamGenerateContent", nil)
	require.NoError(t, err)
	req.Header.Set("x-goog-api-key", "test-ledger-key")
	attempt.ObserveRequest(req)
	attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{"X-Oneapi-Request-Id": {"direct-ledger-request"}, "X-Request-Id": {"next-hop-request"}}})
	result := &ForwardResult{Model: "gemini-ledger", Stream: true}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.Equal(t, "direct-ledger-request", result.UsageLedgerRequestID)
	require.Equal(t, GeminiUsageLedgerIdentity("https://ledger.example.invalid", "test-ledger-key"), result.UsageLedgerIdentityHash)
	require.NotEmpty(t, result.UsageLedgerIdentityHash)
	require.Equal(t, "direct-ledger-request", *usageUpstreamRequestIDForResult(account, result))
	recovered := GeminiResultFromLastAttempt(c, account.ID, "gemini-ledger", true, 0)
	require.Equal(t, result.UsageLedgerRequestID, recovered.UsageLedgerRequestID)
	require.Equal(t, result.UsageLedgerIdentityHash, recovered.UsageLedgerIdentityHash)
	BeginGeminiUpstreamAttempt(c, account, "gemini-ledger", true)
	require.Empty(t, GeminiResultFromLastAttempt(c, account.ID, "gemini-ledger", true, 1).UsageLedgerRequestID)
}

func TestGeminiUsageLedgerNeverGuessesOrTruncatesItsLookupID(t *testing.T) {
	for _, header := range []http.Header{
		{"X-Request-Id": {"not-a-direct-newapi-id"}},
		{"X-Oneapi-Request-Id": {strings.Repeat("x", 129)}},
	} {
		c := newGeminiAttemptTestContext(t)
		attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-ledger", true)
		attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: header})
		result := &ForwardResult{Model: "gemini-ledger", Stream: true}
		finishGeminiUpstreamAttempt(attempt, result, nil)
		require.Empty(t, result.UsageLedgerRequestID)
	}
}

func TestGeminiUsageLedgerResponseMustBelongToObservedOriginAndCredential(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		key  string
		want bool
	}{
		{"same request", "https://ledger.example.invalid/v1beta/models/gemini-ledger", "original-key", true},
		{"same origin redirect", "https://ledger.example.invalid/other/inference", "original-key", true},
		{"different origin", "https://other.example.invalid/inference", "original-key", false},
		{"different port", "https://ledger.example.invalid:8443/inference", "original-key", false},
		{"different credential", "https://ledger.example.invalid/inference", "replacement-key", false},
		{"credential stripped", "https://ledger.example.invalid/inference", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newGeminiAttemptTestContext(t)
			attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-ledger", true)
			request, err := http.NewRequest(http.MethodPost, "https://ledger.example.invalid/v1beta/models/gemini-ledger", nil)
			require.NoError(t, err)
			request.Header.Set("x-goog-api-key", "original-key")
			attempt.ObserveRequest(request)
			finalRequest, err := http.NewRequest(http.MethodPost, tc.url, nil)
			require.NoError(t, err)
			finalRequest.Header.Set("x-goog-api-key", tc.key)
			attempt.ObserveResponse(&http.Response{StatusCode: 200, Request: finalRequest, Header: http.Header{"X-Oneapi-Request-Id": {"final-response-id"}}})
			result := &ForwardResult{Model: "gemini-ledger"}
			finishGeminiUpstreamAttempt(attempt, result, nil)
			if tc.want {
				require.Equal(t, "final-response-id", result.UsageLedgerRequestID)
				require.Equal(t, GeminiUsageLedgerIdentity(tc.url, tc.key), result.UsageLedgerIdentityHash)
			} else {
				require.Empty(t, result.UsageLedgerRequestID)
				require.Empty(t, result.UsageLedgerIdentityHash)
			}
		})
	}
}

func TestGeminiUsageLedgerPersistencePrefersDirectLedgerIDOverOtherHeader(t *testing.T) {
	account := &Account{Platform: PlatformGemini, Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "X-Upstream-Request-Id"}}
	result := &ForwardResult{
		RequireMeasuredUsage: true,
		UsageLedgerRequestID: "direct-ledger-id",
		UpstreamHeaders:      http.Header{"X-Upstream-Request-Id": {"next-hop-id"}},
	}
	require.Equal(t, "direct-ledger-id", *usageUpstreamRequestIDForResult(account, result))
	result.UsageLedgerRequestID = ""
	require.Equal(t, "next-hop-id", *usageUpstreamRequestIDForResult(account, result), "existing configured-header behavior remains when no ledger identity is available")
}

func TestGeminiUsageLedgerDoesNotTreatMalformedAuthorizationAsAnAPIKey(t *testing.T) {
	for _, authorization := range []string{"Basic encoded-credential", "Bearer", "Bearer ", "Bearer key extra"} {
		c := newGeminiAttemptTestContext(t)
		attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-ledger", true)
		request, err := http.NewRequest(http.MethodPost, "https://ledger.example.invalid/inference", nil)
		require.NoError(t, err)
		request.Header.Set("Authorization", authorization)
		attempt.ObserveRequest(request)
		require.Empty(t, attempt.ledgerIdentityHash, authorization)
	}
}
