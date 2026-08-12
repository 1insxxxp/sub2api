package routes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// RegisterGatewayRoutes 注册 API 网关路由（Claude/OpenAI/Gemini 兼容）
func RegisterGatewayRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
) {
	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	textBodyLimit := middleware.RequestBodyLimit(cfg.Gateway.TextMaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()
	compositeTarget := compositeTargetPlatformMiddleware(compositeResolver)
	systemTarget := systemCustomGroupTargetMiddleware(apiKeyService)
	systemGeminiTarget := systemCustomGroupGeminiTargetMiddleware(apiKeyService)
	customTarget := customGroupTargetMiddleware(apiKeyService)
	customGeminiTarget := customGroupGeminiTargetMiddleware(apiKeyService)
	compositeGeminiTarget := compositeGeminiTargetPlatformMiddleware(compositeResolver)

	// 未分组 Key 拦截中间件（按协议格式区分错误响应）
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	requireGroupGoogle := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)

	isOpenAIResponsesCompatibleGatewayPlatform := func(c *gin.Context) bool {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI, service.PlatformGrok:
			return true
		default:
			return false
		}
	}
	isOpenAIGatewayPlatform := func(c *gin.Context) bool {
		return getGroupPlatform(c) == service.PlatformOpenAI
	}
	countTokensHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI:
			h.OpenAIGateway.CountTokens(c)
		case service.PlatformGrok:
			h.OpenAIGateway.GrokCountTokens(c)
		default:
			h.Gateway.CountTokens(c)
		}
	}
	modelsHandler := func(c *gin.Context) {
		if isOpenAIGatewayPlatform(c) && c.Query("client_version") != "" {
			h.OpenAIGateway.CodexModels(c)
			return
		}
		h.Gateway.Models(c)
	}
	isOpenAIOnlyEndpointGatewayPlatform := func(c *gin.Context) bool {
		return getGroupPlatform(c) == service.PlatformOpenAI
	}
	imagesHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI:
			h.OpenAIGateway.Images(c)
		case service.PlatformGrok:
			h.OpenAIGateway.GrokImages(c)
		default:
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Images API is not supported for this platform",
				},
			})
		}
	}
	videoGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoGeneration(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoStatusHandler := func(c *gin.Context) {
		// Video status requests do not carry a model, so composite groups cannot
		// be resolved by compositeTargetPlatformMiddleware. Route them through
		// the Grok handler and let scheduler/account selection enforce capacity.
		if getGroupPlatform(c) == service.PlatformGrok || getGroupPlatform(c) == service.PlatformComposite {
			h.OpenAIGateway.GrokVideoStatus(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoContentHandler := func(c *gin.Context) {
		// Video content requests do not carry a model, so composite groups cannot
		// be resolved by compositeTargetPlatformMiddleware. Route them through
		// the Grok handler just like video status lookups.
		if getGroupPlatform(c) == service.PlatformGrok || getGroupPlatform(c) == service.PlatformComposite {
			h.OpenAIGateway.GrokVideoContent(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoEditHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoEdit(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	videoExtensionHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoExtension(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	// /responses/*subpath 的子路径会被转发到上游同名端点之后，因此在入口就拒掉
	// 不可转发的子路径，不让它进入调度与转发流程。可转发的判定见
	// service.IsForwardableOpenAIResponsesRequestPath 及 upstream_path_guard.go。
	guardResponsesSubpath := func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !service.IsForwardableOpenAIResponsesRequestPath(c) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Unsupported responses subpath",
					},
				})
				return
			}
			next(c)
		}
	}

	// API网关（Claude API兼容）
	gateway := r.Group("/v1")
	gateway.Use(bodyLimit)
	gateway.Use(clientRequestID)
	gateway.Use(opsErrorLogger)
	gateway.Use(endpointNorm)
	gateway.Use(gin.HandlerFunc(apiKeyAuth))
	gateway.GET("/sub2api/billing", h.Gateway.KeyBillingInfo)
	gateway.Use(systemTarget)
	gateway.Use(customTarget)
	gateway.Use(compositeTarget)
	gateway.Use(requireGroupAnthropic)
	{
		// /v1/messages: auto-route based on group platform
		gateway.POST("/messages", func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Messages(c)
				return
			}
			h.Gateway.Messages(c)
		})
		// /v1/messages/count_tokens: OpenAI bridges upstream, Grok estimates
		// locally, and Anthropic-compatible platforms retain their existing path.
		gateway.POST("/messages/count_tokens", countTokensHandler)
		// Codex CLI / Codex app refresh their model picker from the provider's
		// /models endpoint with a client_version query and expect the ChatGPT
		// Codex manifest format; other clients keep the OpenAI-style list.
		gateway.GET("/models", modelsHandler)
		gateway.GET("/usage", h.Gateway.Usage)
		gateway.POST("/live", h.OpenAIGateway.Live)
		gateway.GET("/live/:call_id", h.OpenAIGateway.LiveSideband)
		// OpenAI Responses API: auto-route based on group platform
		gateway.POST("/responses", func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		})
		gateway.POST("/responses/*subpath", guardResponsesSubpath(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		}))
		gateway.POST("/alpha/search", textBodyLimit, h.OpenAIGateway.AlphaSearch)
		gateway.GET("/responses", func(c *gin.Context) {
			h.OpenAIGateway.ResponsesWebSocket(c)
		})
		// OpenAI Chat Completions API: auto-route based on group platform
		gateway.POST("/chat/completions", func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.ChatCompletions(c)
				return
			}
			h.Gateway.ChatCompletions(c)
		})
		gateway.POST("/embeddings", textBodyLimit, func(c *gin.Context) {
			if !isOpenAIOnlyEndpointGatewayPlatform(c) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Embeddings API is not supported for this platform",
					},
				})
				return
			}
			h.OpenAIGateway.Embeddings(c)
		})
		gateway.POST("/images/generations", imagesHandler)
		gateway.POST("/images/edits", imagesHandler)
		gateway.POST("/images/generations/async", h.AsyncImage.Submit)
		gateway.POST("/images/edits/async", h.AsyncImage.Submit)
		gateway.GET("/images/tasks/:task_id", h.AsyncImage.Get)
		gateway.POST("/images/batches", h.BatchImage.Submit)
		gateway.GET("/images/batches", h.BatchImage.List)
		gateway.GET("/images/batches/models", h.BatchImage.Models)
		gateway.GET("/images/batches/:id", h.BatchImage.Get)
		gateway.GET("/images/batches/:id/items", h.BatchImage.Items)
		gateway.GET("/images/batches/:id/items/:custom_id/content", h.BatchImage.ItemContent)
		gateway.GET("/images/batches/:id/download", h.BatchImage.Download)
		gateway.POST("/images/batches/:id/cancel", h.BatchImage.Cancel)
		gateway.DELETE("/images/batches/:id", h.BatchImage.DeleteRecord)
		gateway.DELETE("/images/batches/:id/outputs", h.BatchImage.DeleteOutputs)
		// OpenAI-compatible clients may create through /videos; xAI receives the
		// canonical /videos/generations route inside the Grok media forwarder.
		gateway.POST("/videos", videoGenerationHandler)
		gateway.POST("/videos/generations", videoGenerationHandler)
		gateway.POST("/videos/edits", videoEditHandler)
		gateway.POST("/videos/extensions", videoExtensionHandler)
		gateway.GET("/videos/generations/:request_id/content", videoContentHandler)
		gateway.GET("/videos/edits/:request_id/content", videoContentHandler)
		gateway.GET("/videos/extensions/:request_id/content", videoContentHandler)
		gateway.GET("/videos/generations/:request_id", videoStatusHandler)
		gateway.GET("/videos/edits/:request_id", videoStatusHandler)
		gateway.GET("/videos/extensions/:request_id", videoStatusHandler)
		gateway.GET("/videos/:request_id", videoStatusHandler)
		gateway.GET("/videos/:request_id/content", videoContentHandler)

		// xAI Voice APIs (Grok platform only): HTTP TTS/STT + Realtime WS.
		// Not part of the creation-center product surface — gateway relay only.
		voiceHandler := func(endpoint string) gin.HandlerFunc {
			return func(c *gin.Context) {
				if getGroupPlatform(c) != service.PlatformGrok {
					service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
					c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
					return
				}
				h.OpenAIGateway.GrokVoice(c, endpoint)
			}
		}
		gateway.POST("/tts", voiceHandler("tts"))
		gateway.POST("/stt", voiceHandler("stt"))
		gateway.POST("/custom-voices", voiceHandler("custom-voices"))
		customVoicePathHandler := func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokVoice(c, grokCustomVoiceEndpoint(c))
		}
		gateway.GET("/custom-voices", voiceHandler("custom-voices"))
		gateway.GET("/custom-voices/:voice_id/audio", customVoicePathHandler)
		gateway.GET("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.PATCH("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.DELETE("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.GET("/realtime", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Realtime API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokRealtime(c)
		})
		gateway.POST("/web_search", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Web Search API is not supported for this platform"}})
				return
			}
			h.Gateway.WebSearch(c)
		})
	}

	// Gemini 原生 API 兼容层（Gemini SDK/CLI 直连）
	gemini := r.Group("/v1beta")
	gemini.Use(bodyLimit)
	gemini.Use(clientRequestID)
	gemini.Use(opsErrorLogger)
	gemini.Use(endpointNorm)
	gemini.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	gemini.Use(systemGeminiTarget)
	gemini.Use(customGeminiTarget)
	gemini.Use(compositeGeminiTarget)
	gemini.Use(requireGroupGoogle)
	{
		gemini.GET("/models", h.Gateway.GeminiV1BetaListModels)
		gemini.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		// Gin treats ":" as a param marker, but Gemini uses "{model}:{action}" in the same segment.
		gemini.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

	// OpenAI Responses API（不带v1前缀的别名）— auto-route based on group platform
	responsesHandler := func(c *gin.Context) {
		if isOpenAIResponsesCompatibleGatewayPlatform(c) {
			h.OpenAIGateway.Responses(c)
			return
		}
		h.Gateway.Responses(c)
	}
	r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, responsesHandler)
	r.POST("/responses/*subpath", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, guardResponsesSubpath(responsesHandler))
	r.POST("/alpha/search", textBodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, h.OpenAIGateway.AlphaSearch)
	r.GET("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, func(c *gin.Context) {
		h.OpenAIGateway.ResponsesWebSocket(c)
	})
	r.GET("/models", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, modelsHandler)
	r.POST("/messages/count_tokens", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, countTokensHandler)
	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic)
	{
		codexDirect.POST("/realtime/calls", h.OpenAIGateway.Live)
		codexDirect.GET("/:call_id", h.OpenAIGateway.LiveSideband)
		codexDirect.POST("/responses", responsesHandler)
		codexDirect.POST("/responses/*subpath", guardResponsesSubpath(responsesHandler))
		codexDirect.POST("/alpha/search", textBodyLimit, h.OpenAIGateway.AlphaSearch)
		codexDirect.GET("/responses", func(c *gin.Context) {
			h.OpenAIGateway.ResponsesWebSocket(c)
		})
		codexDirect.GET("/models", h.OpenAIGateway.CodexModels)
	}
	// OpenAI Chat Completions API（不带v1前缀的别名）— auto-route based on group platform
	r.POST("/chat/completions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, func(c *gin.Context) {
		if isOpenAIResponsesCompatibleGatewayPlatform(c) {
			h.OpenAIGateway.ChatCompletions(c)
			return
		}
		h.Gateway.ChatCompletions(c)
	})
	r.POST("/embeddings", textBodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, func(c *gin.Context) {
		if !isOpenAIOnlyEndpointGatewayPlatform(c) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Embeddings API is not supported for this platform",
				},
			})
			return
		}
		h.OpenAIGateway.Embeddings(c)
	})
	r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, imagesHandler)
	r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, imagesHandler)
	r.POST("/images/generations/async", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, h.AsyncImage.Submit)
	r.POST("/images/edits/async", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, h.AsyncImage.Submit)
	r.GET("/images/tasks/:task_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, h.AsyncImage.Get)
	r.POST("/videos/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, videoGenerationHandler)
	r.POST("/videos/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, videoEditHandler)
	r.POST("/videos/extensions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, videoExtensionHandler)
	r.POST("/videos", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, videoGenerationHandler)
	r.GET("/videos/generations/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoContentHandler)
	r.GET("/videos/edits/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoContentHandler)
	r.GET("/videos/extensions/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoContentHandler)
	r.GET("/videos/generations/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoStatusHandler)
	r.GET("/videos/edits/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoStatusHandler)
	r.GET("/videos/extensions/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoStatusHandler)
	r.GET("/videos/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoStatusHandler)
	r.GET("/videos/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, videoContentHandler)

	rootVoiceHandler := func(endpoint string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokVoice(c, endpoint)
		}
	}
	r.POST("/tts", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, rootVoiceHandler("tts"))
	r.POST("/stt", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, rootVoiceHandler("stt"))
	r.POST("/custom-voices", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootVoiceHandler("custom-voices"))
	rootCustomVoicePathHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVoice(c, grokCustomVoiceEndpoint(c))
	}
	r.GET("/custom-voices", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootVoiceHandler("custom-voices"))
	r.GET("/custom-voices/:voice_id/audio", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootCustomVoicePathHandler)
	r.GET("/custom-voices/:voice_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootCustomVoicePathHandler)
	r.PATCH("/custom-voices/:voice_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootCustomVoicePathHandler)
	r.DELETE("/custom-voices/:voice_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, rootCustomVoicePathHandler)
	r.GET("/realtime", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), compositeTarget, requireGroupAnthropic, func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Realtime API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokRealtime(c)
	})
	r.POST("/web_search", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), systemTarget, customTarget, compositeTarget, requireGroupAnthropic, func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Web Search API is not supported for this platform"}})
			return
		}
		h.Gateway.WebSearch(c)
	})

	// Antigravity 模型列表
	r.GET("/antigravity/models", gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.Gateway.AntigravityModels)

	// Antigravity 专用路由（仅使用 antigravity 账户，不混合调度）
	antigravityV1 := r.Group("/antigravity/v1")
	antigravityV1.Use(bodyLimit)
	antigravityV1.Use(clientRequestID)
	antigravityV1.Use(opsErrorLogger)
	antigravityV1.Use(endpointNorm)
	antigravityV1.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1.Use(gin.HandlerFunc(apiKeyAuth))
	antigravityV1.Use(requireGroupAnthropic)
	{
		antigravityV1.POST("/messages", h.Gateway.Messages)
		antigravityV1.POST("/messages/count_tokens", h.Gateway.CountTokens)
		antigravityV1.GET("/models", h.Gateway.AntigravityModels)
		antigravityV1.GET("/usage", h.Gateway.Usage)
	}

	antigravityV1Beta := r.Group("/antigravity/v1beta")
	antigravityV1Beta.Use(bodyLimit)
	antigravityV1Beta.Use(clientRequestID)
	antigravityV1Beta.Use(opsErrorLogger)
	antigravityV1Beta.Use(endpointNorm)
	antigravityV1Beta.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1Beta.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	antigravityV1Beta.Use(requireGroupGoogle)
	{
		antigravityV1Beta.GET("/models", h.Gateway.GeminiV1BetaListModels)
		antigravityV1Beta.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		antigravityV1Beta.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

}

// getGroupPlatform extracts the group platform from the API Key stored in context.
func getGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	if apiKey.Group.Platform == service.PlatformComposite {
		if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
			return platform
		}
	}
	return apiKey.Group.Platform
}

func compositeTargetPlatformMiddleware(resolver *service.CompositeRouteResolver) gin.HandlerFunc {
	if resolver == nil {
		resolver = service.NewCompositeRouteResolver(nil)
	}
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
			c.Next()
			return
		}
		if c.Request == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := http.StatusBadRequest
			message := "Failed to read request body"
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
				message = "Request body is too large"
			}
			c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
			c.Abort()
			return
		}

		model := compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
		if model != "" {
			decision, err := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, compositeRouteEndpointForPath(c.Request.URL.Path))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "server_error", "message": "Failed to resolve composite model route"}})
				c.Abort()
				return
			}
			if decision.Matched {
				c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
				if upstreamModel := strings.TrimSpace(decision.UpstreamModel); upstreamModel != "" && upstreamModel != model && gjson.ValidBytes(body) {
					if rewritten, rewriteErr := sjson.SetBytes(body, "model", upstreamModel); rewriteErr == nil {
						body = rewritten
					}
				}
			}
		}
		resetRequestBody(c, body)
		c.Next()
	}
}

