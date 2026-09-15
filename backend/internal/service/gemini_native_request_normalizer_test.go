package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeGeminiNativeRequestMapsMinimalThinkingLevel(t *testing.T) {
	body := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"thinkingConfig":{"thinkingLevel":"minimal"}
		}
	}`)

	normalized, err := normalizeGeminiNativeRequest(body)

	require.NoError(t, err)
	require.Equal(t, "low", gjson.GetBytes(normalized, "generationConfig.thinkingConfig.thinkingLevel").String())
}

func TestNormalizeGeminiNativeRequestEnablesMixedServerTools(t *testing.T) {
	body := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"tools":[
			{"functionDeclarations":[{"name":"lookup","parameters":{"type":"object"}}]},
			{"googleSearch":{}}
		]
	}`)

	normalized, err := normalizeGeminiNativeRequest(body)

	require.NoError(t, err)
	require.Equal(t, true, gjson.GetBytes(normalized, "toolConfig.includeServerSideToolInvocations").Bool())
}

func TestNormalizeGeminiNativeRequestMergesModelTurnsAndAppendsUserTurn(t *testing.T) {
	body := []byte(`{
		"contents":[
			{"role":"model","parts":[{"text":"first"}]},
			{"role":"model","parts":[{"text":"second"}]}
		]
	}`)

	normalized, err := normalizeGeminiNativeRequest(body)

	require.NoError(t, err)
	contents := gjson.GetBytes(normalized, "contents").Array()
	require.Len(t, contents, 2)
	require.Equal(t, "model", contents[0].Get("role").String())
	require.Len(t, contents[0].Get("parts").Array(), 2)
	require.Equal(t, "user", contents[1].Get("role").String())
	require.Contains(t, contents[1].Get("parts.0.text").String(), "Continue")
}

func TestNormalizeGeminiNativeRequestAlignsFunctionResponseName(t *testing.T) {
	body := []byte(`{
		"contents":[
			{"role":"model","parts":[{"functionCall":{"name":"use_package","args":{}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"package_proxy","response":{"content":"ok"}}}]}
		]
	}`)

	normalized, err := normalizeGeminiNativeRequest(body)

	require.NoError(t, err)
	require.Equal(t, "use_package", gjson.GetBytes(normalized, "contents.1.parts.0.functionResponse.name").String())
}
