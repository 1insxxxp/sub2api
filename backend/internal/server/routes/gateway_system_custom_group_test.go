package routes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type systemCustomUnsupportedAuthRepo struct {
	service.APIKeyRepository
	apiKey *service.APIKey
	loads  int
}

type systemCustomUnsupportedAuthCache struct {
	service.APIKeyCache
	entry *service.APIKeyAuthCacheEntry
}

type systemCustomModelsRouteRepository struct {
	service.SystemCustomGroupRepository
	routes []service.SystemCustomGroupModel
}

func (r systemCustomModelsRouteRepository) ListModels(context.Context, int64, bool) ([]service.SystemCustomGroupModel, error) {
	return append([]service.SystemCustomGroupModel(nil), r.routes...), nil
}

type systemCustomModelsGroupRepository struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r systemCustomModelsGroupRepository) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	group, ok := r.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	clone := *group
	return &clone, nil
}

type systemCustomModelsCatalog struct {
	models map[int64][]string
}

func (c systemCustomModelsCatalog) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	if groupID == nil {
		return nil
	}
	return append([]string(nil), c.models[*groupID]...)
}

func (systemCustomModelsCatalog) HasSchedulableAccountsForGroupPlatform(context.Context, int64, string) bool {
	return true
}

func (c *systemCustomUnsupportedAuthCache) GetAuthCache(context.Context, string) (*service.APIKeyAuthCacheEntry, error) {
	if c.entry == nil {
		return nil, errors.New("cache miss")
	}
	return c.entry, nil
}

func (c *systemCustomUnsupportedAuthCache) SetAuthCache(_ context.Context, _ string, entry *service.APIKeyAuthCacheEntry, _ time.Duration) error {
	c.entry = entry
	return nil
}

func (r *systemCustomUnsupportedAuthRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	if r.apiKey == nil || key != r.apiKey.Key {
		return nil, service.ErrAPIKeyNotFound
	}
	r.loads++
	clone := *r.apiKey
	groupClone := *r.apiKey.Group
	clone.Group = &groupClone
	return &clone, nil
}

func (r *systemCustomUnsupportedAuthRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func newSystemCustomUnsupportedGatewayRouter(t *testing.T) (*gin.Engine, string, *systemCustomUnsupportedAuthRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID := int64(25)
	apiKey := &service.APIKey{
		ID: 7, UserID: 9, Key: "system-custom-unsupported-key", Status: service.StatusActive,
		GroupID: &groupID,
		Group: &service.Group{
			ID: groupID, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true,
			SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
		},
		User: &service.User{ID: 9, Role: service.RoleUser, Status: service.StatusActive},
	}
	cfg := &config.Config{
		RunMode:    config.RunModeSimple,
		Gateway:    config.GatewayConfig{MaxBodySize: 1024 * 1024, TextMaxBodySize: 1024 * 1024},
		APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60},
	}
	repo := &systemCustomUnsupportedAuthRepo{apiKey: apiKey}
	cache := &systemCustomUnsupportedAuthCache{}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{},
			AsyncImage: &handler.AsyncImageHandler{}, BatchImage: &handler.BatchImageHandler{},
		},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg), apiKeyService,
		nil, nil, nil, nil, cfg,
	)
	return router, apiKey.Key, repo
}

