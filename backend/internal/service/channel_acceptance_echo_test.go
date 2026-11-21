package service

import "testing"

func TestAcceptanceEchoLocatesChangedText(t *testing.T) {
	sample := AcceptanceSample{Category: "repeat", Protocol: "chat", Expected: "A7"}
	for _, test := range []struct {
		origin, returned, code string
		complete               bool
	}{
		{"A7", "A7", "", true}, {"A7", "answer A7", "client_challenge_text_changed", true}, {"A7", "", "client_challenge_text_changed", true},
		{"answer A7", "answer A7", "upstream_challenge_extra_text", true}, {"A7", "partial", "", false},
	} {
		r := new(AcceptanceResult)
		acceptanceCheckEcho(sample, r, &acceptanceEvidenceParser{completed: true, excerpt: test.origin}, &acceptanceEvidenceParser{completed: test.complete, excerpt: test.returned})
		if test.code == "" && len(r.Findings) != 0 || test.code != "" && (len(r.Findings) != 1 || r.Findings[0].Code != test.code) {
			t.Fatal(test, r.Findings)
		}
	}
}
