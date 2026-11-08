//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestModelProbeForwardRejectsMappedDedicatedModesBeforeUpstream(t *testing.T) {
	for _, model := range []string{"text-embedding-3-small", "gemini-embedding-001", "omni-moderation-latest", "tts-1", "whisper-1", "gpt-realtime", "gpt-4o-audio-preview", "grok-voice", "grok-web-search", "dall-e-3", "imagen-4", "veo-3", "nai-diffusion-4", "sora-2"} {
		t.Run(model, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			group := &service.Group{ID: 1, Platform: service.PlatformOpenAI}
			route := &modelProbeRoute{ctx: context.Background(), billingGroup: group, group: group, publicModel: "public-alias", routeModel: "public-alias", forwardModel: "channel-alias", platform: service.PlatformOpenAI, endpoint: service.CompositeRouteEndpointAny}
			account := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"channel-alias": model}}}
			result, err := (&GatewayHandler{}).forwardModelProbe(c, route, account)
			require.ErrorIs(t, err, service.ErrModelProbeUnsupported)
			require.Nil(t, result)
		})
	}
}

// No network fallback: these tests run the production adapters against a
// transport that can only return the configured response.
type modelProbeForwardHTTP struct {
	requests []*http.Request
	bodies   [][]byte
	respond  func(*http.Request) (*http.Response, error)
}

func (u *modelProbeForwardHTTP) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	u.requests = append(u.requests, req)
	u.bodies = append(u.bodies, body)
	return u.respond(req)
}

func (u *modelProbeForwardHTTP) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func modelProbeHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func modelProbeForwardFixture(t *testing.T, platform, model, endpoint string, u *modelProbeForwardHTTP, accounting ...*modelProbeAccountingGuard) (*GatewayHandler, *gin.Context, *modelProbeRoute, *service.Account) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	guard := &modelProbeAccountingGuard{}
	if len(accounting) > 0 {
		guard = accounting[0]
		// Exercise normal production mode; simple mode must not be what prevents
		// a regression from entering the accounting path.
		cfg.RunMode = config.RunModeStandard
	}
	t.Cleanup(func() {
		require.Zero(t, guard.usageWrites.Load(), "model probes must not persist customer usage")
		require.Zero(t, guard.billingWrites.Load(), "model probes must not mutate customer billing")
	})
	group := &service.Group{ID: 9001, Platform: platform, Hydrated: true, Status: service.StatusActive, AllowImageGeneration: true}
	account := &service.Account{ID: 9002, Platform: platform, Type: service.AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "fixture-only", "base_url": "https://probe.example.invalid", "pool_mode": true, "model_mapping": map[string]any{"channel-alias": model, model: "must-not-map-twice"},
	}, Extra: map[string]any{"openai_responses_supported": false, "anthropic_passthrough": true}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	require.True(t, middleware.SetAPIKeyAndGroupContext(c, &service.APIKey{Group: group, GroupID: &group.ID}))
	route := &modelProbeRoute{ctx: c.Request.Context(), group: group, billingGroup: group, publicModel: "public-alias", routeModel: "group-alias", forwardModel: "channel-alias", platform: platform, endpoint: endpoint}
	gemini := service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, u, nil, cfg)
	openAI := service.NewOpenAIGatewayService(nil, guard, guard, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, u, nil, nil, nil, nil, nil, nil, nil, nil)
	openAI.SetGeminiChatCompletionsForwarder(gemini)
	gateway := service.NewGatewayService(nil, nil, guard, guard, nil, nil, nil, nil, cfg, nil, nil, nil, &service.RateLimitService{}, nil, nil, u, nil, nil, nil, nil, nil, nil, &service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil)
	return &GatewayHandler{gatewayService: gateway, openAIGatewayService: openAI, geminiCompatService: gemini}, c, route, account
}

// Both repositories are non-nil dependencies of the real Gateway/OpenAI
// services. Unexpected writes are recorded even if an adapter swallows their
// error; other repository operations panic through the embedded nil interfaces.
// Gemini forwarding itself has no usage/billing repository dependency: any
// customer accounting would have to go through these Gateway dependencies.
type modelProbeAccountingGuard struct {
	service.UsageLogRepository
	service.UsageBillingRepository
	usageWrites   atomic.Int64
	billingWrites atomic.Int64
}

func (r *modelProbeAccountingGuard) Create(context.Context, *service.UsageLog) (bool, error) {
	r.usageWrites.Add(1)
	return false, errors.New("unexpected model probe usage write")
}