func TestSystemCustomUnsupportedEndpointsFailClosedInRegisteredRouter(t *testing.T) {
	router, key, repo := newSystemCustomUnsupportedGatewayRouter(t)
	tests := []struct {
		method, path string
		google       bool
		anthropic    bool
	}{
		{method: http.MethodGet, path: "/responses"},
		{method: http.MethodGet, path: "/v1/responses"},
		{method: http.MethodGet, path: "/backend-api/codex/responses"},
		{method: http.MethodPost, path: "/v1/live"},
		{method: http.MethodGet, path: "/v1/live/call-123"},
		{method: http.MethodPost, path: "/backend-api/codex/realtime/calls"},
		{method: http.MethodGet, path: "/backend-api/codex/call-123"},
		{method: http.MethodPost, path: "/v1/images/batches"},
		{method: http.MethodGet, path: "/v1/images/batches"},
		{method: http.MethodGet, path: "/v1/images/batches/models"},
		{method: http.MethodGet, path: "/v1/images/batches/123/items"},
		{method: http.MethodGet, path: "/v1/images/batches/123/items/item-1/content"},
		{method: http.MethodGet, path: "/v1/images/batches/123/download"},
		{method: http.MethodPost, path: "/v1/images/batches/123/cancel"},
		{method: http.MethodDelete, path: "/v1/images/batches/123"},
		{method: http.MethodDelete, path: "/v1/images/batches/123/outputs"},
		{method: http.MethodGet, path: "/antigravity/models", anthropic: true},
		{method: http.MethodPost, path: "/antigravity/v1/messages", anthropic: true},
		{method: http.MethodGet, path: "/antigravity/v1/models", anthropic: true},
		{method: http.MethodGet, path: "/antigravity/v1/usage", anthropic: true},
		{method: http.MethodGet, path: "/antigravity/v1beta/models", google: true},
		{method: http.MethodGet, path: "/antigravity/v1beta/models/gemini-3", google: true},
		{method: http.MethodPost, path: "/antigravity/v1beta/models/gemini-3:generateContent", google: true},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			body := io.Reader(nil)
			if tt.method == http.MethodPost {
				body = bytes.NewBufferString(`{"model":"any"}`)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusNotImplemented, recorder.Code)
			if tt.google {
				require.JSONEq(t, `{"error":{"code":501,"message":"This endpoint is not supported for system custom subscription groups","status":"INTERNAL"}}`, recorder.Body.String())
			} else if tt.anthropic {
				require.Contains(t, recorder.Body.String(), `"type":"not_supported_error"`)
				require.NotContains(t, recorder.Body.String(), `"code":"SYSTEM_CUSTOM_GROUP_ENDPOINT_UNSUPPORTED"`)
			} else {
				require.Contains(t, recorder.Body.String(), `"code":"SYSTEM_CUSTOM_GROUP_ENDPOINT_UNSUPPORTED"`)
				require.NotContains(t, recorder.Body.String(), "MODEL_REQUIRED")
			}
		})
	}
	require.Equal(t, 1, repo.loads, "the first request must load the auth snapshot and every later request must preserve the system marker from cache")
}

