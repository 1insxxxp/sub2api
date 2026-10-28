//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Routing aliases are local identifiers, never literal model paths for another
// provider. This must also hold for Vertex and Code Assist, whose builders do
// not use the AI Studio URL guard.
func TestGeminiForwardNative_RoutingAliasRequiresSafeAccountMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const alias = "按量vet/gemini-4-argon"
	for _, provider := range []string{"api_key", "vertex_express", "vertex_project", "oauth", "code_assist"} {
		for _, target := range []string{"", "prefix/../gemini-4-argon", "gemini-4-argon?key=other"} {
			t.Run(provider+"/"+target, func(t *testing.T) {
				svc, upstream := newGeminiVertexAPIKeyTestService()
				account := geminiSignalTestAccount()
				switch provider {
				case "vertex_express":
					account = geminiVertexAPIKeyTestAccount("")
				case "vertex_project":
					account = geminiVertexAPIKeyTestAccount("test-project")
				case "oauth", "code_assist":
					account.Type = AccountTypeOAuth
					account.Credentials["access_token"] = "test-token"
					svc.tokenProvider = &GeminiTokenProvider{}
					if provider == "code_assist" {
						account.Credentials["project_id"] = "test-project"
					}
				}
				if target != "" {
					account.Credentials["model_mapping"] = map[string]any{alias: target}
				}
				c, recorder := newGeminiNativeTestContext(t)
				result, err := svc.ForwardNative(context.Background(), c, account, alias, "generateContent", false, geminiSignalTestRequest())
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Zero(t, upstream.calls, "unresolved or unsafe aliases must not reach any upstream")
			})
		}
	}
}

func TestGeminiForwardNative_RoutingAliasUsesMappedModel(t *testing.T) {
	const alias = "按量vet/gemini-4-argon"
	for _, provider := range []string{"api_key", "vertex_express", "vertex_project"} {
		t.Run(provider, func(t *testing.T) {
			svc, upstream := newGeminiVertexAPIKeyTestService()
			account := geminiSignalTestAccount()
			if provider == "vertex_express" {
				account = geminiVertexAPIKeyTestAccount("")
			} else if provider == "vertex_project" {
				account = geminiVertexAPIKeyTestAccount("test-project")
			}
			account.Credentials["model_mapping"] = map[string]any{alias: "gemini-4-argon"}
			c, recorder := newGeminiNativeTestContext(t)
			result, err := svc.ForwardNative(context.Background(), c, account, alias, "generateContent", false, geminiSignalTestRequest())
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, 1, upstream.calls)
			require.Contains(t, upstream.lastReq.URL.Path, "/models/gemini-4-argon:generateContent")
			require.Equal(t, alias, result.Model)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Equal(t, 5, result.Usage.OutputTokens)
		})
	}
}
