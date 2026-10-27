//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiUsageProvenanceAllAdapters(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		for _, stream := range []bool{false, true} {
			for _, measured := range []bool{false, true} {
				name := path + "/" + map[bool]string{true: "stream", false: "sync"}[stream] + "/" + map[bool]string{true: "measured zero", false: "input absent"}[measured]
				t.Run(name, func(t *testing.T) {
					payload := `{"candidates":[{"content":{"parts":[{"text":"A complete answer."}]},"finishReason":"STOP"}],"usageMetadata":{"candidatesTokenCount":9}}
`
					if measured {
						payload = strings.Replace(payload, `"candidatesTokenCount":9`, `"promptTokenCount":0,"candidatesTokenCount":9`, 1)
					}
					if stream {
						payload = "data: " + strings.TrimSpace(payload) + "\n\n"
					}
					svc, stub := newGeminiVertexAPIKeyTestService()
					stub.response.Body = io.NopCloser(strings.NewReader(payload))
					if stream {
						stub.response.Header.Set("Content-Type", "text/event-stream")
					}
					c := newGeminiAttemptTestContext(t)
					result, err := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", stream)
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Zero(t, result.Usage.InputTokens)
					expected := UsageTokenSourceUnreported
					if measured {
						expected = UsageTokenSourceUpstream
					}
					require.Equal(t, expected, result.InputTokensSource)
					require.Equal(t, UsageTokenSourceUpstream, result.OutputTokensSource)
					require.Equal(t, UsageTokenSourceUpstream, recordedOutputTokensSource(result))
					require.Equal(t, http.StatusOK, c.Writer.Status())
				})
			}
		}
	}
}
