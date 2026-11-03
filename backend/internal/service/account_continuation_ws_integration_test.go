//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountContinuationNativeWebSocketReusesLeaseAndRoutesNextPublicID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	upstream := &openAIWSCaptureConn{}
	for i, status := range []string{"incomplete", "completed"} {
		id, text := fmt.Sprintf("resp_%d", i+1), []string{"alpha", "beta"}[i]
		upstream.events = append(upstream.events, []byte(fmt.Sprintf(`{"type":"response.created","response":{"id":"%s","status":"in_progress"}}`, id)), []byte(fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"m","output_index":0,"delta":"%s"}`, text)), []byte(fmt.Sprintf(`{"type":"response.%s","response":{"id":"%s","model":"gpt-5.1","status":"%s","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":"%s"}]}],"usage":{"input_tokens":10,"output_tokens":2}}}`, status, id, status, text)))
	}
	upstream.events = append(upstream.events, []byte(`{"type":"response.completed","response":{"id":"resp_3","model":"gpt-5.1","status":"completed","usage":{"input_tokens":5,"output_tokens":1}}}`))
	dialer := &openAIWSCaptureDialer{conn: upstream}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
	account := &Account{ID: 114, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test"}, Extra: map[string]any{"responses_websockets_v2_enabled": true, "anti_truncation": map[string]any{"version": 1, "rules": []any{map[string]any{"model": "gpt-5.1", "enabled": true, "max_rounds": 1}}}}}
	done := make(chan error, 1)
	settled := make(chan *OpenAIForwardResult, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			done <- err
			return
		}
		defer conn.CloseNow()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r.Clone(r.Context())
		c.Set("api_key", &APIKey{ID: 9, UserID: 11})
		_, first, err := conn.Read(r.Context())
		if err != nil {
			done <- err
			return
		}
		done <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "sk-test", first, &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) {
			if result != nil {
				settled <- result
			}
		}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"write long","stream":true}`)))
	var frames [][]byte
	for {
		_, frame, err := client.Read(ctx)
		require.NoError(t, err)
		frames = append(frames, frame)
		if gjson.GetBytes(frame, "type").String() == "response.completed" {
			break
		}
	}
	joined := stringJoinFrames(frames)
	require.NotContains(t, joined, "response.incomplete")
	require.Equal(t, 1, strings.Count(joined, `"type":"response.created"`))
	require.Contains(t, joined, "alpha")
	require.Contains(t, joined, "beta")
	require.NotContains(t, joined, `"id":"resp_2"`)
	var result *OpenAIForwardResult
	select {
	case result = <-settled:
	case <-ctx.Done():
		t.Fatal("timed out waiting first logical turn settlement")
	}
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Len(t, result.ContinuationResults, 2)
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"next","previous_response_id":"resp_1","stream":true}`)))
	_, frame, err := client.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "resp_3", gjson.GetBytes(frame, "response.id").String())
	select {
	case <-settled:
	case <-ctx.Done():
		t.Fatal("timed out waiting second logical turn settlement")
	}
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timed out waiting websocket completion")
	}
	require.Equal(t, 1, dialer.DialCount())
	require.Len(t, upstream.writes, 3)
	body, err := json.Marshal(upstream.writes[1])
	require.NoError(t, err)
	require.Contains(t, string(body), "alpha")
	require.Equal(t, "resp_2", upstream.writes[2]["previous_response_id"])
}

func TestAccountContinuationNativeWebSocketPartialErrorRetainsBillingEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	upstream := &openAIWSCaptureConn{}
	for i, status := range []string{"incomplete"} {
		id, text := fmt.Sprintf("resp_%d", i+1), []string{"alpha", "beta"}[i]
		upstream.events = append(upstream.events, []byte(fmt.Sprintf(`{"type":"response.created","response":{"id":"%s","status":"in_progress"}}`, id)), []byte(fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"m","output_index":0,"delta":"%s"}`, text)), []byte(fmt.Sprintf(`{"type":"response.%s","response":{"id":"%s","model":"gpt-5.1","status":"%s","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":"%s"}]}],"usage":{"input_tokens":10,"output_tokens":2}}}`, status, id, status, text)))
	}
	upstream.events = append(upstream.events,
		[]byte(`{"type":"response.created","response":{"id":"resp_2","status":"in_progress"}}`),
		[]byte(`{"type":"response.output_text.delta","item_id":"m","output_index":0,"delta":"beta"}`),
		[]byte(`{"type":"response.in_progress","response":{"id":"resp_2","model":"gpt-5.1","usage":{"input_tokens":11,"output_tokens":1}}}`))

	dialer := &openAIWSCaptureDialer{conn: upstream}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
	account := &Account{ID: 114, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test"}, Extra: map[string]any{"responses_websockets_v2_enabled": true, "anti_truncation": map[string]any{"version": 1, "rules": []any{map[string]any{"model": "gpt-5.1", "enabled": true, "max_rounds": 1}}}}}
	done := make(chan error, 1)
	settled := make(chan *OpenAIForwardResult, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			done <- err
			return
		}
		defer conn.CloseNow()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r.Clone(r.Context())
		c.Set("api_key", &APIKey{ID: 9, UserID: 11})
		_, first, err := conn.Read(r.Context())
		if err != nil {
			done <- err
			return
		}
		done <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "sk-test", first, &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) {
			if result != nil {
				settled <- result
			}
		}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"write long","stream":true}`)))

	var frames [][]byte
	for {
		_, frame, readErr := client.Read(ctx)
		if readErr != nil {
			break
		}
		frames = append(frames, frame)
	}
	var result *OpenAIForwardResult
	select {
	case result = <-settled:
	case <-ctx.Done():
		t.Fatal("partial turn billing result was dropped")
	}
	require.NotNil(t, result)
	require.Equal(t, 21, result.Usage.InputTokens)
	require.Len(t, result.ContinuationResults, 2)
	require.True(t, result.ContinuationResults[1].Outcome.HasText)
	require.Equal(t, "gpt-5.1", result.ContinuationResults[1].UpstreamResponseModel, "partial rounds retain observed billing model evidence")
	require.Equal(t, UsageRequestInterrupted, ClassifyUsageRequestStatus(result.ContinuationResults[1].Outcome))
	require.Contains(t, stringJoinFrames(frames), "beta")
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Positive(t, *result.DeliveredOutputTokens)
	require.Contains(t, stringJoinFrames(frames), "alpha")
	require.NotContains(t, stringJoinFrames(frames), "response.completed")
	select {
	case err := <-done:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("websocket error did not finish")
	}
	require.Len(t, upstream.writes, 2)
	require.Equal(t, 1, dialer.DialCount())
}
