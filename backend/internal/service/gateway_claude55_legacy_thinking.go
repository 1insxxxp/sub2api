package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (s *GatewayService) normalizeClaude55LegacyRequest(ctx context.Context, parsed *ParsedRequest, model string) (bool, error) {
	original := parsed.Body.Bytes()
	body, changed := normalizeClaude55LegacyThinking(original, model)
	if !changed {
		return false, nil
	}
	// Explicit effort has already passed the ingress policy. Only an effort
	// newly derived from a legacy enabled budget needs its first policy pass;
	// applying mappings twice could turn a permitted value into another tier.
	if gjson.GetBytes(original, "thinking.type").String() == "enabled" &&
		!gjson.GetBytes(original, "output_config.effort").Exists() &&
		gjson.GetBytes(body, "output_config.effort").Exists() && parsed.GroupID != nil && *parsed.GroupID > 0 {
		group := s.groupForReasoningEffort(ctx, parsed.GroupID)
		if group == nil {
			return false, fmt.Errorf("cannot resolve group for legacy thinking effort policy")
		}
		maxEffort := group.MaxReasoningEffort
		mappings := append([]ReasoningEffortMapping(nil), group.ReasoningEffortMappings...)
		// Match ingress handling of composite/OpenAI minimal on Claude.
		if NormalizeMaxReasoningEffort(maxEffort) == "minimal" {
			maxEffort = "low"
		}
		for i := range mappings {
			if NormalizeMaxReasoningEffort(mappings[i].To) == "minimal" {
				mappings[i].To = "low"
			}
		}
		// Ingress policy matches the public model, before group/channel
		// rewrites. Evaluate only the new effort against that same identity;
		// never substitute the forwarding model or copy the message payload.
		policyBody, err := json.Marshal(map[string]any{
			"model":         gatewayPublicModel(ctx, parsed.Model),
			"output_config": map[string]string{"effort": gjson.GetBytes(body, "output_config.effort").String()},
		})
		if err != nil {
			return false, err
		}
		policyBody, _, err = ApplyReasoningEffortPolicy(policyBody, maxEffort, mappings, group.MaxReasoningEffortOverLimit)
		if err != nil {
			return false, err
		}
		body, err = sjson.SetRawBytes(body, "output_config.effort", []byte(gjson.GetBytes(policyBody, "output_config.effort").Raw))
		if err != nil {
			return false, err
		}
	}
	if err := parsed.ReplaceBody(body); err != nil {
		return false, fmt.Errorf("normalize legacy Claude thinking: %w", err)
	}
	return true, nil
}

// normalizeClaude55LegacyThinking adapts old Messages clients to the selected
// upstream model. It only touches legacy modes; native adaptive/between_tools
// requests still undergo strict validation. It must run on each attempt's body,
// after resolving account mappings and before validation or OAuth mimicry.
//
// Opus 5.5 cannot disable thinking. A legacy disabled request therefore uses
// low effort and omitted summaries, not a promise of zero thinking or cost.
// Never increase max_tokens or rewrite signed history/cache breakpoints here.
func normalizeClaude55LegacyThinking(body []byte, model string) ([]byte, bool) {
	if !claude.IsOpus55(model) && !claude.IsSonnet55(model) {
		return body, false
	}
	thinking := gjson.GetBytes(body, "thinking")
	typ := thinking.Get("type").String()
	if !thinking.IsObject() || (typ != "enabled" && typ != "disabled") || !gjson.ValidBytes(body) {
		return body, false
	}
	modified := body
	var err error
	if typ == "enabled" {
		budget := thinking.Get("budget_tokens")
		if budget.Exists() && (budget.Type != gjson.Number || budget.Float() <= 0 || budget.Float() != float64(budget.Int())) {
			// Do not silently repair malformed client values.
			return body, false
		}
		modified, err = sjson.SetBytes(modified, "thinking.type", "adaptive")
		if err == nil {
			modified, err = sjson.DeleteBytes(modified, "thinking.budget_tokens")
		}
		if err == nil && budget.Exists() && !gjson.GetBytes(body, "output_config.effort").Exists() {
			// Approximate the existing group budget tiers; adaptive effort is
			// not an exact thinking budget. The total output cap is untouched.
			effort := "max"
			switch {
			case budget.Int() <= 1024:
				effort = "low"
			case budget.Int() <= 4096:
				effort = "medium"
			case budget.Int() <= 10240:
				effort = "high"
			}
			modified, err = sjson.SetBytes(modified, "output_config.effort", effort)
		}
	} else if claude.IsSonnet55(model) {
		// between_tools accepts no display/budget/block_binding subfields.
		modified, err = sjson.SetRawBytes(modified, "thinking", []byte(`{"type":"between_tools"}`))
		effort := gjson.GetBytes(body, "output_config.effort").String()
		if err == nil && (effort == "xhigh" || effort == "max") {
			modified, err = sjson.SetBytes(modified, "output_config.effort", "high")
		}
	} else {
		modified, err = sjson.SetRawBytes(modified, "thinking", []byte(`{"type":"adaptive","display":"omitted"}`))
		if err == nil {
			modified, err = sjson.SetBytes(modified, "output_config.effort", "low")
		}
	}
	if err != nil {
		return body, false
	}
	return modified, true
}
