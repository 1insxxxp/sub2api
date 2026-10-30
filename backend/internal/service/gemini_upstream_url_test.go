package service

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGeminiAIStudioModelActionURLTaggedModels(t *testing.T) {
	for _, model := range []string{"[P]gemini-4-argon", "[量]gemini-4-argon", "[渠道_2.1]gemini-3.8-flash"} {
		for _, action := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			t.Run(model+"/"+action, func(t *testing.T) {
				requests := make(chan *http.Request, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests <- r
					w.WriteHeader(http.StatusNoContent)
				}))
				defer server.Close()
				endpoint, err := buildGeminiAIStudioModelActionURL(server.URL+"/proxy", model, action, action == "streamGenerateContent")
				require.NoError(t, err)
				response, err := http.Post(endpoint, "application/json", strings.NewReader(`{}`))
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				require.Equal(t, http.StatusNoContent, response.StatusCode)
				observed := <-requests
				require.Equal(t, "/proxy/v1beta/models/"+model+":"+action, observed.URL.Path)
				require.Equal(t, "/proxy/v1beta/models/"+url.PathEscape(model)+":"+action, strings.SplitN(observed.RequestURI, "?", 2)[0])
				if action == "streamGenerateContent" {
					require.Equal(t, "alt=sse", observed.URL.RawQuery)
				} else {
					require.Empty(t, observed.URL.RawQuery)
				}
				require.False(t, IsSafeGeminiModelPathSegment(model), "public model metadata/cache validation stays strict")
				require.False(t, IsValidGeminiRoutingModel(model), "tagged targets are not native public routing names")
				require.False(t, isSafeUpstreamPathSegment(model), "the shared path guard must remain strict")
			})
		}
	}
}

func TestBuildGeminiAIStudioModelActionURLRejectsUnsafeTaggedModels(t *testing.T) {
	for _, model := range []string{
		"[]gemini", "[P]", "[量]..", "[量]...", "[P][量]gemini", "[[P]]gemini", "P]gemini", "[Pgemini",
		"[P]gemini/other", `[P]gemini\other`, "[P]gemini%2fother", "[P]gemini%252fother",
		"[P]gemini?key=x", "[P]gemini#x", "[P]gemini:countTokens", "[P]gemini@001",
		"[P/Q]gemini", `[P\Q]gemini`, "[P%51]gemini", "[P?Q]gemini", "[P#Q]gemini", "[P:Q]gemini",
		"[P Q]gemini", "[P\tQ]gemini", "[P\x00Q]gemini", "[P\u202eQ]gemini", "[P\ufffdQ]gemini",
		"[P]gemini model", "[P]gemini\x00model", "[量]" + strings.Repeat("a", 124),
		"[" + strings.Repeat("量", 43) + "]gemini", "[P]models/gemini", "[P]../gemini",
	} {
		t.Run(model, func(t *testing.T) {
			_, err := buildGeminiAIStudioModelActionURL("https://example.invalid", model, "generateContent", false)
			require.Error(t, err)
			require.NotContains(t, err.Error(), model, "unsafe input must not be reflected in errors")
		})
	}
}

func TestGeminiRoutingModelValidationKeepsUpstreamPathGuardStrict(t *testing.T) {
	for _, model := range []string{"gemini-4-argon", "按量vet/gemini-4-argon", "优惠/Gemini_4.0", strings.Repeat("模", 200)} {
		require.True(t, IsValidGeminiRoutingModel(model), model)
		if strings.ContainsAny(model, "/模") {
			_, err := buildGeminiAIStudioModelActionURL("https://example.invalid", model, "generateContent", false)
			require.Error(t, err, "a routing name must never loosen the upstream URL guard")
		}
	}
	for _, model := range []string{
		"", "/gemini", "gemini/", "a//b", "a/../b", "a/./b", "a/.../b",
		"a%2fb", "a%252fb", "a?key=b", "a#b", "a:b", `a\b`, "a\x00b",
		"a\nb", "a\u202eb", "a\ufffdb", " a", "a ", "a\t", "a\n", "a b",
		strings.Repeat("a/", 8) + "b", strings.Repeat("模", 201),
	} {
		require.False(t, IsValidGeminiRoutingModel(model), model)
	}
}

func TestBuildGeminiAIStudioModelActionURL(t *testing.T) {
	const base = "https://generativelanguage.googleapis.com"

	got, err := buildGeminiAIStudioModelActionURL(base, "gemini-2.5-pro", "generateContent", false)
	require.NoError(t, err)
	require.Equal(t, base+"/v1beta/models/gemini-2.5-pro:generateContent", got)

	got, err = buildGeminiAIStudioModelActionURL(base+"/", " gemini-2.5-flash ", "streamGenerateContent", true)
	require.NoError(t, err)
	require.Equal(t, base+"/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", got)

	got, err = buildGeminiAIStudioModelActionURL(base, "gemini-2.5-pro", "countTokens", false)
	require.NoError(t, err)
	require.Equal(t, base+"/v1beta/models/gemini-2.5-pro:countTokens", got)
}

// TestBuildGeminiAIStudioModelActionURLRejectsNonConformingModel 锁定不变式：
// 模型名来自客户端（native 路由的 URL 片段 / compat 路由的请求体），
// 只有合规的路径片段才允许拼进上游 URL。
func TestBuildGeminiAIStudioModelActionURLRejectsNonConformingModel(t *testing.T) {
	const base = "https://generativelanguage.googleapis.com"

	for _, model := range []string{
		"../../x/y",
		"..",
		".",
		"gemini-2.5-pro/../../x",
		`..\..\x`,
		"gemini-2.5-pro?a=b",
		"gemini-2.5-pro#frag",
		"gemini-2.5-pro%2f..",
		"gemini 2.5 pro",
		"gemini\x00pro",
		"gemini-2.5-pro@001",
		"gemini~pro",
		"models/gemini-2.5-pro",
		"...",
		"",
		"   ",
	} {
		t.Run("model_"+model, func(t *testing.T) {
			_, err := buildGeminiAIStudioModelActionURL(base, model, "generateContent", false)
			require.Error(t, err, "model %q must be rejected", model)
			require.False(t, IsSafeGeminiModelPathSegment(model))
		})
	}

	// action 只允许已知取值，避免未来把可变字符串拼进 path。
	_, err := buildGeminiAIStudioModelActionURL(base, "gemini-2.5-pro", "deleteModel", false)
	require.Error(t, err)

	_, err = buildGeminiAIStudioModelActionURL("", "gemini-2.5-pro", "generateContent", false)
	require.Error(t, err)
}