func (r *modelProbeAccountingGuard) CreateBestEffort(context.Context, *service.UsageLog) error {
	r.usageWrites.Add(1)
	return errors.New("unexpected model probe best-effort usage write")
}

func (r *modelProbeAccountingGuard) Apply(context.Context, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.billingWrites.Add(1)
	return nil, errors.New("unexpected model probe billing write")
}

func (r *modelProbeAccountingGuard) ReserveBatchImageBalance(context.Context, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	r.billingWrites.Add(1)
	return nil, errors.New("unexpected model probe balance reservation")
}

func (r *modelProbeAccountingGuard) CaptureBatchImageBalance(context.Context, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	r.billingWrites.Add(1)
	return nil, errors.New("unexpected model probe balance capture")
}

func (r *modelProbeAccountingGuard) ReleaseBatchImageBalance(context.Context, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	r.billingWrites.Add(1)
	return nil, errors.New("unexpected model probe balance release")
}

type modelProbeInterruptedBody struct{}

func (modelProbeInterruptedBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestModelProbeForwardNeverWritesCustomerAccounting(t *testing.T) {
	for _, tc := range []struct {
		platform, model, prefix, terminal string
	}{
		{
			platform: service.PlatformGemini, model: "gemini-2.5-flash",
			prefix:   "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"OK\"}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":1,\"totalTokenCount\":11}}\n\n",
			terminal: "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":1,\"totalTokenCount\":11}}\n\n",
		},
		{
			platform: service.PlatformOpenAI, model: "gpt-4.1-mini",
			prefix:   "data: {\"id\":\"probe-chat\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4.1-mini\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":null}]}\n\n",
			terminal: "data: {\"id\":\"probe-chat\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":1,\"total_tokens\":11}}\n\ndata: [DONE]\n\n",
		},
		{
			platform: service.PlatformAnthropic, model: "claude-sonnet-4-5",
			prefix:   "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"probe-message\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n",
			terminal: "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		},
	} {
		for _, scenario := range []string{"success_with_usage", "upstream_failure", "interrupted_after_output"} {
			t.Run(tc.platform+"/"+scenario, func(t *testing.T) {
				guard := &modelProbeAccountingGuard{}
				u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
					if scenario == "upstream_failure" {
						return modelProbeHTTPResponse(503, `{"error":{"type":"overloaded_error","message":"unavailable"}}`), nil
					}
					resp := modelProbeHTTPResponse(200, tc.prefix+tc.terminal)
					resp.Header.Set("Content-Type", "text/event-stream")
					if scenario == "interrupted_after_output" {
						resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(tc.prefix), modelProbeInterruptedBody{}))
					}
					return resp, nil
				}}
				h, c, route, account := modelProbeForwardFixture(t, tc.platform, tc.model, service.CompositeRouteEndpointAny, u, guard)
				result, err := h.forwardModelProbe(c, route, account)
				switch scenario {
				case "success_with_usage":
					require.NoError(t, err)
					require.NotNil(t, result)
					require.NotNil(t, result.Outcome)
					require.True(t, result.Outcome.StreamCompleted)
					require.True(t, result.Outcome.HasEffectiveOutput())
				case "upstream_failure":
					require.Error(t, err)
				case "interrupted_after_output":
					require.True(t, err != nil || result == nil || result.Outcome == nil || !result.Outcome.StreamCompleted)
				}
				require.Len(t, u.requests, 1)
				require.Zero(t, guard.usageWrites.Load())
				require.Zero(t, guard.billingWrites.Load())
			})
		}
	}
}

func TestModelProbeForwardGeminiImageUsesNativeImageModalities(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
		return modelProbeHTTPResponse(200, `{"candidates":[{"content":{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":12,"totalTokenCount":22}}`), nil
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGemini, "gemini-2.5-flash-image", service.CompositeRouteEndpointAny, u)
	result, err := h.forwardModelProbe(c, route, account)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.HasMedia)
	require.True(t, result.Outcome.StreamCompleted)
	require.Len(t, u.requests, 1)
	require.Equal(t, "/v1beta/models/gemini-2.5-flash-image:generateContent", u.requests[0].URL.Path)
	require.Equal(t, "IMAGE", gjson.GetBytes(u.bodies[0], "generationConfig.responseModalities.1").String())
}

func TestModelProbeForwardGeminiTextRetainsCompletedOutcome(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
		resp := modelProbeHTTPResponse(200, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"OK\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":1,\"totalTokenCount\":11}}\n\n")
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGemini, "gemini-2.5-flash", service.CompositeRouteEndpointAny, u)
	result, err := h.forwardModelProbe(c, route, account)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.StreamCompleted)
	require.True(t, result.Outcome.HasEffectiveOutput())
	require.False(t, result.MediaSuccess)
	require.Len(t, u.requests, 1)
}

