package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Validate admin writes separately from the permissive legacy read default.
// This configuration only controls model status display, never request access.
func normalizeGroupModelStatusVisibility(cfg GroupModelStatusVisibility) (GroupModelStatusVisibility, error) {
	out := cfg.Normalize()
	if out.Enabled && len(out.Models) == 0 {
		return out, infraerrors.BadRequest("INVALID_MODEL_STATUS_VISIBILITY", "select at least one model for model status visibility, or disable the display restriction")
	}
	for _, model := range out.Models {
		if strings.Contains(strings.TrimSuffix(model, "*"), "*") {
			return out, infraerrors.BadRequest("INVALID_MODEL_STATUS_VISIBILITY", `wildcard "*" is only allowed at the end of a model status visibility entry`)
		}
	}
	return out, nil
}
