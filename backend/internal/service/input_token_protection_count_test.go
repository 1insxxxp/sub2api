//go:build unit

package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateCustomerInputTokenProtectionSemanticFields(t *testing.T) {
	// Each marker independently changes the estimate by the tokenizer's
	// count, including schema property names and structured tool results.
	for _, tc := range []struct {
		path, body string
	}{
		{"/v1/chat/completions", `{"messages":[{"role":"system","content":"SYS"},{"role":"user","content":"HIST"},{"role":"assistant","content":null,"tool_calls":[{"id":"c","type":"function","function":{"name":"run","arguments":"ARGS"}}]},{"role":"tool","tool_call_id":"c","content":"RESULT"}],"tools":[{"type":"function","function":{"name":"run","parameters":{"properties":{"SCHEMA":{"type":"string"}}}}}]}`},
		{"/antigravity/v1/messages", `{"system":[{"type":"text","text":"SYS"}],"messages":[{"role":"user","content":"HIST"},{"role":"assistant","content":[{"type":"tool_use","id":"c","name":"run","input":{"x":"ARGS"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c","content":[{"type":"text","text":"RESULT"}]}]}],"tools":[{"name":"run","input_schema":{"properties":{"SCHEMA":{}}}}]}`},
		{"/responses", `{"instructions":"SYS","input":[{"role":"developer","content":[{"type":"input_text","text":"HIST"}]},{"type":"function_call","call_id":"c","name":"run","arguments":"ARGS"},{"type":"function_call_output","call_id":"c","output":{"type":"forecast","RESULT":true}}],"tools":[{"name":"run","type":"function","parameters":{"description":"SCHEMA"}}]}`},
		{"/v1beta/models/gemini-model:streamGenerateContent?alt=sse", `{"systemInstruction":{"parts":[{"text":"SYS"}]},"contents":[{"parts":[{"text":"HIST"},{"functionCall":{"name":"run","args":{"x":"ARGS"}}},{"functionResponse":{"name":"run","response":{"RESULT":true}}}]}],"tools":[{"functionDeclarations":[{"name":"run","parametersJsonSchema":{"properties":{"SCHEMA":{}}}}]}]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			base := EstimateCustomerInputTokenProtection(tc.path, []byte(tc.body))
			require.False(t, base.Pending)
			phrase := strings.Repeat(" world", 160)
			codec, err := openAIInputTokensCodecForModel("")
			require.NoError(t, err)
			n, err := codec.Count(phrase)
			require.NoError(t, err)
			for _, key := range []string{"SYS", "HIST", "ARGS", "RESULT", "SCHEMA"} {
				extended := strings.Replace(tc.body, key, key+phrase, 1)
				got := EstimateCustomerInputTokenProtection(tc.path, []byte(extended))
				require.False(t, got.Pending, key)
				require.InDelta(t, n, got.Limit-base.Limit, 2, key)
			}
		})
	}
}

func TestEstimateCustomerInputTokenProtectionWS(t *testing.T) {
	const req = `{"model":"gpt-4o","instructions":"stay brief","input":[{"role":"user","content":"你好"}],"tools":[{"type":"function","name":"info"}]}`
	want := EstimateCustomerInputTokenProtection("/v1/responses", []byte(req))
	require.False(t, want.Pending)
	for _, frame := range []string{
		`{"type":"response.create",` + req[1:],
		`{"type":"response.create","event_id":"client-event","response":` + req + `}`,
	} {
		for _, path := range []string{"/backend-api/codex/responses", "response.create", ""} {
			require.Equal(t, want, EstimateCustomerInputTokenProtection(path, []byte(frame)))
		}
	}
}

func TestEstimateCustomerInputTokenProtectionIgnoresTransport(t *testing.T) {
	base := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"f","description":"run","parameters":{"type":"object"}}}]}`
	extra := strings.Repeat("unrelated ", 2000)
	inflated := strings.NewReplacer(
		`"model":`, `"metadata":{"note":"`+extra+`"},"tool_choice":"auto","stream":true,"max_tokens":200000,"temperature":0.7,"model":`,
		`"role":`, `"metadata":"`+extra+`","role":`,
		`"description":`, `"client_metadata":"`+extra+`","description":`,
	).Replace(base)
	estimate := func(body string) *InputTokenProtection {
		return EstimateCustomerInputTokenProtection("/v1/chat/completions", []byte(body))
	}
	want := estimate(base)
	require.False(t, want.Pending)
	require.Equal(t, want, estimate(inflated))
}

func TestEstimateCustomerInputTokenProtectionEmbeddings(t *testing.T) {
	texts := []string{"你好", "the second input"}
	codec, err := openAIInputTokensCodecForModel("text-embedding-3-large")
	require.NoError(t, err)
	total := 0
	for _, text := range texts {
		want, err := codec.Count(text)
		require.NoError(t, err)
		total += want
		body, err := json.Marshal(map[string]any{"model": "text-embedding-3-large", "input": text})
		require.NoError(t, err)
		require.Equal(t, &InputTokenProtection{Limit: want}, EstimateCustomerInputTokenProtection("/v1/embeddings", body))
	}
	body, err := json.Marshal(map[string]any{"model": "text-embedding-3-large", "input": texts, "dimensions": 1024})
	require.NoError(t, err)
	require.Equal(t, &InputTokenProtection{Limit: total}, EstimateCustomerInputTokenProtection("/embeddings", body))
}

func TestEstimateCustomerInputTokenProtectionPending(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/messages", `{"messages":[{"role":"user","content":[{"type":"text","text":"known"},{"type":"image","source":{"type":"base64","data":"AAA"}}]}]}`},
		{"/chat/completions", `{"messages":[{"role":"assistant","content":"known","audio":{"id":"audio123"}}]}`},
		{"/responses", `{"input":[{"type":"function_call_output","call_id":"c","output":[{"type":"input_image","image_url":"url"}]}]}`},
		{"/responses", `{"input":[{"type":"item_reference","id":"old"}]}`},
		{"/responses", `{"input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"known"}],"encrypted_content":"opaque"}]}`},
		{"/responses", `{"previous_response_id":"r","input":"known"}`},
		{"/responses", `{"type":"response.create","response":{"previous_response_id":"r","input":"known"}} `},
		{"/responses", `{"type":"response.create","input":"known","response":{"input":"ambiguous"}}`},
		{"/responses", `{"conversation":{"id":"c"},"input":"known"}`},
		{"/responses", `{"prompt":{"id":"p"},"input":"known"}`},
		{"/v1beta/models", `{"cachedContent":"cachedContents/x","contents":[{"parts":[{"text":"known"}]}]}`},
		{"/v1beta/models", `{"contents":[{"parts":[{"text":"known"},{"inlineData":{"data":"AA"}}]}]}`},
		{"/v1beta/models", `{"contents":[{"parts":[{"functionResponse":{"name":"f","response":{},"parts":[{"fileData":{}}]}}]}]}`},
		{"/embeddings", `{"input":["known",{}]}`},
		{"/messages", `{"messages":[{"role":"user","content":[{"type":"undocumented"}]}]}`},
		{"/messages", `{"messages":[{}],"tools":[{"name":"run"}]}`},
		{"/responses", `{"input":"a","input":"b"}`},
		{"/responses", `{"input":"a"}{}`},
		{"/responses", `{"input":`},
		{"/responses", `{}`},
		{"/images/generations", `{"input":"known"}`},
	} {
		require.Equal(t, &InputTokenProtection{Pending: true}, EstimateCustomerInputTokenProtection(tc.path, []byte(tc.body)), "%s %s", tc.path, tc.body)
	}
}

func TestEstimateCustomerInputTokenProtectionBudgets(t *testing.T) {
	for _, body := range [][]byte{
		bytes.Repeat([]byte(" "), geminiInputCountMaxRequestBytes+1),
		[]byte(`{"input":"known","metadata":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}`),
		[]byte(`{"input":"known","metadata":[` + strings.Repeat("0,", 40_000) + `0]}`),
	} {
		require.Equal(t, &InputTokenProtection{Pending: true}, EstimateCustomerInputTokenProtection("/responses", body))
	}
	const emptyPart = `{"type":"input_text","text":""},`
	body := `{"input":[{"role":"user","content":[` + strings.Repeat(emptyPart, 1024) + `{"type":"input_text","text":"hello"}]}]}`
	got := EstimateCustomerInputTokenProtection("/responses", []byte(body))
	require.False(t, got.Pending)
	require.Equal(t, 1+256, got.Limit)
}

func TestEstimateCustomerInputTokenProtectionSystemOne(t *testing.T) {
	const path = "/v1/systemone"
	for _, template := range []string{
		`{"model":"jev-latest","state":"STATE","questions":{"LABEL":{"type":"noul","instructions":"INSTR","criteria":{"true":"CRIT","false":null}}}}`,
		`{"model":"jev-latest","state":{"STATE":"report"},"questions":{"LABEL":{"type":"choice","instructions":{"task":"INSTR"},"criteria":{"CRIT":{"explanation":["A"]},"b":null}}}}`,
		`{"model":"jev-latest","state":["STATE"],"questions":{"LABEL":{"type":"score","instructions":["INSTR"],"criteria":[{"description":"CRIT"}]}}}`,
	} {
		base := EstimateCustomerInputTokenProtection(path, []byte(template))
		require.False(t, base.Pending)
		for _, field := range []string{"STATE", "LABEL", "INSTR", "CRIT"} {
			expanded := strings.Replace(template, field, field+strings.Repeat(" world", 100), 1)
			got := EstimateCustomerInputTokenProtection(path, []byte(expanded))
			require.False(t, got.Pending)
			require.InDelta(t, 100, got.Limit-base.Limit, 2, field)
		}
		settings := `{"stream":false,"metadata":"` + strings.Repeat("unused", 2000) + `",` + template[1:]
		require.Equal(t, base, EstimateCustomerInputTokenProtection(path, []byte(settings)))
	}
	for _, raw := range []string{
		`{"model":"jev-latest","state":42,"questions":{"q":{"type":"noul"}}}`,
		`{"model":"jev-latest","state":"x","questions":{"q":{"type":"choice"}}}`,
		`{"model":"jev-latest","state":"x","questions":{"q":{"type":"noul"}},"stream":true}`,
		`{"model":"jev-latest","state":"x","questions":{"q":{"type":"score","criteria":["ok"],"extension":"opaque"}}}`,
		`{"model":"jev-latest","state":[{"image_url":{"url":"opaque"}}],"questions":{"q":{"type":"noul"}}}`,
		`{"model":"jev-latest","state":"x","questions":{"q":{"type":"noul","instructions":{"ref":{"cachedContent":"remote"}}}}}`,
	} {
		require.Equal(t, &InputTokenProtection{Pending: true}, EstimateCustomerInputTokenProtection(path, []byte(raw)))
	}
}

func TestEstimateCustomerInputTokenProtectionGeminiCacheCreate(t *testing.T) {
	const path = "/v1beta/cachedContents"
	const contents = `"contents":[{"parts":[{"text":"history"}]}]`
	const system = `"systemInstruction":{"parts":[{"text":"system prompt"}]}`
	const tools = `"tools":[{"functionDeclarations":[{"name":"lookup","description":"find a city"}]}]`
	count := func(fields string) *InputTokenProtection {
		return EstimateCustomerInputTokenProtection(path, []byte(`{"model":"models/gemini-test",`+fields+`}`))
	}
	contentOnly, systemOnly, toolsOnly := count(contents), count(system), count(tools)
	for _, got := range []*InputTokenProtection{contentOnly, systemOnly, toolsOnly} {
		require.False(t, got.Pending)
		require.Positive(t, got.Limit)
	}
	all := strings.Join([]string{contents, system, tools}, ",")
	want := &InputTokenProtection{Limit: contentOnly.Limit + systemOnly.Limit + toolsOnly.Limit}
	require.Equal(t, want, count(all))
	require.Equal(t, want, count(all+`,"ttl":"3600s","displayName":"test","usageMetadata":{"totalTokenCount":999999}`))
	require.Equal(t, systemOnly, count(`"contents":[],`+strings.Replace(system, "systemInstruction", "system_instruction", 1)))

	for _, fields := range []string{
		`"ttl":"120s"`,
		contents + `,"cachedContent":"cachedContents/other"`,
		system + `,"contents":[{"parts":[{"fileData":{"fileUri":"gs://bucket/file"}}]}]`,
		tools + `,"contents":{"unrecognized":true}`,
	} {
		require.Equal(t, &InputTokenProtection{Pending: true}, count(fields))
	}
	// The collection rule must not match the storage-extension resource path.
	require.True(t, EstimateCustomerInputTokenProtection(path+"/cache-id?updateMask=ttl", []byte("{"+all+"}")).Pending)
}

func TestEstimateCustomerInputTokenProtectionImages(t *testing.T) {
	const prompt = "Paint a watercolor of 海边 at dawn."
	codec, err := openAIInputTokensCodecForModel("gpt-image-1")
	require.NoError(t, err)
	want, err := codec.Count(prompt)
	require.NoError(t, err)
	for _, path := range []string{"/v1/images/generations", "/images/generations"} {
		for _, body := range []string{
			`{"prompt":"` + prompt + `"}`,
			`{"model":"gpt-image-1","prompt":"` + prompt + `","size":"1024x1024","quality":"high","n":3,"stream":false}`,
		} {
			require.Equal(t, &InputTokenProtection{Limit: want}, EstimateCustomerInputTokenProtection(path, []byte(body)))
		}
	}
	for _, body := range []string{
		`{"input":"no prompt"}`, `{"prompt":"  "}`, `{"prompt":["caption"]}`, `{"prompt":{"id":"remote"}}`,
		`{"prompt":"draw","image_url":"https://example.invalid/ref.png"}`, `{"prompt":"draw","mask":{"file_id":"f"}}`,
		`{"prompt":"draw","negative_prompt":"another model input"}`, `{"prompt":"draw","model":{"id":"custom"}}`,
		`{"prompt":"draw","size":{"reference":"file-id"}}`, `{"prompt":"draw","previous_response_id":"r"}`,
	} {
		require.Equal(t, &InputTokenProtection{Pending: true}, EstimateCustomerInputTokenProtection("/v1/images/generations", []byte(body)), body)
	}
	for _, path := range []string{"/v1/images/edits", "/images/edits", "/v1/images/generations/async"} {
		require.Equal(t, &InputTokenProtection{Pending: true}, EstimateCustomerInputTokenProtection(path, []byte(`{"prompt":"draw"}`)), path)
	}
}