type customGroupModelResolver interface {
	ResolveCustomGroupModel(context.Context, *service.APIKey, string) (*service.CustomGroupModelResolution, error)
}

type systemCustomGroupModelResolver interface {
	ResolveSystemCustomGroupModel(context.Context, *service.APIKey, string) (*service.SystemCustomGroupModelResolution, error)
}

func systemCustomGroupTargetMiddleware(apiKeys systemCustomGroupModelResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSystemCustomRouteGroup() {
			c.Next()
			return
		}
		if c.Request == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := http.StatusBadRequest
			message := "Failed to read request body"
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
				message = "Request body is too large"
			}
			writeSystemCustomRootError(c, status, "invalid_request_error", "INVALID_REQUEST", message)
			return
		}
		model := compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
		if model == "" {
			resetRequestBody(c, body)
			if !systemCustomRequestRequiresModel(c.Request.URL.Path) {
				c.Next()
				return
			}
			writeSystemCustomRootError(c, http.StatusBadRequest, "invalid_request_error", "MODEL_REQUIRED", "Model is required for a system custom subscription group")
			return
		}

		resolution, err := apiKeys.ResolveSystemCustomGroupModel(c.Request.Context(), apiKey, model)
		if err != nil {
			writeSystemCustomResolutionError(c, err, false)
			return
		}
		if !validSystemCustomGroupResolution(resolution, apiKey.Group.ID) {
			writeSystemCustomResolutionError(c, service.ErrSystemCustomGroupSourceUnavailable, false)
			return
		}
		rewrittenBody, err := rewriteCustomGroupRequestModel(c.GetHeader("Content-Type"), body, resolution.PublicModel, resolution.SourceModel)
		if err != nil {
			writeSystemCustomRootError(c, http.StatusBadRequest, "invalid_request_error", "MODEL_REWRITE_FAILED", "Failed to rewrite the system custom model alias")
			return
		}
		ctx := service.WithSystemCustomGroupResolution(c.Request.Context(), resolution.SystemCustomGroupResolution)
		c.Request = c.Request.WithContext(ctx)
		resetRequestBody(c, rewrittenBody)
		if !middleware.SetAPIKeyAndGroupContext(c, resolution.APIKey) {
			writeSystemCustomResolutionError(c, service.ErrSystemCustomGroupSourceUnavailable, false)
			return
		}
		c.Next()
	}
}