func TestModelProbeForwardUsesFrozenFirstOutputGuard(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGemini, "gemini-2.5-flash", service.CompositeRouteEndpointAny, u)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	// The parent copies this frozen root policy into each fresh attempt.
	c.Set("sub2api.model_first_output_request_policy", service.ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 1, SwitchSeconds: 1, HardCapSeconds: 5})
	_, err := h.forwardModelProbe(c, route, account)
	var failover *service.UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, service.GatewayFailureReason("first_output_timeout"), failover.Reason)
	require.True(t, failover.PreferNextAccount)
	require.NoError(t, ctx.Err(), "only the production per-account guard should expire")
	require.Len(t, u.requests, 1)
}

func TestModelProbeForwardGrokVideoMissingIDCannotReplay(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
		return modelProbeHTTPResponse(202, `{"status":"pending"}`), nil
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGrok, "grok-imagine-video", service.CompositeRouteEndpointAny, u)
	_, err := h.forwardModelProbe(c, route, account)
	require.Error(t, err)
	require.True(t, service.IsResponseCommitted(c))
	require.Len(t, u.requests, 1)
}

func TestModelProbeVideoStatusCaptureIsBounded(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	capture := &modelProbeStatusCapture{ResponseWriter: c.Writer}
	n, err := capture.Write([]byte(strings.Repeat("x", 256<<10)))
	require.NoError(t, err)
	require.Equal(t, 256<<10, n)
	require.Len(t, capture.body, 64<<10)
	require.True(t, capture.overflow)
}

func TestModelProbeForwardDoesNotTreatImageBillingFallbackAsOutput(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) { return modelProbeHTTPResponse(200, `{"data":[]}`), nil }}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformOpenAI, "gpt-image-2", service.CompositeRouteEndpointAny, u)
	route.group.ImageCountBilling = true
	result, err := h.forwardModelProbe(c, route, account)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.MediaSuccess, "the billing layer's fallback count does not prove generation")
	require.True(t, result.Outcome == nil || !result.Outcome.HasEffectiveOutput())
}

func TestModelProbeForwardTextCannotSucceedFromPartialImageCount(t *testing.T) {
	result := modelProbeOpenAIResult(&service.OpenAIForwardResult{ImageCount: 1, Outcome: &service.ResponseOutcome{HasMedia: true}})
	require.False(t, result.MediaSuccess)
	require.False(t, result.Outcome.StreamCompleted)
}

func TestModelProbeForwardVideoContentFailureIsNotHiddenByAcceptedHTTPStatus(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return modelProbeHTTPResponse(202, `{"request_id":"accepted-task"}`), nil
		}
		if strings.HasSuffix(req.URL.Path, "/content") {
			return modelProbeHTTPResponse(416, `{"error":"no content"}`), nil
		}
		return modelProbeHTTPResponse(200, `{"status":"done"}`), nil
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGrok, "grok-imagine-video", service.CompositeRouteEndpointAny, u)
	result, err := h.forwardModelProbe(c, route, account)
	require.True(t, err != nil || result == nil || !result.MediaSuccess)
	require.True(t, service.IsResponseCommitted(c))
}

func TestModelProbeForwardUsesProductionEndpointAndSingleAccountMapping(t *testing.T) {
	for _, tc := range []struct {
		name, platform, model, endpoint, path string
		responses                             bool
	}{
		{"openai_native_chat", service.PlatformOpenAI, "gpt-4.1-mini", service.CompositeRouteEndpointAny, "/v1/chat/completions", false},
		{"openai_responses", service.PlatformOpenAI, "gpt-4.1-mini", service.CompositeRouteEndpointResponses, "/v1/responses", true},
		{"anthropic_native", service.PlatformAnthropic, "claude-sonnet-4-5", service.CompositeRouteEndpointAny, "/v1/messages", false},
		{"gemini_native", service.PlatformGemini, "gemini-2.5-flash", service.CompositeRouteEndpointAny, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", false},
		{"gemini_messages", service.PlatformGemini, "gemini-2.5-flash", service.CompositeRouteEndpointMessages, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", false},
		{"gemini_chat", service.PlatformGemini, "gemini-2.5-flash", service.CompositeRouteEndpointChatCompletions, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
				return modelProbeHTTPResponse(503, `{"error":{"type":"overloaded_error","message":"unavailable"}}`), nil
			}}
			h, c, route, account := modelProbeForwardFixture(t, tc.platform, tc.model, tc.endpoint, u)
			account.Extra["openai_responses_supported"] = tc.responses
			_, err := h.forwardModelProbe(c, route, account)
			var failover *service.UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Equal(t, 503, failover.StatusCode)
			require.Len(t, u.requests, 1, "the sidecar must not retry or select another account")
			require.Equal(t, tc.path, u.requests[0].URL.Path)
			if tc.platform != service.PlatformGemini {
				require.Equal(t, tc.model, gjson.GetBytes(u.bodies[0], "model").String())
			}
			require.Equal(t, tc.model, account.GetMappedModel("channel-alias"))
			require.Equal(t, "must-not-map-twice", account.GetMappedModel(tc.model))
		})
	}
}

