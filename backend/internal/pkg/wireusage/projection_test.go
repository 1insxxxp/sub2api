package wireusage

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func limit(n int) CapFunc {
	return func(r, i, s, l int) (int, int, int, int) {
		r = min(r, n)
		i = min(i, n-r)
		s = min(s, n-r-i)
		return r, i, s, min(l, n-r-i-s)
	}
}

func TestProjectedUsage(t *testing.T) {
	for _, tc := range []struct {
		name      string
		messages  bool
		budget    int
		raw, want string
	}{
		{"chat", false, 10, `{"choices":[{"message":{"content":"usage"}}],"usage":{"prompt_tokens":100,"prompt_cache_hit_tokens":8,"prompt_cache_miss_tokens":92,"prompt_tokens_details":{"cached_tokens":8},"completion_tokens":5,"total_tokens":108}}`, `{"choices":[{"message":{"content":"usage"}}],"usage":{"prompt_tokens":10,"prompt_cache_hit_tokens":8,"prompt_cache_miss_tokens":2,"prompt_tokens_details":{"cached_tokens":8},"completion_tokens":5,"total_tokens":18}}`},
		{"responses", false, 10, `{"response":{"usage":{"input_tokens":14,"input_tokens_details":{"cached_tokens":3,"cache_write_tokens":8,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":6}},"output_tokens":4,"total_tokens":18}}}`, `{"response":{"usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":3,"cache_write_tokens":4,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":2}},"output_tokens":4,"total_tokens":14}}}`},
		{"messages", true, 8, `{"message":{"usage":{"input_tokens":3,"cache_read_input_tokens":2,"cache_creation_input_tokens":8,"cache_creation":{"ephemeral_5m_input_tokens":2,"ephemeral_1h_input_tokens":6},"output_tokens":7}}}`, `{"message":{"usage":{"input_tokens":3,"cache_read_input_tokens":2,"cache_creation_input_tokens":3,"cache_creation":{"ephemeral_5m_input_tokens":2,"ephemeral_1h_input_tokens":1},"output_tokens":7}}}`},
		{"gemini array", false, 4, `[{"response":{"usageMetadata":{"promptTokenCount":20,"cachedContentTokenCount":5,"totalTokenCount":30,"promptTokensDetails":[{"modality":"TEXT","tokenCount":20}],"cacheTokensDetails":[{"modality":"TEXT","tokenCount":5}],"thoughtsTokenCount":6}}}]`, `[{"response":{"usageMetadata":{"promptTokenCount":4,"cachedContentTokenCount":4,"totalTokenCount":14,"promptTokensDetails":[{"modality":"TEXT","tokenCount":4}],"cacheTokensDetails":[{"modality":"TEXT","tokenCount":4}],"thoughtsTokenCount":6}}}]`},
		{"pending", false, 0, `{"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":33},"output_tokens":5,"total_tokens":105}}`, `{"usage":{"input_tokens":0,"input_tokens_details":{"cached_tokens":0},"output_tokens":5,"total_tokens":5}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(tc.raw)
			p := New(limit(tc.budget), tc.messages)
			got := p.JSON(raw)
			require.JSONEq(t, tc.want, string(got))
			require.Equal(t, tc.raw, string(raw))
			stream := New(limit(tc.budget), tc.messages)
			var out bytes.Buffer
			frame := "data: " + tc.raw + "\r\n\r\n"
			for i := 0; i < len(frame); i += 7 {
				checkWrite(t, stream.WriteSSE, &out, []byte(frame[i:min(i+7, len(frame))]))
			}
			require.JSONEq(t, tc.want, strings.TrimSpace(strings.TrimPrefix(out.String(), "data: ")))
		})
	}
}

func TestFragmentedStream(t *testing.T) {
	p := New(limit(10), true)
	var out bytes.Buffer
	raw := ": ping\r\n\r\nevent: message_start\r\ndata: {\"message\":{\"usage\":{\"input_tokens\":20,\"cache_read_input_tokens\":0}}}\r\n\r\ndata: {\"type\":\"message_delta\",\"usage\":{\"cache_read_input_tokens\":8,\"output_tokens\":7}}\n\ndata: [DONE]\n\n"
	for i := range len(raw) {
		checkWrite(t, p.WriteSSE, &out, []byte(raw[i:i+1]))
	}
	lines := strings.Split(out.String(), "data: ")
	require.Len(t, lines, 4)
	require.EqualValues(t, 10, gjson.Get(lines[1], "message.usage.input_tokens").Int())
	require.EqualValues(t, 2, gjson.Get(lines[2], "usage.input_tokens").Int())
	require.EqualValues(t, 8, gjson.Get(lines[2], "usage.cache_read_input_tokens").Int())
	require.Equal(t, "[DONE]\n\n", lines[3])
}

func TestPassthroughFirstContent(t *testing.T) {
	p := New(limit(1), false)
	var out bytes.Buffer
	first := `data: {"choices":[{"message":{"usage":{"prompt_tokens":999},"content":"` + strings.Repeat("a", 1<<20)
	checkWrite(t, p.WriteSSE, &out, []byte(first))
	require.Equal(t, first, out.String())
	checkWrite(t, p.WriteSSE, &out, []byte(`"}}],"usage":{"prompt_tokens":20}}`+"\n\n"))
	require.Contains(t, out.String(), `"usage":{"prompt_tokens":1}`)
}

func TestPendingProviderCost(t *testing.T) {
	got := New(limit(0), false).JSON([]byte(`{"usage":{"prompt_tokens":12,"completion_tokens":2,"total_tokens":14,"cost":4.5,"cost_details":{"upstream_inference_cost":4.5}}}`))
	require.JSONEq(t, `{"usage":{"prompt_tokens":0,"completion_tokens":2,"total_tokens":2}}`, string(got))
}

func TestPartialOpenAIInputDetails(t *testing.T) {
	p := New(limit(6), false)
	_ = p.JSON([]byte(`{"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":4,"cached_tokens_details":{"image_tokens":4},"image_tokens":12,"text_tokens":8}}}`))
	got := p.JSON([]byte(`{"usage":{"output_tokens":9,"total_tokens":29}}`))
	require.JSONEq(t, `{"usage":{"output_tokens":9,"total_tokens":15}}`, string(got))
}

func TestUnchangedUnderCapAndOpaquePayload(t *testing.T) {
	raw := []byte(`{ "id":9007199254740993, "output":[{"usage":{"input_tokens":991}}], "usage":{"input_tokens":12,"input_tokens_details":{"cached_tokens":8},"total_tokens":14} }`)
	p := New(limit(20), false)
	require.Equal(t, raw, p.JSON(raw))
	for _, opaque := range []string{`this is not json`, `{"output":[{"usage":{"input_tokens":123}}]}`, `{"usage":null}`} {
		require.Equal(t, opaque, string(p.JSON([]byte(opaque))))
	}
}

func TestFragmentedUsageLimit(t *testing.T) {
	p := New(limit(1), false)
	var out bytes.Buffer
	checkWrite(t, p.WriteJSON, &out, []byte(`{"content":"ok","usage":{"junk":"`))
	b := bytes.Repeat([]byte("a"), 4096)
	for i := 0; i < 100; i++ {
		checkWrite(t, p.WriteJSON, &out, b)
		require.LessOrEqual(t, len(p.stream.capture), maxFragmentedUsage)
	}
	checkWrite(t, p.WriteJSON, &out, []byte(`","input_tokens":999},"extra":"ok"}`))
	require.JSONEq(t, `{"content":"ok","usage":{},"extra":"ok"}`, out.String())
}

func TestGeminiLateCacheCorrectsModalities(t *testing.T) {
	p := New(limit(5), false)
	_ = p.JSON([]byte(`{"usageMetadata":{"promptTokenCount":12,"promptTokensDetails":[{"modality":"TEXT","tokenCount":8},{"modality":"IMAGE","tokenCount":4}]}}`))
	got := p.JSON([]byte(`{"usageMetadata":{"cachedContentTokenCount":4,"cacheTokensDetails":[{"modality":"IMAGE","tokenCount":4}],"totalTokenCount":19}}`))
	require.JSONEq(t, `{"usageMetadata":{"cachedContentTokenCount":4,"cacheTokensDetails":[{"modality":"IMAGE","tokenCount":4}],"promptTokensDetails":[{"modality":"TEXT","tokenCount":1},{"modality":"IMAGE","tokenCount":4}],"totalTokenCount":12}}`, string(got))
}

func checkWrite(t *testing.T, fn func(io.Writer, []byte) (int, error), dst io.Writer, b []byte) {
	t.Helper()
	n, err := fn(dst, b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
}

type shortSink int

func (s *shortSink) Write(b []byte) (int, error) {
	n := min(int(*s), len(b))
	*s -= shortSink(n)
	if n < len(b) {
		return n, io.ErrClosedPipe
	}
	return n, nil
}
func TestPartialDelivery(t *testing.T) {
	frame := []byte("event: response.completed\ndata: " + `{"response":{"usage":{"input_tokens":12345}}}` + "\n\n")
	p := New(limit(2), false)
	var dst bytes.Buffer
	checkWrite(t, p.WriteSSE, &dst, frame)
	for _, cut := range []int{3, 27, dst.Len() - 1} {
		sink := shortSink(cut)
		n, err := New(limit(2), false).WriteSSE(&sink, frame)
		require.ErrorIs(t, err, io.ErrClosedPipe)
		want := cut
		if cut == dst.Len()-1 {
			want = len(frame) - 1
		}
		require.Equal(t, want, n)
	}
}

func TestMultilineSSEUsage(t *testing.T) {
	raw := "event: message_delta\r\ndata: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":12,\r\ndata: \"output_tokens\":4}}\r\n\r\n"
	p := New(limit(3), true)
	var out bytes.Buffer
	checkWrite(t, p.WriteSSE, &out, []byte(raw))
	var fields []string
	for _, line := range strings.Split(out.String(), "\r\n") {
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			fields = append(fields, data)
		}
	}
	require.JSONEq(t, `{"type":"message_delta","usage":{"input_tokens":3,"output_tokens":4}}`, strings.Join(fields, "\n"))
}

func TestCacheModalityWithoutAggregate(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":20,"input_tokens_details":{"image_tokens":8,"text_tokens":12,"cached_tokens_details":{"image_tokens":5,"text_tokens":3}}}}`)
	got := New(limit(4), false).JSON(raw)
	require.JSONEq(t, `{"usage":{"input_tokens":4,"input_tokens_details":{"image_tokens":4,"text_tokens":0,"cached_tokens_details":{"image_tokens":4,"text_tokens":0}}}}`, string(got))
}