func systemCustomGroupGeminiTargetMiddleware(apiKeys systemCustomGroupModelResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSystemCustomRouteGroup() {
			c.Next()
			return
		}
		model := compositeGeminiModelFromParams(c)
		if model == "" { // List/status endpoints without a model are not resolved here.
			c.Next()
			return
		}
		resolution, err := apiKeys.ResolveSystemCustomGroupModel(c.Request.Context(), apiKey, model)
		if err != nil {
			writeSystemCustomResolutionError(c, err, true)
			return
		}
		if !validSystemCustomGroupResolution(resolution, apiKey.Group.ID) {
			writeSystemCustomResolutionError(c, service.ErrSystemCustomGroupSourceUnavailable, true)
			return
		}
		if resolution.SourcePlatform != service.PlatformGemini || resolution.APIKey.Group.Platform != service.PlatformGemini {
			middleware.GoogleErrorWriter(c, http.StatusForbidden, "The selected model is not available through the Gemini API")
			c.Abort()
			return
		}
		for i := range c.Params {
			switch c.Params[i].Key {
			case "model":
				c.Params[i].Value = resolution.SourceModel
			case "modelAction":
				value := strings.TrimPrefix(c.Params[i].Value, "/")
				action := ""
				if idx := strings.LastIndex(value, ":"); idx >= 0 {
					action = value[idx:]
				}
				c.Params[i].Value = "/" + resolution.SourceModel + action
			}
		}
		ctx := service.WithSystemCustomGroupResolution(c.Request.Context(), resolution.SystemCustomGroupResolution)
		c.Request = c.Request.WithContext(ctx)
		if !middleware.SetAPIKeyAndGroupContext(c, resolution.APIKey) {
			writeSystemCustomResolutionError(c, service.ErrSystemCustomGroupSourceUnavailable, true)
			return
		}
		c.Next()
	}
}

