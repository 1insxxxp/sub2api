package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/wireusage"

// ProjectInputTokenProtectionJSON applies a single turn's customer policy to a
// complete client-bound frame (including a Responses WebSocket terminal event).
// The provider frame remains available verbatim to usage recording.
func ProjectInputTokenProtectionJSON(payload []byte, p *InputTokenProtection) []byte {
	if p == nil {
		return payload
	}
	return wireusage.New(p.Cap, false).JSON(payload)
}