func TestSystemCustomUnsupportedEndpointMiddlewareAllowsOrdinaryKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
			Group: &service.Group{
				ID: 42, Platform: service.PlatformOpenAI, Status: service.StatusActive,
				SubscriptionType: service.SubscriptionTypeStandard,
			},
		})
		c.Next()
	})
	router.Use(systemCustomGroupUnsupportedEndpointMiddleware(systemCustomUnsupportedOpenAI, false))
	router.GET("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestSystemCustomModelListsUseRealAuthSnapshotAndAliasesInRegisteredRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID := int64(25)
	apiKey := &service.APIKey{
		ID: 7, UserID: 9, Key: "system-custom-model-list-key", Status: service.StatusActive,
		GroupID: &billingGroupID,
		Group: &service.Group{
			ID: billingGroupID, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true,
			SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
		},
		User: &service.User{ID: 9, Role: service.RoleUser, Status: service.StatusActive},
	}
	authRepo := &systemCustomUnsupportedAuthRepo{apiKey: apiKey}
	authCache := &systemCustomUnsupportedAuthCache{}
	groupRepo := systemCustomModelsGroupRepository{groups: map[int64]*service.Group{
		10: {ID: 10, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true},
		20: {ID: 20, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true},
	}}
	routeRepo := systemCustomModelsRouteRepository{routes: []service.SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "claude-monthly", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "gemini-monthly", SourceGroupID: 20, SourceModel: "gemini-2.5-flash", Enabled: true},
	}}
	catalog := systemCustomModelsCatalog{models: map[int64][]string{10: {"claude-sonnet-4"}, 20: {"gemini-2.5-flash"}}}
	cfg := &config.Config{
		RunMode:    config.RunModeSimple,
		Gateway:    config.GatewayConfig{MaxBodySize: 1024 * 1024, TextMaxBodySize: 1024 * 1024},
		APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60},
	}
	apiKeyService := service.NewAPIKeyService(authRepo, nil, groupRepo, nil, nil, authCache, cfg)
	apiKeyService.SetSystemCustomGroupRepository(routeRepo)
	apiKeyService.SetSystemCustomGroupModelCatalog(catalog)
	gatewayHandler := handler.NewGatewayHandler(
		nil, nil, nil, nil, nil, nil, nil, nil, apiKeyService, nil, nil, nil, nil, cfg, nil,
	)
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway: gatewayHandler, OpenAIGateway: &handler.OpenAIGatewayHandler{},
			AsyncImage: &handler.AsyncImageHandler{}, BatchImage: &handler.BatchImageHandler{},
		},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg), apiKeyService,
		nil, nil, nil, nil, cfg,
	)

	tests := []struct {
		name string
		path string
		kind string
		want []string
	}{
		{name: "root standard", path: "/models", kind: "openai", want: []string{"claude-monthly", "gemini-monthly"}},
		{name: "v1 standard", path: "/v1/models", kind: "openai", want: []string{"claude-monthly", "gemini-monthly"}},
		{name: "root codex", path: "/models?client_version=0.144.0", kind: "codex", want: []string{"claude-monthly", "gemini-monthly"}},
		{name: "v1 codex", path: "/v1/models?client_version=0.144.0", kind: "codex", want: []string{"claude-monthly", "gemini-monthly"}},
		{name: "direct codex", path: "/backend-api/codex/models", kind: "codex", want: []string{"claude-monthly", "gemini-monthly"}},
		{name: "gemini", path: "/v1beta/models", kind: "gemini", want: []string{"models/gemini-monthly"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey.Key)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			resultPath := "data.#.id"
			switch tt.kind {
			case "codex":
				resultPath = "models.#.slug"
				require.False(t, gjson.Get(recorder.Body.String(), "object").Exists(), "Codex clients require a manifest, not the standard OpenAI list")
			case "gemini":
				resultPath = "models.#.name"
			case "openai":
				require.Equal(t, "list", gjson.Get(recorder.Body.String(), "object").String())
				items := gjson.Get(recorder.Body.String(), "data").Array()
				require.Len(t, items, len(tt.want))
				for _, item := range items {
					require.Equal(t, "model", item.Get("object").String())
					require.Equal(t, gjson.Number, item.Get("created").Type)
					require.Positive(t, item.Get("created").Int())
					require.NotEmpty(t, item.Get("owned_by").String())
				}
			}
			results := gjson.Get(recorder.Body.String(), resultPath).Array()
			got := make([]string, 0, len(results))
			for _, result := range results {
				got = append(got, result.String())
			}
			require.Equal(t, tt.want, got)
			require.NotContains(t, recorder.Body.String(), "claude-sonnet-4")
			require.NotContains(t, recorder.Body.String(), "gemini-2.5-flash")
		})
	}
	require.Equal(t, 1, authRepo.loads, "all paths after the first must preserve the system custom marker through the auth cache")
}

type systemCustomGroupResolverStub struct {
	resolution *service.SystemCustomGroupModelResolution
	err        error
	calls      int
	model      string
}

type customGroupResolverCallStub struct {
	calls int
}

func (s *customGroupResolverCallStub) ResolveCustomGroupModel(_ context.Context, _ *service.APIKey, _ string) (*service.CustomGroupModelResolution, error) {
	s.calls++
	return nil, errors.New("user custom resolver must not run after a system custom resolution")
}

func (s *systemCustomGroupResolverStub) ResolveSystemCustomGroupModel(_ context.Context, _ *service.APIKey, model string) (*service.SystemCustomGroupModelResolution, error) {
	s.calls++
	s.model = model
	return s.resolution, s.err
}

