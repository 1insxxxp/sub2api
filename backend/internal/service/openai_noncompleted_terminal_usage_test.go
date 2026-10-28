package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func openAINonCompletedUsageSSE(status, terminalUsage string, progressive bool) string {
	var body strings.Builder
	if progressive {
		_, _ = body.WriteString("event: response.in_progress\n")
		_, _ = body.WriteString(`data: {"type":"response.in_progress","response":{"id":"resp_7978","status":"in_progress","usage":{"input_tokens":500,"output_tokens":25,"total_tokens":525}}}` + "\n\n")
	}
	fmt.Fprintf(&body, "event: response.%s\ndata: {\"type\":\"response.%s\",\"response\":{\"id\":\"resp_7978\",\"object\":\"response\",\"status\":\"%s\",\"output\":[]%s}}\n\n", status, status, status, terminalUsage)
	return body.String()
}

func TestOpenAINonCompletedTerminalUsagePreservesProgressiveMeasurement(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, status := range []string{"incomplete", "cancelled"} {
			for _, tc := range []struct {
				name  string
				usage string
			}{
				{name: "missing"},
				{name: "empty", usage: `,"usage":{}`},
				{name: "all zero", usage: `,"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}`},
			} {
				t.Run(fmt.Sprintf("passthrough=%t/%s/%s", passthrough, status, tc.name), func(t *testing.T) {
					rec, _, result := runOpenAIOAuthNonStreamRegression(t, passthrough,
						`{"model":"gpt-5.4","instructions":"x","input":"hi","stream":false}`,
						openAINonCompletedUsageSSE(status, tc.usage, true))

					require.False(t, result.Stream)
					require.Equal(t, status, gjson.GetBytes(rec.Body.Bytes(), "status").String())
					require.Equal(t, 500, result.Usage.InputTokens)
					require.Equal(t, 25, result.Usage.OutputTokens)
				})
			}
		}
	}
}

func TestOpenAINonCompletedTerminalUsageRetainsTerminalAuthority(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, status := range []string{"incomplete", "cancelled"} {
			for _, tc := range []struct {
				name        string
				progressive bool
				usage       string
				wantInput   int
				wantOutput  int
			}{
				{name: "explicit zero without prior measurement", usage: `,"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}`},
				{name: "lower measured terminal replaces prior values", progressive: true, usage: `,"usage":{"input_tokens":400,"output_tokens":15,"total_tokens":415}`, wantInput: 400, wantOutput: 15},
			} {
				t.Run(fmt.Sprintf("passthrough=%t/%s/%s", passthrough, status, tc.name), func(t *testing.T) {
					_, _, result := runOpenAIOAuthNonStreamRegression(t, passthrough,
						`{"model":"gpt-5.4","instructions":"x","input":"hi","stream":false}`,
						openAINonCompletedUsageSSE(status, tc.usage, tc.progressive))

					require.Equal(t, tc.wantInput, result.Usage.InputTokens)
					require.Equal(t, tc.wantOutput, result.Usage.OutputTokens)
				})
			}
		}
	}
}
