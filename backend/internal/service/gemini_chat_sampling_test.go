//go:build unit

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiChatCompletionsPreservesSamplingParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeServiceAccount, AccountTypeOAuth} {
		for _, stream := range []bool{false, true} {
			for _, tc := range []struct {
				name        string
				seedField   string
				wantSeed    string
				toolHistory bool
			}{
				{name: "omitted"},
				{name: "null", seedField: `,"seed":null`},
				{name: "zero", seedField: `,"seed":0`, wantSeed: "0"},
				{name: "positive", seedField: `,"seed":202`, wantSeed: "202"},
				{name: "negative", seedField: `,"seed":-101`, wantSeed: "-101"},
				{name: "large_integer", seedField: `,"seed":9007199254740993`, wantSeed: "9007199254740993"},
				{name: "tool_history", seedField: `,"seed":9007199254740993`, wantSeed: "9007199254740993", toolHistory: true},
			} {
				t.Run(fmt.Sprintf("%s/stream=%t/%s", accountType, stream, tc.name), func(t *testing.T) {
					svc, upstream := newGeminiVertexAPIKeyTestService()
					account := geminiSignalTestAccount()
					if accountType == AccountTypeServiceAccount {
						account = geminiVertexAPIKeyTestAccount("")
					}
					responseBody := geminiVertexAPIKeyTestResponse
					generationPath := "generationConfig"
					clientModel := "ultra/gemini-4-argon"
					if accountType == AccountTypeOAuth {
						account.Type = AccountTypeOAuth
						account.Credentials["project_id"] = "sampling-test-project"
						svc.tokenProvider = NewGeminiTokenProvider(nil, &fakeGeminiTokenCache{token: "sampling-test-token"}, nil)
						responseBody = `{"response":` + responseBody + `}`
						generationPath = "request.generationConfig"
						clientModel = "gemini-4-argon"
					}
					account.Credentials["model_mapping"] = map[string]any{
						"ultra/gemini-4-argon": "gemini-4-argon",
					}
					if stream || accountType == AccountTypeOAuth {
						upstream.response.Header.Set("Content-Type", "text/event-stream")
						upstream.response.Body = io.NopCloser(strings.NewReader("data: " + responseBody + "\n\n"))
					}
					messages := `[{"role":"user","content":"hello"}]`
					tools := ""
					if tc.toolHistory {
						messages = `[{"role":"user","content":"look up a value"},{"role":"assistant","tool_calls":[{"id":"call_seed","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_seed","content":"done"},{"role":"user","content":"hello"}]`
						tools = `,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object","properties":{}}}}]`
					}
					body := []byte(fmt.Sprintf(`{"model":"%s","messages":%s,"stream":%t,"temperature":0,"top_p":0.95%s%s}`, clientModel, messages, stream, tc.seedField, tools))
					recorder := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(recorder)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
					c.Request.Header.Set("Content-Type", "application/json")

					result, err := svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body)
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Equal(t, 1, upstream.calls)
					require.Equal(t, stream, result.Stream)
					require.Equal(t, clientModel, result.Model)
					require.Equal(t, "gemini-4-argon", result.UpstreamModel)
					require.Equal(t, 10, result.Usage.InputTokens)
					require.Equal(t, 5, result.Usage.OutputTokens)

					sentBody, err := upstream.lastReq.GetBody()
					require.NoError(t, err)
					defer sentBody.Close()
					sent, err := io.ReadAll(sentBody)
					require.NoError(t, err)
					require.True(t, gjson.GetBytes(sent, generationPath+".temperature").Exists())
					require.Zero(t, gjson.GetBytes(sent, generationPath+".temperature").Float())
					require.Equal(t, 0.95, gjson.GetBytes(sent, generationPath+".topP").Float())
					seed := gjson.GetBytes(sent, generationPath+".seed")
					require.Equal(t, tc.wantSeed != "", seed.Exists(), string(sent))
					require.Equal(t, tc.wantSeed, seed.Raw)
				})
			}
		}
	}
}

func TestGeminiChatSamplingPreservesSeedInSearchNormalization(t *testing.T) {
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"generationConfig":{"seed":9007199254740993},"tools":[{"googleSearch":{}}]}`)
	normalized := normalizeGeminiRequestForAIStudio(body)
	require.Equal(t, "9007199254740993", gjson.GetBytes(normalized, "generationConfig.seed").Raw)
	require.True(t, gjson.GetBytes(normalized, "tools.0.google_search").Exists())
}

func TestGeminiChatSamplingPreservesOtherBridgeDecoding(t *testing.T) {
	for _, seed := range []string{`"legacy"`, "9223372036854775808"} {
		t.Run(seed, func(t *testing.T) {
			var request apicompat.ChatCompletionsRequest
			err := json.Unmarshal([]byte(fmt.Sprintf(`{"model":"claude-sonnet-4-6","messages":[],"seed":%s}`, seed)), &request)
			require.NoError(t, err)
		})
	}
}

func TestGeminiChatCompletionsRejectsInvalidSeed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, seed := range []string{`"202"`, "1.5", "9223372036854775808"} {
		t.Run(seed, func(t *testing.T) {
			svc, upstream := newGeminiVertexAPIKeyTestService()
			body := []byte(fmt.Sprintf(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"seed":%s}`, seed))
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))

			_, err := svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiSignalTestAccount(), body)
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Zero(t, upstream.calls)
		})
	}
}
