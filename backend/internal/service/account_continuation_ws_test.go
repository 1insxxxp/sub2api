//go:build unit

package service

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountContinuationWebSocketSinkDoesNotCapCurrentEvent(t *testing.T) {
	text := strings.Repeat("x", accountContinuationCaptureLimit+1)
	event := []byte(`{"type":"response.output_text.delta","delta":"` + text + `"}`)
	var delivered []byte
	sink := &accountContinuationWSSink{header: make(http.Header), status: http.StatusOK, size: -1, send: func(payload []byte) error {
		delivered = append([]byte(nil), payload...)
		return nil
	}}
	frame := append(append([]byte("data: "), event...), '\n', '\n')
	n, err := sink.Write(frame)
	require.NoError(t, err, "capture safety must not limit a valid current websocket event")
	require.Equal(t, len(frame), n)
	require.Equal(t, event, delivered)
}

func TestAccountContinuationWebSocketJoinsOneLogicalTurn(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil)
	c.Set("api_key", &APIKey{ID: 9, UserID: 11})
	account := &Account{ID: 1, Extra: map[string]any{"anti_truncation": map[string]any{"version": 1, "rules": []any{map[string]any{"model": "gpt-text", "enabled": true, "max_rounds": 1}}}}}
	svc := &OpenAIGatewayService{}
	var frames [][]byte
	write := func(frame []byte) error { frames = append(frames, append([]byte(nil), frame...)); return nil }
	calls := 0
	result, err := svc.runAccountContinuationWSTurn(context.Background(), c, account, []byte(`{"type":"response.create","model":"gpt-text","input":"write"}`), "gpt-text", &write, func(body []byte) (*OpenAIForwardResult, error) {
		calls++
		if calls == 2 {
			require.Contains(t, string(body), "alpha")
			require.Len(t, gjson.GetBytes(body, "input").Array(), 3)
		}
		status, reason, text := "incomplete", "max_output_tokens", "alpha"
		if calls == 2 {
			status, reason, text = "completed", "", "beta"
		}
		for _, frame := range []string{fmt.Sprintf(`{"type":"response.created","response":{"id":"resp_%d","status":"in_progress"}}`, calls), fmt.Sprintf(`{"type":"response.output_text.delta","output_index":0,"item_id":"m","delta":"%s"}`, text), fmt.Sprintf(`{"type":"response.%s","response":{"id":"resp_%d","status":"%s","incomplete_details":{"reason":"%s"},"output":[{"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":"%s"}]}]}}`, status, calls, status, reason, text)} {
			require.NoError(t, write([]byte(frame)))
		}
		return &OpenAIForwardResult{RequestID: fmt.Sprintf("resp_%d", calls), ResponseID: fmt.Sprintf("resp_%d", calls), Usage: OpenAIUsage{InputTokens: 10, OutputTokens: 2}}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 20, result.Usage.InputTokens)
	joined := stringJoinFrames(frames)
	require.Equal(t, 1, strings.Count(joined, `"type":"response.created"`))
	require.NotContains(t, joined, `"type":"response.incomplete"`)
	require.Equal(t, 1, strings.Count(joined, `"type":"response.completed"`))
	require.NotContains(t, joined, `"id":"resp_2"`)
	actual, err := svc.ResolveOpenAIContinuationAlias(context.Background(), 0, 11, "resp_1")
	require.NoError(t, err)
	require.Equal(t, "resp_2", actual)
}
func stringJoinFrames(frames [][]byte) string {
	var b strings.Builder
	for _, f := range frames {
		b.Write(f)
		b.WriteByte('\n')
	}
	return b.String()
}