func TestSystemCustomTargetMiddlewareRewritesJSONAndReplacesRequestContexts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	billingGroup := systemCustomBillingGroup(billingGroupID)
	sourceGroup := &service.Group{ID: sourceGroupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	originalKey := &service.APIKey{ID: 7, GroupID: &billingGroupID, Group: billingGroup}
	resolvedKey := &service.APIKey{ID: 7, GroupID: &sourceGroupID, Group: sourceGroup}
	subscription := &service.UserSubscription{ID: 88, GroupID: billingGroupID}
	resolver := &systemCustomGroupResolverStub{resolution: &service.SystemCustomGroupModelResolution{
		APIKey: resolvedKey,
		SystemCustomGroupResolution: service.SystemCustomGroupResolution{
			BillingGroupID: billingGroupID, SourceGroupID: sourceGroupID,
			PublicModel: "monthly-gpt", SourceModel: "gpt-5.4", SourcePlatform: service.PlatformOpenAI,
		},
	}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyAPIKey), originalKey)
		c.Set(string(servermiddleware.ContextKeySubscription), subscription)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, billingGroup))
		c.Next()
	})
	router.Use(systemCustomGroupTargetMiddleware(resolver))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		key, ok := servermiddleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		requestKey, ok := c.Request.Context().Value(ctxkey.APIKey).(*service.APIKey)
		require.True(t, ok)
		requestGroup, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group)
		require.True(t, ok)
		gotSub, ok := servermiddleware.GetSubscriptionFromContext(c)
		require.True(t, ok)
		resolution, ok := service.SystemCustomGroupResolutionFromContext(c.Request.Context())
		require.True(t, ok)
		publicModel, publicOK := service.RequestedPublicModelFromContext(c.Request.Context())
		sourceModel, sourceOK := service.ResolvedUpstreamModelFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{
			"body_model": gjson.GetBytes(body, "model").String(), "key_group": *key.GroupID,
			"request_key_group": *requestKey.GroupID, "request_group": requestGroup.ID,
			"subscription_group": gotSub.GroupID, "resolution_billing": resolution.BillingGroupID,
			"resolution_source": resolution.SourceGroupID, "resolution_platform": resolution.SourcePlatform,
			"public_model": publicModel, "source_model": sourceModel, "models_ok": publicOK && sourceOK,
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"monthly-gpt","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"body_model":"gpt-5.4","key_group":42,"request_key_group":42,"request_group":42,
		"subscription_group":25,"resolution_billing":25,"resolution_source":42,
		"resolution_platform":"openai","public_model":"monthly-gpt","source_model":"gpt-5.4","models_ok":true
	}`, recorder.Body.String())
	require.Equal(t, "monthly-gpt", resolver.model)
	require.Equal(t, billingGroupID, *originalKey.GroupID)
	require.Same(t, billingGroup, originalKey.Group)
}

func TestSystemCustomTargetMiddlewarePreventsSecondUserCustomResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID, staleCustomGroupID := int64(25), int64(42), int64(99)
	billingGroup := systemCustomBillingGroup(billingGroupID)
	originalKey := &service.APIKey{ID: 7, GroupID: &billingGroupID, CustomGroupID: &staleCustomGroupID, Group: billingGroup}
	resolvedKey := &service.APIKey{
		ID: 7, GroupID: &sourceGroupID, Group: &service.Group{
			ID: sourceGroupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true,
		},
	}
	systemResolver := &systemCustomGroupResolverStub{resolution: &service.SystemCustomGroupModelResolution{
		APIKey: resolvedKey,
		SystemCustomGroupResolution: service.SystemCustomGroupResolution{
			BillingGroupID: billingGroupID, SourceGroupID: sourceGroupID,
			PublicModel: "monthly-gpt", SourceModel: "gpt-5.4", SourcePlatform: service.PlatformOpenAI,
		},
	}}
	userResolver := &customGroupResolverCallStub{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyAPIKey), originalKey)
		c.Next()
	})
	router.Use(systemCustomGroupTargetMiddleware(systemResolver))
	router.Use(customGroupTargetMiddleware(userResolver))
	router.Use(compositeTargetPlatformMiddleware(nil))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		_, compositeResolved := service.ResolvedTargetPlatformFromContext(c.Request.Context())
		require.False(t, compositeResolved, "source group must bypass the later composite resolver")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"monthly-gpt"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Zero(t, userResolver.calls)
	require.Equal(t, staleCustomGroupID, *originalKey.CustomGroupID)
}

func TestSystemCustomTargetMiddlewareRejectsResolutionForAnotherBillingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(999, sourceGroupID, service.PlatformOpenAI, "monthly-gpt", "gpt-5.4")
	router := systemCustomTestRouter(systemCustomBillingGroup(billingGroupID), resolver)
	router.POST("/v1/chat/completions", func(c *gin.Context) { t.Fatal("handler must not run") })

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"monthly-gpt"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.JSONEq(t, `{"error":{"type":"service_unavailable_error","message":"The selected model source is temporarily unavailable","code":"SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"}}`, recorder.Body.String())
}

func TestSystemCustomTargetMiddlewareRejectsResolutionWithMismatchedSourcePlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformOpenAI, "monthly-gpt", "gpt-5.4")
	resolver.resolution.SourcePlatform = service.PlatformAnthropic
	router := systemCustomTestRouter(systemCustomBillingGroup(billingGroupID), resolver)
	router.POST("/v1/chat/completions", func(c *gin.Context) { t.Fatal("handler must not run") })

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"monthly-gpt"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.JSONEq(t, `{"error":{"type":"service_unavailable_error","message":"The selected model source is temporarily unavailable","code":"SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"}}`, recorder.Body.String())
}

func TestSystemCustomTargetMiddlewareRewritesMultipartWithoutDamagingOtherParts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformOpenAI, "image-monthly", "gpt-image-1")
	router := systemCustomTestRouter(systemCustomBillingGroup(billingGroupID), resolver)
	router.POST("/v1/images/edits", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		mediaReader := multipart.NewReader(bytes.NewReader(body), multipartBoundary(t, c.GetHeader("Content-Type")))
		form, err := mediaReader.ReadForm(1024)
		require.NoError(t, err)
		defer func() { require.NoError(t, form.RemoveAll()) }()
		opened, err := form.File["image"][0].Open()
		require.NoError(t, err)
		fileBytes, err := io.ReadAll(opened)
		require.NoError(t, err)
		require.NoError(t, opened.Close())
		c.JSON(http.StatusOK, gin.H{"model": form.Value["model"][0], "prompt": form.Value["prompt"][0], "file": string(fileBytes)})
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "image-monthly"))
	require.NoError(t, writer.WriteField("prompt", "keep me"))
	file, err := writer.CreateFormFile("image", "sample.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("png-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"gpt-image-1","prompt":"keep me","file":"png-bytes"}`, recorder.Body.String())
}

