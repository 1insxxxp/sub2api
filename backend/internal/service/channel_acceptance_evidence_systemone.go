package service

import (
	"sort"
	"strings"
)

// A System One choice response is one complete JSON document. Only answer
// values are visible output; question IDs and provider extensions can echo input.
func (p *acceptanceEvidenceParser) consumeSystemOne(m map[string]any) {
	answers := acceptanceObject(m["answers"])
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	p.responseModel = acceptanceString(m["model"])
	p.completed = len(keys) != 0 && m["error"] == nil
	for _, key := range keys {
		a := acceptanceObject(answers[key])
		value := acceptanceString(a["choice"])
		if acceptanceString(a["type"]) != "choice" || strings.TrimSpace(value) == "" {
			p.completed = false
			continue
		}
		if p.hasText {
			p.text("\n", false)
		}
		p.text(value, false)
	}
	if p.completed {
		p.finishReason = "answered"
	}
	p.captureUsage(acceptanceObject(m["usage"]), "upstream_systemone_reported")
}
