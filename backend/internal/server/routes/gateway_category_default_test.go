package routes

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type categoryDefaultRouteResolverStub struct {
	resolution *service.CategoryDefaultGroupModelResolution
	err        error
	calls      int
}

func (s *categoryDefaultRouteResolverStub) ResolveCategoryDefaultGroupModel(context.Context, *service.APIKey, string) (*service.CategoryDefaultGroupModelResolution, error) {
	s.calls++
	return s.resolution, s.err
}

func TestCategoryDefaultRuntimeErrorsExplainRequiredAction(t *testing.T) {
	for _, tt := range []struct {
		err     error
		status  int
		message string
	}{
		{service.ErrCategoryDefaultModelConflict, http.StatusConflict, "模型调用名重名冲突"},
		{service.ErrCategoryDefaultPendingSync, http.StatusServiceUnavailable, "等待管理员确认同步"},
		{service.ErrCategoryDefaultSourceUnavailable, http.StatusServiceUnavailable, "来源分组暂不可用"},
	} {
		t.Run(tt.message, func(t *testing.T) {
			resolver := &categoryDefaultRouteResolverStub{err: tt.err}
			r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
			r.POST("/v1/responses", func(c *gin.Context) { t.Fatal("unavailable source reached upstream") })
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"gemini-public"}`)))
			require.Equal(t, tt.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), tt.message)
		})
	}
}

func categoryDefaultMiddlewareFixture(t *testing.T, resolver *categoryDefaultRouteResolverStub) (*gin.Engine, *service.APIKey, *service.APIKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	entryID, sourceID := int64(91), int64(12)
	original := &service.APIKey{ID: 7, UserID: 3, GroupID: &entryID, User: &service.User{ID: 3}, Group: &service.Group{ID: entryID, Platform: service.PlatformComposite, IsCategoryDefault: true, Hydrated: true, Status: service.StatusActive}}
	source := &service.APIKey{ID: 7, UserID: 3, GroupID: &sourceID, CategoryDefaultGroupID: &entryID, User: &service.User{ID: 3}, Group: &service.Group{ID: sourceID, Platform: service.PlatformGemini, Hydrated: true, Status: service.StatusActive}}
	resolver.resolution = &service.CategoryDefaultGroupModelResolution{APIKey: source, EntryGroupID: entryID, PublicModel: "gemini-public", SourceModel: "gemini-public", GeminiNative: true}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyAPIKey), original); c.Next() })
	r.Use(categoryDefaultGroupTargetMiddleware(resolver))
	r.Use(middleware.GroupModelAllowlist())
	return r, original, source
}

func TestCategoryDefaultRouteBindsSourceBeforePricingAndAllowlist(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, original, source := categoryDefaultMiddlewareFixture(t, resolver)
	r.POST("/v1/messages", func(c *gin.Context) {
		key, _ := middleware.GetAPIKeyFromContext(c)
		require.Same(t, source, key)
		require.Same(t, source.Group, c.Request.Context().Value(ctxkey.Group))
		require.Same(t, source, c.Request.Context().Value(ctxkey.APIKey))
		_, hasSubscriptionResolution := service.SystemCustomGroupResolutionFromContext(c.Request.Context())
		require.False(t, hasSubscriptionResolution)
		body, _ := io.ReadAll(c.Request.Body)
		require.Equal(t, "gemini-public", gjson.GetBytes(body, "model").String())
		require.False(t, gjson.GetBytes(body, "Model").Exists(), "mixed-case duplicates are canonicalized before downstream parsers")
		c.Status(204)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"gemini-public","Model":"GEMINI-PUBLIC","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	require.Equal(t, 204, rec.Code)
	require.Equal(t, 1, resolver.calls)
	require.True(t, original.Group.IsCategoryDefault)
	require.Equal(t, int64(91), *original.GroupID)
}

func TestCategoryDefaultSourceAllowlistRunsAfterResolution(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, source := categoryDefaultMiddlewareFixture(t, resolver)
	source.Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other-model"}}
	r.POST("/v1/messages", func(c *gin.Context) { t.Fatal("source allowlist must reject before upstream") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"gemini-public"}`))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCategoryDefaultAmbiguousModelNeverResolves(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
	r.POST("/v1/chat/completions", func(c *gin.Context) { t.Fatal("ambiguous request reached handler") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gemini-public","MODEL":"unpublished-model"}`)))
	require.Equal(t, 400, rec.Code)
	require.Zero(t, resolver.calls)
}

func TestCategoryDefaultNativeGeminiUsesPublishedSource(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
	r.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		require.Equal(t, "gemini-public", compositeGeminiModelFromParams(c))
		c.Status(204)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public:streamGenerateContent", bytes.NewBufferString(`{"contents":[]}`)))
	require.Equal(t, 204, rec.Code)
	require.Equal(t, 1, resolver.calls)
}

func TestCategoryDefaultAntigravityNativeAliasPassesConcreteHandlerAdmission(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, source := categoryDefaultMiddlewareFixture(t, resolver)
	source.Group.Platform = service.PlatformAntigravity
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: source.UserID})
		c.Next()
	})
	h := &handler.GatewayHandler{}
	r.POST("/v1beta/models/*modelAction", h.GeminiV1BetaModels)
	recorder := httptest.NewRecorder()
	// An empty generation body stops after protocol admission, before any
	// scheduler or upstream service can be called by this isolated handler.
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public:generateContent", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Request body is empty")
	require.NotContains(t, recorder.Body.String(), "group platform is not gemini")
}

