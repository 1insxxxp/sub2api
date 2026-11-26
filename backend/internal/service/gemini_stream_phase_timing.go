package service

import (
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// geminiStreamPhaseTiming belongs to one forwarding attempt. It keeps no
// content and leaves unobserved phases nil.
type geminiStreamPhaseTiming struct {
	start             time.Time
	firstEvent        *int
	firstReasoning    *int
	firstText         *int
	downstreamContent *int
}

func (t *geminiStreamPhaseTiming) mark(target **int) {
	if *target == nil {
		elapsed := int(time.Since(t.start).Milliseconds())
		*target = &elapsed
	}
}

func (t *geminiStreamPhaseTiming) observeUpstream(raw []byte) {
	if !gjson.ValidBytes(raw) || !gjson.ParseBytes(raw).IsObject() {
		return
	}
	t.mark(&t.firstEvent)
	for _, candidate := range gjson.GetBytes(raw, "candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			text := part.Get("text")
			if text.Type != gjson.String || strings.TrimSpace(text.String()) == "" {
				continue
			}
			if part.Get("thought").Bool() {
				t.mark(&t.firstReasoning)
			} else {
				t.mark(&t.firstText)
			}
		}
	}
}

func (t *geminiStreamPhaseTiming) apply(outcome *ResponseOutcome) {
	outcome.UpstreamFirstEventMs = t.firstEvent
	outcome.UpstreamFirstReasoningMs = t.firstReasoning
	outcome.UpstreamFirstTextMs = t.firstText
	outcome.DownstreamFirstContentMs = t.downstreamContent
}