func validSystemCustomGroupResolution(resolution *service.SystemCustomGroupModelResolution, billingGroupID int64) bool {
	if resolution == nil || resolution.APIKey == nil || resolution.APIKey.Group == nil || resolution.APIKey.GroupID == nil {
		return false
	}
	return billingGroupID > 0 && resolution.BillingGroupID == billingGroupID && resolution.SourceGroupID > 0 &&
		*resolution.APIKey.GroupID == resolution.SourceGroupID && resolution.APIKey.Group.ID == resolution.SourceGroupID &&
		strings.TrimSpace(resolution.PublicModel) != "" && strings.TrimSpace(resolution.SourceModel) != "" &&
		strings.TrimSpace(resolution.SourcePlatform) != "" && resolution.SourcePlatform == resolution.APIKey.Group.Platform
}

func writeSystemCustomResolutionError(c *gin.Context, err error, google bool) {
	status := infraerrors.Code(err)
	errorType := "server_error"
	code := infraerrors.Reason(err)
	message := "Failed to resolve the system custom model route"
	switch {
	case errors.Is(err, service.ErrSystemCustomGroupModelNotAllowed):
		status = http.StatusForbidden
		errorType = "permission_error"
		code = "SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED"
		message = "The requested model is not enabled for this subscription group"
	case errors.Is(err, service.ErrSystemCustomGroupSourceUnavailable):
		status = http.StatusServiceUnavailable
		errorType = "service_unavailable_error"
		code = "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"
		message = "The selected model source is temporarily unavailable"
	default:
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		if code == "" {
			code = "SYSTEM_CUSTOM_GROUP_RESOLUTION_FAILED"
		}
	}
	if google {
		middleware.GoogleErrorWriter(c, status, message)
		c.Abort()
		return
	}
	writeSystemCustomRootError(c, status, errorType, code, message)
}

