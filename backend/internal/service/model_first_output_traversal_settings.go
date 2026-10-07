package service

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// StartModelFirstOutputTraversal bounds Gemini text account selection, queueing,
// authentication and attempts with one shared budget. Per-account first-output
// guards stop it when the response starts; normal output has no such deadline.
func StartModelFirstOutputTraversal(c *gin.Context, settings *SettingService, platform, model string, stream bool) func() {
	if c == nil || c.Request == nil || settings == nil || platform != PlatformGemini || !stream || isImageGenerationModel(model) {
		return func() {}
	}
	if modelFirstOutputTraversalFromContext(c.Request.Context()) != nil {
		return func() {}
	}
	policy := modelFirstOutputRequestPolicy(c.Request.Context(), c, settings, platform, model)
	// Keep every account attempt consistent with the budget created here, even
	// if settings change or become unavailable while this request is waiting.
	c.Set(modelFirstOutputRequestPolicyKey, policy)
	if !modelFirstOutputTimeoutEnabled(c.Request.Context(), policy) || policy.HardCapSeconds <= 0 {
		return func() {}
	}
	start := ensureModelFirstOutputBudget(c, time.Now())
	modelFirstOutputAttemptDeadline(c, policy, start)
	deadline := start.Add(time.Duration(policy.HardCapSeconds) * time.Second)
	if value, ok := c.Get(string(modelFirstOutputBudgetDeadlineKey)); ok {
		if fixedDeadline, ok := value.(time.Time); ok && fixedDeadline.Before(deadline) {
			deadline = fixedDeadline
		}
	}
	return startModelFirstOutputTraversal(c, deadline)
}

const modelFirstOutputRequestPolicyKey = "sub2api.model_first_output_request_policy"

func modelFirstOutputRequestPolicy(ctx context.Context, c *gin.Context, settings *SettingService, platform, model string) ModelFirstOutputTimeoutPolicy {
	// The snapshot belongs to the incoming Gemini request, including attempts
	// served by native Antigravity accounts through the Messages/native routes.
	if c != nil {
		if value, ok := c.Get(modelFirstOutputRequestPolicyKey); ok {
			if policy, valid := value.(ModelFirstOutputTimeoutPolicy); valid {
				return policy
			}
		}
	}
	if settings == nil {
		return ModelFirstOutputTimeoutPolicy{}
	}
	return settings.ResolveModelFirstOutputTimeout(ctx, platform, model)
}