func TestSystemCustomTargetMiddlewarePreservesExactSourceModelCasing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformOpenAI, "gpt-5.4", "gpt-5.4")
	router := systemCustomTestRouter(systemCustomBillingGroup(billingGroupID), resolver)
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.JSON(http.StatusOK, gin.H{"model": gjson.GetBytes(body, "model").String()})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"GPT-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"gpt-5.4"}`, recorder.Body.String())
}

func TestSystemCustomTargetMiddlewarePreservesExactSourceModelCasingInMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformOpenAI, "gpt-image-1", "gpt-image-1")
	router := systemCustomTestRouter(systemCustomBillingGroup(billingGroupID), resolver)
	router.POST("/v1/images/edits", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		mediaReader := multipart.NewReader(bytes.NewReader(body), multipartBoundary(t, c.GetHeader("Content-Type")))
		form, err := mediaReader.ReadForm(1024)
		require.NoError(t, err)
		defer func() { require.NoError(t, form.RemoveAll()) }()
		c.JSON(http.StatusOK, gin.H{"model": form.Value["model"][0]})
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "GPT-IMAGE-1"))
	require.NoError(t, writer.WriteField("prompt", "keep me"))
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"gpt-image-1"}`, recorder.Body.String())
}

func TestSystemCustomGeminiTargetMiddlewareRewritesPathAndPreservesContexts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformGemini, "gemini-monthly", "gemini-2.5-flash")
	router := systemCustomAuthTestRouter(systemCustomBillingGroup(billingGroupID))
	router.Use(systemCustomGroupGeminiTargetMiddleware(resolver))
	router.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		resolution, ok := service.SystemCustomGroupResolutionFromContext(c.Request.Context())
		require.True(t, ok)
		group := c.Request.Context().Value(ctxkey.Group).(*service.Group)
		c.JSON(http.StatusOK, gin.H{"model": compositeGeminiModelFromParams(c), "group": group.ID, "billing": resolution.BillingGroupID})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-monthly:generateContent", bytes.NewBufferString(`{"contents":[]}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"gemini-2.5-flash","group":42,"billing":25}`, recorder.Body.String())
}

func TestSystemCustomGeminiTargetMiddlewareRewritesGetModelPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformGemini, "gemini-monthly", "gemini-2.5-flash")
	router := systemCustomAuthTestRouter(systemCustomBillingGroup(billingGroupID))
	router.Use(systemCustomGroupGeminiTargetMiddleware(resolver))
	router.GET("/v1beta/models/:model", func(c *gin.Context) {
		publicModel, publicOK := service.RequestedPublicModelFromContext(c.Request.Context())
		sourceModel, sourceOK := service.ResolvedUpstreamModelFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{
			"path_model": c.Param("model"), "public_model": publicModel,
			"source_model": sourceModel, "models_ok": publicOK && sourceOK,
		})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1beta/models/gemini-monthly", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"path_model":"gemini-2.5-flash","public_model":"gemini-monthly","source_model":"gemini-2.5-flash","models_ok":true}`, recorder.Body.String())
}

