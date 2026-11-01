//go:build unit

package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicOutputLimitUsesProtocolDefault(t *testing.T) {
	for _, tc := range []struct {
		name     string
		client   *int
		fallback int
		want     int
	}{
		{name: "optional intermediate", fallback: 0},
		{name: "required upstream", fallback: 128000, want: 128000},
		{name: "explicit small", client: outputLimitTestPtr(32), fallback: 128000, want: 32},
		{name: "explicit 8192", client: outputLimitTestPtr(8192), fallback: 128000, want: 8192},
		{name: "explicit large", client: outputLimitTestPtr(128000), fallback: 64000, want: 128000},
		{name: "explicit intermediate", client: outputLimitTestPtr(8192), want: 8192},
		{name: "existing zero behavior", client: outputLimitTestPtr(0), fallback: 64000, want: 64000},
		{name: "existing negative behavior", client: outputLimitTestPtr(-1), fallback: 64000, want: 64000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &ResponsesRequest{Model: "gemini-4-argon", Input: json.RawMessage(`"hello"`), MaxOutputTokens: tc.client}
			got, err := ResponsesToAnthropicRequestWithDefaultMaxTokens(req, tc.fallback)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.MaxTokens)
			require.Same(t, tc.client, req.MaxOutputTokens, "conversion must not overwrite client intent")
		})
	}
}

func outputLimitTestPtr(value int) *int { return &value }