func writeSystemCustomRootError(c *gin.Context, status int, errorType, code, message string) {
	if systemCustomUsesAnthropicEnvelope(c.Request.URL.Path) {
		c.AbortWithStatusJSON(status, gin.H{"type": "error", "error": gin.H{"type": errorType, "message": message}})
		return
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": errorType, "message": message, "code": code}})
}

func systemCustomUsesAnthropicEnvelope(path string) bool {
	path = strings.TrimRight(path, "/")
	return path == "/v1/messages" || path == "/v1/messages/count_tokens" || path == "/messages/count_tokens"
}

func systemCustomRequestRequiresModel(path string) bool {
	path = strings.TrimRight(path, "/")
	switch path {
	case "/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens",
		"/v1/live", "/v1/responses", "/responses", "/v1/alpha/search", "/alpha/search",
		"/v1/chat/completions", "/chat/completions", "/v1/embeddings", "/embeddings",
		"/v1/images/generations", "/images/generations", "/v1/images/edits", "/images/edits",
		"/v1/images/generations/async", "/images/generations/async", "/v1/images/edits/async", "/images/edits/async",
		"/v1/videos", "/videos", "/v1/videos/generations", "/videos/generations",
		"/v1/videos/edits", "/videos/edits", "/v1/videos/extensions", "/videos/extensions",
		"/v1/tts", "/tts", "/v1/stt", "/stt", "/v1/web_search", "/web_search",
		"/backend-api/codex/realtime/calls", "/backend-api/codex/responses", "/backend-api/codex/alpha/search":
		return true
	default:
		return false
	}
}

func customGroupTargetMiddleware(apiKeys customGroupModelResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.CustomGroupID == nil {
			c.Next()
			return
		}
		if c.Request == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Failed to read request body"}})
			return
		}
		model := compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
		if model == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Model is required for a custom group key"}})
			return
		}
		resolution, err := apiKeys.ResolveCustomGroupModel(c.Request.Context(), apiKey, model)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "permission_error", "message": "The selected model source group is unavailable"}})
			return
		}
		rewrittenBody, err := rewriteCustomGroupRequestModel(c.GetHeader("Content-Type"), body, resolution.PublicModel, resolution.SourceModel)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Failed to rewrite the custom model alias"}})
			return
		}
		c.Request = c.Request.WithContext(service.WithCustomGroupModelResolution(c.Request.Context(), resolution.PublicModel, resolution.SourceModel))
		resetRequestBody(c, rewrittenBody)
		c.Set(string(middleware.ContextKeyAPIKey), resolution.APIKey)
		c.Next()
	}
}

