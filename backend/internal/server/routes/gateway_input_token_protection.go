package routes

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Store only scalar evidence between authentication and group/model resolution.
// Neither retries nor asynchronous settlement need a copy of the prompt.
type originalInputEvidence struct {
	protection     *service.InputTokenProtection
	model          string
	useSourceGroup bool
}

const originalInputEvidenceKey = "gateway.original_customer_input_evidence"

func inputProtectionImageRoute(path string) bool {
	return strings.HasSuffix(path, "/images/generations") || strings.HasSuffix(path, "/images/edits")
}

func inputProtectionRoute(path string) bool {
	if inputProtectionImageRoute(path) {
		return true
	}
	for _, suffix := range []string{"/chat/completions", "/messages", "/responses", "/responses/compact", "/embeddings", "/systemone", "/v1beta/cachedContents"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return strings.Contains(path, "/models/") && (strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent"))
}

func captureOriginalCustomerInput(c *gin.Context) {
	if c.Request.Method != http.MethodPost || !inputProtectionRoute(c.Request.URL.Path) {
		return
	}
	key, _ := middleware.GetAPIKeyFromContext(c)
	if key == nil {
		return
	}
	// Personal model aliases and category defaults settle against a resolved
	// source group. A system custom subscription settles against its container.
	source := key.CustomGroupID != nil && !key.Group.IsSystemCustomRouteGroup() || key.Group != nil && key.Group.IsCategoryDefault
	if !source && (key.Group == nil || !key.Group.GeminiInputCountingEnabled) {
		return
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if !middleware.RejectRequestBodyReadError(c, "middleware.input_token_protection", err) {
			middleware.AbortWithRequestBodyReadFailure(c, "middleware.input_token_protection", err)
		}
		return
	}
	requestmodel.ResetRequestBody(c.Request, body)
	model := compositeGeminiModelFromParams(c)
	if model == "" {
		model = compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
	}
	if model == "" && inputProtectionImageRoute(c.Request.URL.Path) {
		model = "gpt-image-2"
	}
	if c.Request.URL.Path == "/v1beta/cachedContents" {
		model = strings.TrimPrefix(model, "models/")
	}
	c.Set(originalInputEvidenceKey, originalInputEvidence{
		protection: service.EstimateCustomerInputTokenProtection(c.Request.URL.Path, body),
		model:      model, useSourceGroup: source,
	})
}

func bindCustomerInputProtection(billsByTokens func(context.Context, *service.APIKey, string, service.InputTokenProtectionAttempt, bool) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, _ := c.Get(originalInputEvidenceKey)
		original, ok := value.(originalInputEvidence)
		if !ok {
			return
		}
		key, _ := middleware.GetAPIKeyFromContext(c)
		if key == nil || key.Group == nil || (original.useSourceGroup && !key.Group.GeminiInputCountingEnabled) {
			return
		}
		c.Request = c.Request.WithContext(service.WithInputTokenProtection(c.Request.Context(), original.protection))
		if c.Request.URL.Path == "/v1beta/cachedContents" {
			// usageMetadata describes the stored resource and its storage charge.
			return
		}
		imageEndpoint := inputProtectionImageRoute(c.Request.URL.Path)
		var pricedKey *service.APIKey
		var lastAttempt service.InputTokenProtectionAttempt
		var metered bool
		middleware.InstallInputTokenProtectionWriter(c, func() bool {
			attempt := service.InputTokenProtectionHTTPAttempt(c)
			pricingKey := key
			if value, ok := c.Get(middleware.InputProtectionPricingKey); ok {
				if retryKey, ok := value.(*service.APIKey); ok && retryKey != nil {
					pricingKey = retryKey
				}
			}
			if pricedKey != pricingKey || attempt != lastAttempt {
				metered = billsByTokens(c.Request.Context(), pricingKey, original.model, attempt, imageEndpoint)
				lastAttempt, pricedKey = attempt, pricingKey
			}
			return metered
		})
	}
}
