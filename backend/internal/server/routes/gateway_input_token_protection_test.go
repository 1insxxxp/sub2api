package routes

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestInputProtectionBillingGroupAndOriginalPayload(t *testing.T) {
	const path = "/v1/chat/completions"
	const original = `{"model":"alias","messages":[{"role":"user","content":"hello"}]}`
	limit := service.EstimateCustomerInputTokenProtection(path, []byte(original)).Limit
	require.Positive(t, limit)
	for _, tc := range []struct {
		name                                 string
		entry, source, deferred, token, want bool
	}{
		{"enabled group survives fallback to disabled group", true, false, false, true, true},
		{"disabled group survives fallback to enabled group", false, true, false, true, false},
		{"category entry follows source group", false, true, true, true, true},
		{"category entry respects source opt out", true, false, true, true, false},
		{"flat price retains evidence", true, true, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			setKey := func(c *gin.Context, enabled, category bool) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 42, IsCategoryDefault: category, GeminiInputCountingEnabled: enabled}})
			}
			r.POST(path, func(c *gin.Context) { setKey(c, tc.entry, tc.deferred) }, captureOriginalCustomerInput, func(c *gin.Context) {
				setKey(c, tc.source, false)
				requestmodel.ResetRequestBody(c.Request, []byte(strings.Replace(original, "hello", strings.Repeat("injected ", 700), 1)))
			}, bindCustomerInputProtection(func(ctx context.Context, key *service.APIKey, model string, attempt service.InputTokenProtectionAttempt, imageEndpoint bool) bool {
				require.Equal(t, "alias", model)
				return tc.token
			}), func(c *gin.Context) {
				p := service.InputTokenProtectionFromContext(c.Request.Context())
				if tc.want {
					require.NotNil(t, p)
					require.Equal(t, limit, p.Limit)
				} else {
					require.Nil(t, p)
				}
				c.Data(200, "application/json", []byte(`{"usage":{"prompt_tokens":5356,"completion_tokens":2,"total_tokens":5358,"prompt_tokens_details":{"cached_tokens":3295}}}`))
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(original)))
			require.Equal(t, 200, w.Code)
			want := 5356
			if tc.want && tc.token {
				want = limit
			}
			require.EqualValues(t, want, gjson.GetBytes(w.Body.Bytes(), "usage.prompt_tokens").Int())
			require.EqualValues(t, min(want, 3295), gjson.GetBytes(w.Body.Bytes(), "usage.prompt_tokens_details.cached_tokens").Int())
		})
	}
}

func TestInputProtectionRouteExclusions(t *testing.T) {
	for _, path := range []string{"/v1/images/generations/async", "/v1/videos", "/v1/messages/count_tokens", "/v1beta/models/custom:countTokens", "/v1/responses/input_tokens", "/v1/alpha/search"} {
		require.False(t, inputProtectionRoute(path), path)
	}
	for _, path := range []string{"/v1beta/models/custom:streamGenerateContent", "/v1beta/cachedContents", "/antigravity/v1/messages", "/relay-stream/v1/chat/completions", "/backend-api/codex/responses", "/v1/systemone", "/v1/images/generations", "/v1/images/edits"} {
		require.True(t, inputProtectionRoute(path), path)
	}
}