func TestModelProbeForwardPreservesTransportErrorWithoutInventingFailover(t *testing.T) {
	transportErr := errors.New("local fake transport stopped")
	u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) { return nil, transportErr }}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformOpenAI, "gpt-image-2", service.CompositeRouteEndpointAny, u)
	_, err := h.forwardModelProbe(c, route, account)
	require.Error(t, err)
	var failover *service.UpstreamFailoverError
	require.False(t, errors.As(err, &failover), "the image adapter's untyped transport error must remain untyped")
	require.Len(t, u.requests, 1)
}

func TestModelProbeForwardMediaRequiresActualImageOutput(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			model := "gpt-image-2"
			if platform == service.PlatformGrok {
				model = "grok-imagine-image"
			}
			u := &modelProbeForwardHTTP{respond: func(*http.Request) (*http.Response, error) {
				return modelProbeHTTPResponse(200, `{"data":[{"b64_json":"aW1hZ2U="}]}`), nil
			}}
			h, c, route, account := modelProbeForwardFixture(t, platform, model, service.CompositeRouteEndpointAny, u)
			result, err := h.forwardModelProbe(c, route, account)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.MediaSuccess)
			require.Len(t, u.requests, 1)
			require.Equal(t, "/v1/images/generations", u.requests[0].URL.Path)
			require.Equal(t, model, gjson.GetBytes(u.bodies[0], "model").String())
			require.False(t, gjson.GetBytes(u.bodies[0], "response_format").Exists())
		})
	}
}

func TestModelProbeForwardGrokVideoPollFailureCannotReplayAcceptedTask(t *testing.T) {
	u := &modelProbeForwardHTTP{respond: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return modelProbeHTTPResponse(202, `{"request_id":"accepted-task","status":"pending"}`), nil
		}
		return modelProbeHTTPResponse(503, `{"error":{"message":"status temporarily unavailable"}}`), nil
	}}
	h, c, route, account := modelProbeForwardFixture(t, service.PlatformGrok, "grok-imagine-video", service.CompositeRouteEndpointAny, u)
	result, err := h.forwardModelProbe(c, route, account)
	require.Error(t, err)
	var failover *service.UpstreamFailoverError
	require.ErrorAs(t, err, &failover, "the production adapter error must be retained")
	require.True(t, service.IsResponseCommitted(c), "the parent must not replay an accepted task, even for typed errors")
	require.True(t, result == nil || !result.MediaSuccess)
	require.Len(t, u.requests, 2)
	require.Equal(t, http.MethodPost, u.requests[0].Method)
	require.Equal(t, http.MethodGet, u.requests[1].Method)
	require.Equal(t, "/v1/videos/accepted-task", u.requests[1].URL.Path)
}

func TestModelProbeForwardGrokVideoRequiresCompletedTask(t *testing.T) {
	for _, status := range []string{"done", "completed", "failed", "expired"} {
		t.Run(status, func(t *testing.T) {
			u := &modelProbeForwardHTTP{respond: func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodPost {
					return modelProbeHTTPResponse(202, `{"request_id":"accepted-task"}`), nil
				}
				return modelProbeHTTPResponse(200, `{"status":"`+status+`","video":{"url":"https://vidgen.x.ai/test.mp4","duration":6}}`), nil
			}}
			h, c, route, account := modelProbeForwardFixture(t, service.PlatformGrok, "grok-imagine-video", service.CompositeRouteEndpointAny, u)
			result, err := h.forwardModelProbe(c, route, account)
			if status == "done" || status == "completed" {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.MediaSuccess)
			} else {
				require.Error(t, err)
				require.True(t, result == nil || !result.MediaSuccess)
			}
			require.Len(t, u.requests, 2)
		})
	}
}