func TestSystemCustomGeminiTargetMiddlewarePreservesExactSourceModelCasing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billingGroupID, sourceGroupID := int64(25), int64(42)
	resolver := resolvedSystemCustomStub(billingGroupID, sourceGroupID, service.PlatformGemini, "gemini-2.5-flash", "gemini-2.5-flash")
	router := systemCustomAuthTestRouter(systemCustomBillingGroup(billingGroupID))
	router.Use(systemCustomGroupGeminiTargetMiddleware(resolver))
	router.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"model": compositeGeminiModelFromParams(c)})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/GEMINI-2.5-FLASH:generateContent", bytes.NewBufferString(`{"contents":[]}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"gemini-2.5-flash"}`, recorder.Body.String())
}

func TestSystemCustomTargetMiddlewareRejectsNonStringJSONModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	protocols := []struct {
		name string
		path string
	}{
		{name: "anthropic", path: "/v1/messages"},
		{name: "openai", path: "/v1/chat/completions"},
	}
	values := []struct {
		name string
		body string
	}{
		{name: "number", body: `{"model":123}`},
		{name: "boolean", body: `{"model":true}`},
		{name: "object", body: `{"model":{"alias":"gpt-5.4"}}`},
		{name: "array", body: `{"model":["gpt-5.4"]}`},
		{name: "null", body: `{"model":null}`},
		{name: "missing", body: `{}`},
		{name: "malformed", body: `{"model":`},
	}
	for _, protocol := range protocols {
		for _, value := range values {
			t.Run(protocol.name+"/"+value.name, func(t *testing.T) {
				resolver := &systemCustomGroupResolverStub{err: service.ErrSystemCustomGroupModelNotAllowed}
				router := systemCustomTestRouter(systemCustomBillingGroup(25), resolver)
				router.POST(protocol.path, func(c *gin.Context) { t.Fatal("handler must not run") })
				req := httptest.NewRequest(http.MethodPost, protocol.path, bytes.NewBufferString(value.body))
				req.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)

				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
				require.Zero(t, resolver.calls, "invalid model types must be rejected before alias resolution")
			})
		}
	}
}