func rewriteCustomGroupRequestModel(contentType string, body []byte, publicModel, sourceModel string) ([]byte, error) {
	publicModel = strings.TrimSpace(publicModel)
	sourceModel = strings.TrimSpace(sourceModel)
	if sourceModel == "" {
		return nil, errors.New("custom group source model is empty")
	}
	if publicModel == sourceModel {
		return body, nil
	}
	if gjson.ValidBytes(body) {
		rewritten, err := sjson.SetBytes(body, "model", sourceModel)
		if err != nil {
			return nil, err
		}
		return rewritten, nil
	}
	return rewriteMultipartModel(contentType, body, sourceModel)
}

func rewriteMultipartModel(contentType string, body []byte, sourceModel string) ([]byte, error) {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") || strings.TrimSpace(params["boundary"]) == "" {
		return nil, errors.New("custom group alias body is neither JSON nor multipart")
	}
	boundary := strings.TrimSpace(params["boundary"])
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var output bytes.Buffer
	writer := multipart.NewWriter(&output)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, err
	}
	foundModel := false
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		destination, createErr := writer.CreatePart(part.Header)
		if createErr != nil {
			return nil, createErr
		}
		if part.FormName() == "model" && part.FileName() == "" {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return nil, err
			}
			if _, err := io.WriteString(destination, sourceModel); err != nil {
				return nil, err
			}
			foundModel = true
			continue
		}
		if _, err := io.Copy(destination, part); err != nil {
			return nil, err
		}
	}
	if !foundModel {
		return nil, errors.New("multipart model field is missing")
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func customGroupGeminiTargetMiddleware(apiKeys customGroupModelResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.CustomGroupID == nil {
			c.Next()
			return
		}
		model := compositeGeminiModelFromParams(c)
		if model == "" { // GET /v1beta/models is handled by the list handler.
			c.Next()
			return
		}
		resolution, err := apiKeys.ResolveCustomGroupModel(c.Request.Context(), apiKey, model)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": http.StatusForbidden, "message": "The selected model source group is unavailable", "status": "PERMISSION_DENIED"}})
			return
		}
		if resolution.APIKey == nil || resolution.APIKey.Group == nil || resolution.APIKey.Group.Platform != service.PlatformGemini {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": http.StatusForbidden, "message": "The selected model is not available through the Gemini API", "status": "PERMISSION_DENIED"}})
			return
		}
		for i := range c.Params {
			switch c.Params[i].Key {
			case "model":
				c.Params[i].Value = resolution.SourceModel
			case "modelAction":
				value := strings.TrimPrefix(c.Params[i].Value, "/")
				action := ""
				if idx := strings.LastIndex(value, ":"); idx >= 0 {
					action = value[idx:]
				}
				c.Params[i].Value = "/" + resolution.SourceModel + action
			}
		}
		c.Request = c.Request.WithContext(service.WithCustomGroupModelResolution(c.Request.Context(), resolution.PublicModel, resolution.SourceModel))
		c.Set(string(middleware.ContextKeyAPIKey), resolution.APIKey)
		c.Next()
	}
}

