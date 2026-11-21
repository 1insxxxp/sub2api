package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectInputTokenProtectionJSON(t *testing.T) {
	raw := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":5023,"input_tokens_details":{"cached_tokens":8},"output_tokens":5,"total_tokens":5028}}}`)
	original := string(raw)
	for _, tc := range []struct {
		name   string
		policy *InputTokenProtection
		want   string
	}{
		{"per-request", nil, original},
		{"token", &InputTokenProtection{Limit: 7}, `{"type":"response.completed","response":{"usage":{"input_tokens":7,"input_tokens_details":{"cached_tokens":0},"output_tokens":5,"total_tokens":12}}}`},
		{"missing estimate", &InputTokenProtection{Pending: true, Limit: 23}, original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.JSONEq(t, tc.want, string(ProjectInputTokenProtectionJSON(raw, tc.policy)))
			require.Equal(t, original, string(raw))
		})
	}
}

func TestProjectInputTokenProtectionZeroEstimate(t *testing.T) {
	p := &InputTokenProtection{Limit: 0}
	missing := []byte(`{"usageMetadata":{"cachedContentTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`)
	require.Equal(t, missing, ProjectInputTokenProtectionJSON(missing, p))
	measured := []byte(`{"usageMetadata":{"promptTokenCount":3,"cachedContentTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":5}}`)
	require.JSONEq(t, `{"usageMetadata":{"promptTokenCount":0,"cachedContentTokenCount":0,"candidatesTokenCount":2,"totalTokenCount":2}}`, string(ProjectInputTokenProtectionJSON(measured, p)))
}
