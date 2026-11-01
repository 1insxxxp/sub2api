package claude

import (
	"regexp"
	"strings"
)

var outputModelVersionSuffix = regexp.MustCompile(`(?:-v[0-9]+(?::[0-9]+)?|[-@][0-9]{8})$`)

// ModelMaxOutputTokens returns the published output capability of a known
// Claude model identity. It only normalizes recognized provider decorations
// for this lookup; callers must keep the actual upstream model unchanged.
// Unknown models and unrecognized suffixes have no inferred output default.
func ModelMaxOutputTokens(model string) (int, bool) {
	model = normalizeOutputCapabilityModel(model)
	switch model {
	case "claude-fable-5-1", "claude-fable-5",
		"claude-opus-5-5", "claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6",
		"claude-sonnet-5-5", "claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-5-5":
		return 128000, true
	case "claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-4-5":
		return 64000, true
	default:
		return 0, false
	}
}

func normalizeOutputCapabilityModel(model string) string {
	id := strings.ToLower(strings.TrimSpace(model))
	for {
		previous := id
		for _, prefix := range []string{"[次cursor]", "models/", "a/", "anthropic/", "us.", "eu.", "apac.", "jp.", "au.", "us-gov.", "global.", "anthropic.", "anti-"} {
			id = strings.TrimSpace(strings.TrimPrefix(id, prefix))
		}
		for _, suffix := range []string{"[次cursor]", "-thinking", "-cu"} {
			id = strings.TrimSpace(strings.TrimSuffix(id, suffix))
		}
		id = outputModelVersionSuffix.ReplaceAllString(id, "")
		if previous == id {
			break
		}
	}
	// Dotted minor versions are provider spellings of the same model version.
	// Exact matching below deliberately rejects private or unknown families.
	return strings.ReplaceAll(id, ".", "-")
}