func compositeRequestModelFromBody(contentType string, body []byte) string {
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); model != "" {
		return model
	}
	return compositeMultipartModelFromBody(contentType, body)
}

func compositeMultipartModelFromBody(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return ""
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return ""
		}
		if err != nil {
			return ""
		}
		if part.FormName() != "model" || part.FileName() != "" {
			continue
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func compositeGeminiTargetPlatformMiddleware(resolver *service.CompositeRouteResolver) gin.HandlerFunc {
	if resolver == nil {
		resolver = service.NewCompositeRouteResolver(nil)
	}
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if ok && apiKey != nil && apiKey.Group != nil && apiKey.Group.Platform == service.PlatformComposite {
			model := compositeGeminiModelFromParams(c)
			if model != "" {
				decision, err := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, service.CompositeRouteEndpointGemini)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "server_error", "message": "Failed to resolve composite model route"}})
					c.Abort()
					return
				}
				if decision.Matched {
					c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
				}
			}
			if _, resolved := service.ResolvedTargetPlatformFromContext(c.Request.Context()); !resolved {
				c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformGemini))
			}
		}
		c.Next()
	}
}

// grokCustomVoiceEndpoint derives the upstream Voice endpoint for the
// /custom-voices/:voice_id[/audio] routes.
//
// The /audio suffix must be decided from the matched route template, not from
// the raw URL path: a voice literally named "audio" makes GET
// /custom-voices/audio match /custom-voices/:voice_id, and a raw-path suffix
// check would rewrite it to custom-voices/audio/audio — turning a profile
// lookup into an audio download.
func grokCustomVoiceEndpoint(c *gin.Context) string {
	endpoint := "custom-voices/" + c.Param("voice_id")
	if strings.HasSuffix(c.FullPath(), "/:voice_id/audio") {
		endpoint += "/audio"
	}
	return endpoint
}

func compositeGeminiModelFromParams(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return model
	}
	modelAction := strings.TrimPrefix(strings.TrimSpace(c.Param("modelAction")), "/")
	if modelAction == "" {
		return ""
	}
	if idx := strings.LastIndex(modelAction, ":"); idx >= 0 {
		return strings.TrimSpace(modelAction[:idx])
	}
	return modelAction
}

func resetRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func compositeRouteEndpointForPath(path string) string {
	switch {
	case strings.Contains(path, "/messages/count_tokens"):
		return service.CompositeRouteEndpointCountTokens
	case strings.Contains(path, "/messages"):
		return service.CompositeRouteEndpointMessages
	case strings.Contains(path, "/responses"):
		return service.CompositeRouteEndpointResponses
	case strings.Contains(path, "/chat/completions"):
		return service.CompositeRouteEndpointChatCompletions
	case strings.Contains(path, "/embeddings"):
		return service.CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/"):
		return service.CompositeRouteEndpointImages
	case strings.Contains(path, "/v1beta/"):
		return service.CompositeRouteEndpointGemini
	default:
		return service.CompositeRouteEndpointAny
	}
}