func TestCategoryDefaultNativeRejectsIncompatiblePublishedModel(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
	resolver.resolution.GeminiNative = false
	r.POST("/v1beta/models/*modelAction", func(c *gin.Context) { t.Fatal("incompatible native model reached handler") })
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public:generateContent", bytes.NewBufferString(`{"contents":[]}`)))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "不支持 Gemini 原生接口")
}

func TestCategoryDefaultOrdinaryCompositeSourceKeepsItsPublicMapping(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "messages", true: "native"}[native], func(t *testing.T) {
			resolver := &categoryDefaultRouteResolverStub{}
			r, _, source := categoryDefaultMiddlewareFixture(t, resolver)
			source.Group.Platform = service.PlatformComposite
			source.Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-public"}}
			composite := service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{{ID: 1, GroupID: source.Group.ID, PublicModel: "gemini-public", MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformGemini, UpstreamModel: "gemini-4-argon", Enabled: true}}})
			path, target := "/v1/messages", "/v1/messages"
			if native {
				r.Use(compositeGeminiTargetPlatformMiddleware(composite))
				path, target = "/v1beta/models/*modelAction", "/v1beta/models/gemini-public:generateContent"
			} else {
				r.Use(compositeTargetPlatformMiddleware(composite))
			}
			r.POST(path, func(c *gin.Context) {
				key, _ := middleware.GetAPIKeyFromContext(c)
				require.Equal(t, source.Group.ID, *key.GroupID)
				upstream, ok := service.ResolvedUpstreamModelFromContext(c.Request.Context())
				require.True(t, ok)
				require.Equal(t, "gemini-4-argon", upstream)
				platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
				require.True(t, ok)
				require.Equal(t, service.PlatformGemini, platform)
				if !native {
					body, _ := io.ReadAll(c.Request.Body)
					require.Equal(t, "gemini-4-argon", gjson.GetBytes(body, "model").String())
				}
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(`{"model":"gemini-public"}`)))
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		})
	}
}

func TestCategoryDefaultRejectsCompositeProviderAmbiguity(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "messages", true: "native"}[native], func(t *testing.T) {
			resolver := &categoryDefaultRouteResolverStub{}
			r, _, source := categoryDefaultMiddlewareFixture(t, resolver)
			source.Group.Platform = service.PlatformComposite
			composite := service.NewCompositeRouteResolver(nil)
			composite.SetModelOwnershipResolver(func(context.Context, int64, string) (service.CompositeModelOwnership, error) {
				return service.CompositeModelOwnership{Ambiguous: true}, nil
			})
			path, target := "/v1/messages", "/v1/messages"
			if native {
				r.Use(compositeGeminiTargetPlatformMiddleware(composite))
				path, target = "/v1beta/models/*modelAction", "/v1beta/models/gemini-public:generateContent"
			} else {
				r.Use(compositeTargetPlatformMiddleware(composite))
			}
			r.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(`{"model":"gemini-public"}`)))
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Contains(t, recorder.Body.String(), "模型路由歧义")
		})
	}
}