func TestSystemCustomTargetMiddlewareFailsClosedWithProtocolErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		path       string
		gemini     bool
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "anthropic unknown alias", path: "/v1/messages", err: service.ErrSystemCustomGroupModelNotAllowed, wantStatus: 403, wantBody: `{"type":"error","error":{"type":"permission_error","message":"The requested model is not enabled for this subscription group"}}`},
		{name: "openai source unavailable", path: "/v1/chat/completions", err: service.ErrSystemCustomGroupSourceUnavailable, wantStatus: 503, wantBody: `{"error":{"type":"service_unavailable_error","message":"The selected model source is temporarily unavailable","code":"SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"}}`},
		{name: "gemini unknown alias", path: "/v1beta/models/missing:generateContent", gemini: true, err: service.ErrSystemCustomGroupModelNotAllowed, wantStatus: 403, wantBody: `{"error":{"code":403,"message":"The requested model is not enabled for this subscription group","status":"PERMISSION_DENIED"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			billingGroup := systemCustomBillingGroup(25)
			resolver := &systemCustomGroupResolverStub{err: tt.err}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := billingGroup.ID
				c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: billingGroup})
				c.Next()
			})
			if tt.gemini {
				router.Use(systemCustomGroupGeminiTargetMiddleware(resolver))
				router.POST("/v1beta/models/*modelAction", func(c *gin.Context) { t.Fatal("handler must not run") })
			} else {
				router.Use(systemCustomGroupTargetMiddleware(resolver))
				router.POST(tt.path, func(c *gin.Context) { t.Fatal("handler must not run") })
			}
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(`{"model":"missing"}`))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.JSONEq(t, tt.wantBody, recorder.Body.String())
			require.True(t, recorder.Code >= 400)
		})
	}
}

func TestSystemCustomMiddlewaresNoOpForOrdinaryKeysAndModelList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ordinaryGroup := &service.Group{ID: 3, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true}
	resolver := &systemCustomGroupResolverStub{err: errors.New("must not be called")}
	router := systemCustomTestRouter(ordinaryGroup, resolver)
	router.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Zero(t, resolver.calls)
}

func TestSystemCustomGeminiMiddlewareDoesNotResolveListModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := &systemCustomGroupResolverStub{err: errors.New("must not be called")}
	router := systemCustomAuthTestRouter(systemCustomBillingGroup(25))
	router.Use(systemCustomGroupGeminiTargetMiddleware(resolver))
	router.GET("/v1beta/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Zero(t, resolver.calls)
}

func systemCustomBillingGroup(id int64) *service.Group {
	return &service.Group{
		ID: id, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true,
		SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
}

func resolvedSystemCustomStub(billingID, sourceID int64, platform, publicModel, sourceModel string) *systemCustomGroupResolverStub {
	return &systemCustomGroupResolverStub{resolution: &service.SystemCustomGroupModelResolution{
		APIKey: &service.APIKey{GroupID: &sourceID, Group: &service.Group{ID: sourceID, Platform: platform, Status: service.StatusActive, Hydrated: true}},
		SystemCustomGroupResolution: service.SystemCustomGroupResolution{
			BillingGroupID: billingID, SourceGroupID: sourceID, PublicModel: publicModel, SourceModel: sourceModel, SourcePlatform: platform,
		},
	}}
}

func systemCustomTestRouter(group *service.Group, resolver *systemCustomGroupResolverStub) *gin.Engine {
	router := systemCustomAuthTestRouter(group)
	router.Use(systemCustomGroupTargetMiddleware(resolver))
	return router
}

func systemCustomAuthTestRouter(group *service.Group) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := group.ID
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: group})
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Next()
	})
	return router
}

func multipartBoundary(t *testing.T, contentType string) string {
	t.Helper()
	const prefix = "multipart/form-data; boundary="
	require.Contains(t, contentType, prefix)
	return contentType[len(prefix):]
}
