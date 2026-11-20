package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectInputTokenProtectionJSON(t *testing.T) {
	raw := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":23,"input_tokens_details":{"cached_tokens":8},"output_tokens":5,"total_tokens":28}}}`)
	original := string(raw)
	for _, tc := range []struct {
		name   string
		policy *InputTokenProtection
		want   string
	}{
		{"per-request", nil, original},
		{"token", &InputTokenProtection{Limit: 7}, `{"type":"response.completed","response":{"usage":{"input_tokens":7,"input_tokens_details":{"cached_tokens":7},"output_tokens":5,"total_tokens":12}}}`},
		{"pending", &InputTokenProtection{Pending: true, Limit: 23}, `{"type":"response.completed","response":{"usage":{"input_tokens":0,"input_tokens_details":{"cached_tokens":0},"output_tokens":5,"total_tokens":5}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.JSONEq(t, tc.want, string(ProjectInputTokenProtectionJSON(raw, tc.policy)))
			require.Equal(t, original, string(raw))
		})
	}
}
