//go:build unit

package apicompat

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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
		{name: "explicit zero survives", client: outputLimitTestPtr(0), fallback: 64000},
		{name: "explicit negative left for upstream validation", client: outputLimitTestPtr(-1), fallback: 64000, want: -1},
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

func TestResponsesToAnthropicLegacyConversionUsesKnownModelCapability(t *testing.T) {
	for _, tc := range []struct {
		model  string
		client *int
		want   int
	}{
		{model: "claude-opus-4-6", want: 128000},
		{model: "a/anti-claude-opus-4.6-thinking-cu[次cursor]", want: 128000},
		{model: "claude-sonnet-4-5-thinking", want: 64000},
		{model: "private-compatible-model"},
		{model: "claude-opus-4-6-private"},
		{model: "gemini-4-argon"},
		{model: "private-compatible-model", client: outputLimitTestPtr(0)},
		{model: "private-compatible-model", client: outputLimitTestPtr(-1), want: -1},
		{model: "claude-opus-4-6", client: outputLimitTestPtr(262144), want: 262144},
		{model: "private-compatible-model", client: outputLimitTestPtr(8192), want: 8192},
	} {
		t.Run(tc.model+"/client="+fmt.Sprint(tc.client), func(t *testing.T) {
			req := &ResponsesRequest{Model: tc.model, Input: json.RawMessage(`"hello"`), MaxOutputTokens: tc.client}
			got, err := ResponsesToAnthropicRequest(req)
			require.NoError(t, err)
			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			limit := gjson.GetBytes(encoded, "max_tokens")
			require.Equal(t, tc.want != 0 || tc.client != nil, limit.Exists())
			require.Equal(t, int64(tc.want), limit.Int())
			require.Equal(t, tc.model, got.Model)
			require.Same(t, tc.client, req.MaxOutputTokens)
		})
	}
}

func TestResponsesToAnthropicOptionalDefaultOmitsAbsentLimit(t *testing.T) {
	req := &ResponsesRequest{Model: "gemini-4-argon", Input: json.RawMessage(`"hello"`)}
	got, err := ResponsesToAnthropicRequestWithDefaultMaxTokens(req, 0)
	require.NoError(t, err)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(encoded, "max_tokens").Exists(), "do not send max_tokens:0 when there is no known default")
}