func TestCategoryDefaultCompositeGuardPreservesOrdinaryKeyRouting(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "messages", true: "native"}[native], func(t *testing.T) {
			resolver := &categoryDefaultRouteResolverStub{}
			r, ordinary, _ := categoryDefaultMiddlewareFixture(t, resolver)
			ordinary.Group.IsCategoryDefault = false
			composite := service.NewCompositeRouteResolver(nil)
			composite.SetModelOwnershipResolver(func(context.Context, int64, string) (service.CompositeModelOwnership, error) {
				return service.CompositeModelOwnership{Ambiguous: true}, nil
			})
			path, target := "/v1/messages", "/v1/messages"
			if native {
				r.Use(compositeGeminiTargetPlatformMiddleware(composite))
				path, target = "/v1beta/models/*modelAction", "/v1beta/models/gemini-public:generateContent"
			} else {
				r.Use(compositeTargetPlatformMiddleware(composite))
			}
			r.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(`{"model":"gemini-public"}`)))
			require.Equal(t, http.StatusNoContent, recorder.Code)
			require.Zero(t, resolver.calls, "ordinary keys never acquire a category source")
		})
	}
}

func TestCategoryDefaultUnsupportedEndpointNeverResolves(t *testing.T) {
	resolver := &categoryDefaultRouteResolverStub{}
	r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
	r.GET("/v1/responses", func(c *gin.Context) { t.Fatal("unsupported websocket reached handler") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	require.Equal(t, 501, rec.Code)
	require.Zero(t, resolver.calls)
}

func TestCategoryDefaultBackgroundTasksRequireSpecificGroup(t *testing.T) {
	for _, payload := range []string{
		`{"model":"gemini-public","background":true}`,
		`{"model":"gemini-public","background":false,"Background":true}`,
	} {
		resolver := &categoryDefaultRouteResolverStub{}
		r, _, _ := categoryDefaultMiddlewareFixture(t, resolver)
		r.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(payload)))
		require.Equal(t, http.StatusNotImplemented, recorder.Code, recorder.Body.String())
		require.Zero(t, resolver.calls, "asynchronous response creation cannot acquire a model source")
	}
}

func TestCategoryDefaultRegisteredRoutesDoNotFallThroughToOrdinaryComposite(t *testing.T) {
	r, key, repo := newSystemCustomUnsupportedGatewayRouter(t)
	repo.apiKey.Group.SystemCustomRoutingEnabled = false
	repo.apiKey.Group.SubscriptionType = service.SubscriptionTypeStandard
	repo.apiKey.Group.IsCategoryDefault = true
	for _, tt := range []struct{ method, path string }{
		{"GET", "/v1/responses"}, {"GET", "/responses"}, {"GET", "/backend-api/codex/responses"},
		{"POST", "/v1/live"}, {"POST", "/backend-api/codex/realtime/calls"},
		{"GET", "/v1/live/call-123"}, {"GET", "/backend-api/codex/call-123"},
		{"POST", "/v1/images/batches"}, {"GET", "/v1/images/batches/models"}, {"GET", "/v1/images/tasks/test"},
		{"POST", "/v1/images/generations"}, {"POST", "/images/generations/async"},
		{"GET", "/v1beta/cachedContents"}, {"POST", "/v1beta/cachedContents"},
		{"POST", "/antigravity/v1/messages"}, {"GET", "/antigravity/models"}, {"GET", "/antigravity/v1beta/models"},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(`{"model":"gemini-public"}`))
			req.Header.Set("Authorization", "Bearer "+key)
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
		})
	}
	require.Equal(t, 1, repo.loads, "the category marker must survive auth-cache hits")
}

func TestCategoryDefaultRegisteredTextAliasesAllRequireSourceResolution(t *testing.T) {
	r, key, repo := newSystemCustomUnsupportedGatewayRouter(t)
	repo.apiKey.Group.SystemCustomRoutingEnabled = false
	repo.apiKey.Group.SubscriptionType = service.SubscriptionTypeStandard
	repo.apiKey.Group.IsCategoryDefault = true
	// The intentionally unconfigured resolver must reject each supported alias
	// locally, instead of passing an entry without accounts into a real handler.
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens", "/v1/chat/completions", "/chat/completions", "/v1/responses", "/responses", "/responses/compact", "/backend-api/codex/responses", "/relay-stream/v1/responses", "/relay-nonstream/v1/chat/completions", "/v1beta/models/gemini-public:generateContent", "/v1beta/models/gemini-public:countTokens"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"model":"gemini-public"}`))
			req.Header.Set("Authorization", "Bearer "+key)
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		})
	}
}
