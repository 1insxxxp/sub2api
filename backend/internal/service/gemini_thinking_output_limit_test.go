//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiCompatThinkingBudgetFitsFinalOutputLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, model := range []struct {
		name             string
		minimum, maximum int64
	}{
		{"gemini-2.5-pro", 128, 32768},
		{"gemini-2.5-flash", 1, 24576},
		{"gemini-2.5-flash-lite", 512, 24576},
	} {
		for _, protocol := range []string{"chat", "messages"} {
			for _, limit := range []int64{-2, -1, 0, 8, 128, 129, 512, 513, 4096, 128000} {
				t.Run(fmt.Sprintf("%s/%s/%d", model.name, protocol, limit), func(t *testing.T) {
					payload := map[string]any{"model": model.name, "messages": []map[string]string{{"role": "user", "content": "hello"}}}
					field := "max_tokens"
					if protocol == "chat" {
						field = "max_completion_tokens"
						payload["reasoning_effort"] = "high"
					} else {
						payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": 32000}
						payload["output_config"] = map[string]string{"effort": "high"}
					}
					if limit != -2 {
						payload[field] = limit
					}
					body, err := json.Marshal(payload)
					require.NoError(t, err)
					upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}`)),
					}}}
					svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
					c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/"+protocol, body)
					if protocol == "chat" {
						_, err = svc.ForwardAsChatCompletions(context.Background(), c, geminiCompatThinkingTestAccount(), body)
					} else {
						_, err = svc.Forward(context.Background(), c, geminiCompatThinkingTestAccount(), body)
					}
					require.NoError(t, err)
					require.Len(t, upstream.requestBodies, 1)
					wire := upstream.requestBodies[0]
					cap := gjson.GetBytes(wire, "generationConfig.maxOutputTokens")
					require.Equal(t, limit != -2, cap.Exists())
					if limit != -2 {
						require.Equal(t, limit, cap.Int())
					}
					budget := gjson.GetBytes(wire, "generationConfig.thinkingConfig.thinkingBudget")
					if limit != -2 && limit <= model.minimum {
						require.False(t, budget.Exists(), "leave incompatible optional thinking to native protocol validation")
					} else {
						require.True(t, budget.Exists())
						want := model.maximum
						if limit != -2 {
							want = min(want, limit-1)
						}
						require.Equal(t, want, budget.Int())
					}
				})
			}
		}
	}
}

func TestGeminiCompatSmallOutputLimitPreservesThinkingDisabled(t *testing.T) {
	for _, model := range []string{"gemini-2.5-flash", "gemini-2.5-flash-lite"} {
		t.Run(model, func(t *testing.T) {
			got := applyGeminiCompatThinkingConfig([]byte(`{"generationConfig":{"maxOutputTokens":8}}`), []byte(`{"thinking":{"type":"disabled"}}`), model, true)
			budget := gjson.GetBytes(got, "generationConfig.thinkingConfig.thinkingBudget")
			require.True(t, budget.Exists())
			require.Zero(t, budget.Int())
			require.Equal(t, int64(8), gjson.GetBytes(got, "generationConfig.maxOutputTokens").Int())
			require.False(t, gjson.GetBytes(got, "generationConfig.thinkingConfig.includeThoughts").Exists())
		})
	}
}
