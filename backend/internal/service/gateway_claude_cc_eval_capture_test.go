//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claudeCCEvalFixture struct {
	AccountPrompt string             `json:"account_prompt"`
	Cases         []claudeCCEvalCase `json:"cases"`
}

type claudeCCEvalCase struct {
	ID    string          `json:"id"`
	Input json.RawMessage `json:"input"`
}

type claudeCCEvalCapture struct {
	ID      string          `json:"id"`
	Input   json.RawMessage `json:"input"`
	Body    json.RawMessage `json:"body"`
	Headers http.Header     `json:"headers"`
}

// TestExportClaudeCCEvaluationRequests captures the real gateway's outgoing
// requests with a fake upstream. It makes no network requests and needs no key.
func TestExportClaudeCCEvaluationRequests(t *testing.T) {
	fixturePath := os.Getenv("CLAUDE_CC_EVAL_FIXTURE")
	outputPath := os.Getenv("CLAUDE_CC_EVAL_OUTPUT")
	if fixturePath == "" || outputPath == "" {
		t.Skip("set evaluation fixture and output paths to capture requests")
	}
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	var fixture claudeCCEvalFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.NotEmpty(t, fixture.Cases)
	gin.SetMode(gin.TestMode)
	results := make([]claudeCCEvalCapture, 0, len(fixture.Cases))
	for _, item := range fixture.Cases {
		results = append(results, captureClaudeCCEvaluationRequest(t, item, fixture.AccountPrompt))
	}
	encoded, err := json.MarshalIndent(results, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outputPath, encoded, 0o600))
}

func captureClaudeCCEvaluationRequest(t *testing.T, item claudeCCEvalCase, prompt string) claudeCCEvalCapture {
	t.Helper()
	upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
	svc := &GatewayService{
		cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra = map[string]any{}
	account.Credentials["model_mapping"] = map[string]any{
		"ccA2/claude-opus-4-6": "claude-opus-4-6",
		"ccA2/claude-opus-5-5": "claude-opus-5-5",
	}
	account.ModelSystemPrompts = map[string]string{"claude-opus-4-6": prompt}
	group := &Group{ID: 84, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: "low"}
	ctx := svc.withGroupContext(context.Background(), group)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(item.Input)))
	c.Request.Header.Set("User-Agent", "SillyTavern-compatibility-evaluation")
	_, err := svc.ForwardAsChatCompletions(ctx, c, account, item.Input, &ParsedRequest{GroupID: &group.ID})
	require.NoError(t, err, item.ID)
	require.NotEmpty(t, upstream.lastBody, item.ID)
	headers := upstream.lastReq.Header.Clone()
	for _, name := range []string{"Authorization", "X-Api-Key", "Cookie"} {
		headers.Del(name)
	}
	return claudeCCEvalCapture{ID: item.ID, Input: item.Input, Body: upstream.lastBody, Headers: headers}
}
