package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type continuationBudgetUpstream struct {
	bodies    [][]byte
	responses []*http.Response
}

func (u *continuationBudgetUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		u.bodies = append(u.bodies, body)
	}
	if len(u.responses) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	resp := u.responses[0]
	u.responses = u.responses[1:]
	return resp, nil
}

func (u *continuationBudgetUpstream) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, accountID, concurrency)
}

func continuationBudgetCCResponse(content, finish string, input, output int) *http.Response {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"" + content + "\"},\"finish_reason\":\"" + finish + "\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":" + itoa(input) + ",\"completion_tokens\":" + itoa(output) + ",\"total_tokens\":" + itoa(input+output) + "}}\n\n" +
		"data: [DONE]\n\n"
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func continuationBudgetAnthropicResponse(content, stop string, input, output int) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(anthropicTextStream(content, stop, input, output)))}
}

func TestChatToAnthropicContinuationHonorsOriginalClientOutputBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, policyCap := range []int{0, 100} {
		t.Run("policy_"+itoa(policyCap), func(t *testing.T) {
			upstream := &continuationBudgetUpstream{responses: []*http.Response{
				continuationBudgetAnthropicResponse("partial", "max_tokens", 5, 15),
				continuationBudgetAnthropicResponse("done", "end_turn", 5, 5),
			}}
			cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true, AllowPrivateHosts: true}}}
			svc := &GatewayService{cfg: cfg, httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
			account := newAnthropicAPIKeyAccountForTest()
			body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":20,"stream":true,"messages":[{"role":"user","content":"question"}]}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
			ctx := WithRawChatAntiTruncationPolicy(context.Background(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: policyCap})
			result, err := svc.ForwardAsChatCompletions(ctx, c, account, body, nil)
			require.NoError(t, err)
			require.Equal(t, 1, result.AntiTruncationRounds)
			require.Len(t, upstream.bodies, 2)
			require.Equal(t, int64(20), gjson.GetBytes(upstream.bodies[0], "max_tokens").Int())
			require.Equal(t, int64(5), gjson.GetBytes(upstream.bodies[1], "max_tokens").Int())
		})
	}
}

func TestChatToAnthropicDoesNotTreatConvertedDefaultAsClientCumulativeCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &continuationBudgetUpstream{responses: []*http.Response{
		continuationBudgetAnthropicResponse("partial", "max_tokens", 5, 15),
		continuationBudgetAnthropicResponse("done", "end_turn", 5, 5),
	}}
	cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true, AllowPrivateHosts: true}}}
	svc := &GatewayService{cfg: cfg, httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
	account := newAnthropicAPIKeyAccountForTest()
	body := []byte(`{"model":"claude-sonnet-4-5","stream":true,"messages":[{"role":"user","content":"question"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	ctx := WithRawChatAntiTruncationPolicy(context.Background(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1})
	result, err := svc.ForwardAsChatCompletions(ctx, c, account, body, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, int64(8192), gjson.GetBytes(upstream.bodies[0], "max_tokens").Int(), "converter default remains the per-round upstream limit")
	require.Equal(t, int64(8192), gjson.GetBytes(upstream.bodies[1], "max_tokens").Int(), "converter default must not become a cumulative client cap")
}

func TestAnthropicMessagesChatFallbackContinuationHonorsOriginalClientOutputBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, policyCap := range []int{0, 100} {
		t.Run("policy_"+itoa(policyCap), func(t *testing.T) {
			upstream := &continuationBudgetUpstream{responses: []*http.Response{
				continuationBudgetCCResponse("partial", "length", 5, 15),
				continuationBudgetCCResponse("done", "stop", 5, 5),
			}}
			cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true, AllowPrivateHosts: true}}}
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
			account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "test-local-only", "base_url": "https://api.openai.com"}}
			group := &Group{ModelAliases: []domain.ModelAlias{{Alias: "public-alias", UpstreamModel: "gpt-5.4", AntiTruncation: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: policyCap}}}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"public-alias","max_tokens":20,"stream":true,"messages":[{"role":"user","content":"question"}]}`))
			c.Set("api_key", &APIKey{Group: group})
			result, err := svc.forwardAnthropicViaRawChatCompletions(c.Request.Context(), c, account, []byte(`{"model":"public-alias","max_tokens":20,"stream":true,"messages":[{"role":"user","content":"question"}]}`), "gpt-5.4")
			require.NoError(t, err)
			require.Equal(t, 1, result.AntiTruncationRounds)
			require.Len(t, upstream.bodies, 2)
			require.Equal(t, int64(20), gjson.GetBytes(upstream.bodies[0], "max_completion_tokens").Int())
			require.Equal(t, int64(5), gjson.GetBytes(upstream.bodies[1], "max_completion_tokens").Int())
		})
	}
}