func TestInputProtectionPricesUsageAtWrite(t *testing.T) {
	const path = "/v1/responses"
	const raw = `{"model":"public","input":"hello"}`
	limit := service.EstimateCustomerInputTokenProtection(path, []byte(raw)).Limit
	for _, token := range []bool{true, false} {
		r := gin.New()
		var prices int
		r.POST(path, func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{GeminiInputCountingEnabled: true}})
		}, captureOriginalCustomerInput, bindCustomerInputProtection(func(_ context.Context, key *service.APIKey, requested string, attempt service.InputTokenProtectionAttempt, image bool) bool {
			prices++
			require.EqualValues(t, 72, key.Group.ID)
			require.Equal(t, "public", requested)
			require.Equal(t, "selected-account-model", attempt.UpstreamModel)
			require.False(t, image)
			return token
		}), func(c *gin.Context) {
			require.Equal(t, limit, service.InputTokenProtectionFromContext(c.Request.Context()).Limit)
			c.Header("Content-Type", "text/event-stream")
			_, err := c.Writer.WriteString(": ping\n\ndata: {\"response\":{")
			require.NoError(t, err)
			c.Writer.Flush()
			require.Zero(t, prices)
			c.Set(middleware.InputProtectionPricingKey, &service.APIKey{Group: &service.Group{ID: 72}})
			service.SetOpsUpstreamModel(c, "selected-account-model")
			_, err = c.Writer.WriteString(`"usage":{"input_tokens":5000,"output_tokens":1}}}` + "\n\n")
			require.NoError(t, err)
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(raw)))
		require.Equal(t, 1, prices)
		_, data, ok := strings.Cut(w.Body.String(), "data: ")
		require.True(t, ok)
		want := 5000
		if token {
			want = limit
		}
		require.EqualValues(t, want, gjson.Get(data, "response.usage.input_tokens").Int())
	}
}

func TestInputProtectionCachedContentsSize(t *testing.T) {
	const path = "/v1beta/cachedContents"
	const metadata = `{"name":"cachedContents/1","usageMetadata":{"totalTokenCount":9987}}`
	r := gin.New()
	r.POST(path, func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{GeminiInputCountingEnabled: true}})
	}, captureOriginalCustomerInput, bindCustomerInputProtection(func(context.Context, *service.APIKey, string, service.InputTokenProtectionAttempt, bool) bool {
		t.Fatal("cached resource metadata does not have a wire tariff")
		return true
	}), func(c *gin.Context) {
		require.NotNil(t, service.InputTokenProtectionFromContext(c.Request.Context()))
		c.Data(200, "application/json", []byte(metadata))
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(`{"model":"models/gemini","contents":[{"parts":[{"text":"hello"}]}]}`)))
	require.Equal(t, metadata, w.Body.String())
}

func TestInputProtectionCustomGroupOwnsOptIn(t *testing.T) {
	const path = "/v1/responses"
	const body = `{"model":"my-alias","input":"hello"}`
	id := int64(17)
	for _, tc := range []struct {
		name                           string
		container, entry, source, want bool
	}{
		{"personal alias uses source", false, false, true, true},
		{"personal alias honors disabled source", false, true, false, false},
		{"subscription container owns switch even with stale personal ID", true, true, false, true},
		{"disabled subscription container", true, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST(path, func(c *gin.Context) {
				entry := &service.Group{GeminiInputCountingEnabled: tc.entry}
				if tc.container {
					entry.SystemCustomRoutingEnabled = true
					entry.Platform = service.PlatformComposite
					entry.SubscriptionType = service.SubscriptionTypeSubscription
				}
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{CustomGroupID: &id, Group: entry})
			}, captureOriginalCustomerInput, func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &id, Group: &service.Group{ID: id, GeminiInputCountingEnabled: tc.source}})
			}, bindCustomerInputProtection(func(_ context.Context, key *service.APIKey, _ string, _ service.InputTokenProtectionAttempt, _ bool) bool {
				require.Equal(t, id, key.Group.ID)
				return true
			}), func(c *gin.Context) {
				require.Equal(t, tc.want, service.InputTokenProtectionFromContext(c.Request.Context()) != nil)
				c.Data(200, "application/json", []byte(`{"usage":{"input_tokens":999}}`))
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(body)))
			want := 999
			if tc.want {
				want = service.EstimateCustomerInputTokenProtection(path, []byte(body)).Limit
			}
			require.EqualValues(t, want, gjson.Get(w.Body.String(), "usage.input_tokens").Int())
		})
	}
}
