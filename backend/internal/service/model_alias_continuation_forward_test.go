package service

import (
	"bytes"
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

// Exercise the full outbound HTTP pipeline, including credentials and model
// rewriting, rather than only invoking the continuation parser callback.
type aliasContinuationHTTPTransport struct {
	accountIDs []int64
}

func (u *aliasContinuationHTTPTransport) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	return http.DefaultClient.Do(req)
}

func (u *aliasContinuationHTTPTransport) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, accountID, concurrency)
}

func TestModelAliasContinuationFullHTTPForward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, requestedModel := range []string{"story-continue", "gpt-5.4"} {
		t.Run(requestedModel, func(t *testing.T) {
			var upstreamBodies [][]byte
			var upstreamAuth, upstreamPaths []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				upstreamBodies = append(upstreamBodies, body)
				upstreamAuth = append(upstreamAuth, r.Header.Get("Authorization"))
				upstreamPaths = append(upstreamPaths, r.URL.Path)
				w.Header().Set("Content-Type", "text/event-stream")
				if len(upstreamBodies) == 1 {
					_, _ = io.WriteString(w, "data: {\"id\":\"first\",\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":3,\"total_tokens\":7}}\n\ndata: [DONE]\n\n")
					return
				}
				_, _ = io.WriteString(w, "data: {\"id\":\"second\",\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" continued\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":4,\"total_tokens\":12}}\n\ndata: [DONE]\n\n")
			}))
			defer upstream.Close()

			group := &Group{ModelAliases: []domain.ModelAlias{{Alias: "story-continue", UpstreamModel: "gpt-5.4", AntiTruncation: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxDurationSeconds: 30}}}}
			account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "test-local-only", "base_url": upstream.URL}}
			transport := &aliasContinuationHTTPTransport{}
			svc := &OpenAIGatewayService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true, AllowPrivateHosts: true}}}, httpUpstream: transport}
			body := []byte(`{"model":"` + requestedModel + `","stream":true,"max_tokens":100000,"messages":[{"role":"user","content":"question"}]}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			ctx := WithGroupAliasRawChatAntiTruncationPolicy(c.Request.Context(), group, requestedModel)
			target, _ := group.ResolveModelAlias(requestedModel)
			result, err := svc.forwardAsRawChatCompletions(ctx, c, account, body, target)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, requestedModel, result.Model)
			require.Equal(t, "gpt-5.4", result.UpstreamModel)
			require.Equal(t, 1, strings.Count(recorder.Body.String(), "data: [DONE]"))
			require.Contains(t, recorder.Body.String(), `"model":"`+requestedModel+`"`)
			for i, sent := range upstreamBodies {
				require.Equal(t, "gpt-5.4", gjson.GetBytes(sent, "model").String())
				wantMaxTokens := int64(100000)
				if i == 1 && requestedModel == "story-continue" {
					wantMaxTokens = 99997
				}
				require.Equal(t, wantMaxTokens, gjson.GetBytes(sent, "max_tokens").Int(), "continuation must use only the client's remaining cumulative output budget")
				require.True(t, gjson.GetBytes(sent, "stream_options.include_usage").Bool())
				require.Equal(t, "Bearer test-local-only", upstreamAuth[i])
				require.Equal(t, "/v1/chat/completions", upstreamPaths[i])
				require.Equal(t, account.ID, transport.accountIDs[i])
			}
			if requestedModel == "story-continue" {
				require.Len(t, upstreamBodies, 2)
				require.Equal(t, 1, result.AntiTruncationRounds)
				require.Equal(t, 12, result.Usage.InputTokens)
				require.Equal(t, 7, result.Usage.OutputTokens)
				require.Equal(t, "partial", gjson.GetBytes(upstreamBodies[1], "messages.1.content").String())
				require.Equal(t, "assistant", gjson.GetBytes(upstreamBodies[1], "messages.1.role").String())
				require.Equal(t, 1, strings.Count(recorder.Body.String(), `"finish_reason":"stop"`))
				require.NotContains(t, recorder.Body.String(), `"finish_reason":"length"`)
				require.Contains(t, recorder.Body.String(), `"completion_tokens":7`)
			} else {
				require.Len(t, upstreamBodies, 1)
				require.Zero(t, result.AntiTruncationRounds)
				require.Equal(t, 4, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
				require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
			}
		})
	}
}

func TestModelAliasContinuationNeverExceedsClientOutputBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, policyCap := range []int{0, 100} {
		t.Run("policy_cap_"+itoa(policyCap), func(t *testing.T) {
			var bodies [][]byte
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				bodies = append(bodies, body)
				w.Header().Set("Content-Type", "text/event-stream")
				if len(bodies) == 1 {
					_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":15,\"total_tokens\":20}}\n\ndata: [DONE]\n\n")
					return
				}
				_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" done\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\ndata: [DONE]\n\n")
			}))
			defer upstream.Close()

			group := &Group{ModelAliases: []domain.ModelAlias{{Alias: "story-continue", UpstreamModel: "gpt-5.4", AntiTruncation: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: policyCap, MaxDurationSeconds: 30}}}}
			account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "test-local-only", "base_url": upstream.URL}}
			transport := &aliasContinuationHTTPTransport{}
			svc := &OpenAIGatewayService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true, AllowPrivateHosts: true}}}, httpUpstream: transport}
			body := []byte(`{"model":"story-continue","stream":true,"max_tokens":20,"messages":[{"role":"user","content":"question"}]}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			ctx := WithGroupAliasRawChatAntiTruncationPolicy(c.Request.Context(), group, "story-continue")
			result, err := svc.forwardAsRawChatCompletions(ctx, c, account, body, "gpt-5.4")
			require.NoError(t, err)
			require.Equal(t, 1, result.AntiTruncationRounds)
			require.Len(t, bodies, 2)
			require.Equal(t, int64(20), gjson.GetBytes(bodies[0], "max_tokens").Int())
			require.Equal(t, int64(5), gjson.GetBytes(bodies[1], "max_tokens").Int())
		})
	}
}
