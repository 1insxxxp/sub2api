//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcceptanceFingerprintKiroEventStream(t *testing.T) {
	headers := map[string]string{":message-type": "event", ":event-type": "assistantResponseEvent"}
	frame := buildBedrockEventStreamFrame(headers, []byte(`{"content":"private-canary"}`))
	meter := buildBedrockEventStreamFrame(map[string]string{":message-type": "event", ":event-type": "meteringEvent"}, []byte(`{"usage":0.25,"unit":"credit"}`))
	combined := append(append([]byte(nil), frame...), meter...)
	invalid := append([]byte(nil), frame...)
	invalid[len(invalid)-1] ^= 1
	for _, tc := range []struct {
		name, media string
		body        []byte
		compatible  bool
	}{
		{"native frame", "application/vnd.amazon.eventstream; charset=binary", frame, true},
		{"two independent events", "application/vnd.amazon.eventstream", combined, true},
		{"corrupt", "application/vnd.amazon.eventstream", invalid, false},
		{"incomplete", "application/vnd.amazon.eventstream", frame[:len(frame)-1], false},
		{"oversized length", "application/vnd.amazon.eventstream", []byte{255, 255, 255, 255}, false},
		{"wrong media", "application/octet-stream", frame, false},
		{"bedrock chunk", "application/vnd.amazon.eventstream", bedrockChunkFrame(`{"type":"assistantResponseEvent","content":"private-canary"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser := newAcceptanceEvidenceParser()
			parser.fingerprint = newAcceptanceFingerprint()
			base := &acceptanceExecutorHTTP{status: 200, contentType: tc.media, body: string(tc.body)}
			u := &acceptanceHTTPUpstream{accountID: 7, base: base, parser: parser, result: &AcceptanceResult{EffectiveParameters: map[string]any{}}}
			r := httptest.NewRequest(http.MethodPost, "https://relay.example/v1/messages", nil)
			resp, err := u.Do(r, "", 7, 1)
			require.NoError(t, err)
			var got strings.Builder
			var b [1]byte
			for {
				n, err := resp.Body.Read(b[:])
				got.Write(b[:n])
				if err != nil {
					break
				}
			}
			require.NoError(t, resp.Body.Close())
			require.Equal(t, tc.body, []byte(got.String()))
			f := parser.snapshot().fingerprint
			require.Equal(t, tc.compatible, f.compare("kiro", f.kiroEventKinds()) == "source_compatible")
			result := &AcceptanceResult{Status: "passed", EffectiveParameters: map[string]any{}}
			f.report("claude_max", result)
			require.Equal(t, tc.name == "two independent events", result.Findings[0].Code == "source_mismatch")
			require.Equal(t, 1, base.calls)
		})
	}
}
