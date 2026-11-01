package claude

import "testing"

func TestModelMaxOutputTokensRecognizedIdentitiesOnly(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  int
	}{
		{"claude-opus-4-6", 128000},
		{"eu.anthropic.claude-opus-4-6-v1", 128000},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", 64000},
		{"a/anti-claude-opus-4.6-thinking-cu[次cursor]", 128000},
		{"claude-opus-4-6@20260205", 128000},
		{"claude-sonnet-4-5-latest", 0},
		{"claude-opus-4-6-private", 0},
		{"claude-opus-9-9", 0},
		{"private-model", 0},
	} {
		t.Run(tc.model, func(t *testing.T) {
			got, known := ModelMaxOutputTokens(tc.model)
			if got != tc.want || known != (tc.want != 0) {
				t.Fatalf("got (%d, %v), want (%d, %v)", got, known, tc.want, tc.want != 0)
			}
		})
	}
}
