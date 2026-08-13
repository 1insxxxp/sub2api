//go:build unit

package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type systemCustomIntegrationSubscriptionRepo struct {
	service.UserSubscriptionRepository
	mu      sync.Mutex
	sub     *service.UserSubscription
	lookups [][2]int64
}

func (r *systemCustomIntegrationSubscriptionRepo) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, [2]int64{userID, groupID})
	copy := *r.sub
	return &copy, nil
}

func (r *systemCustomIntegrationSubscriptionRepo) subscriptionLookups() [][2]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]int64(nil), r.lookups...)
}

type systemCustomIntegrationUserRepo struct {
	service.UserRepository
	user        *service.User
	lookupCalls atomic.Int32
	walletCalls atomic.Int32
}

func (r *systemCustomIntegrationUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	r.lookupCalls.Add(1)
	if r.user == nil {
		return nil, service.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

func (r *systemCustomIntegrationUserRepo) DeductBalance(context.Context, int64, float64) error {
	r.walletCalls.Add(1)
	return nil
}

type systemCustomProductionRouteRepo struct {
	service.SystemCustomGroupRepository
	mu     sync.Mutex
	routes map[string]*service.SystemCustomGroupModel
	calls  []string
	groups []int64
}

func (r *systemCustomProductionRouteRepo) ResolveModel(_ context.Context, groupID int64, model string) (*service.SystemCustomGroupModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, model)
	r.groups = append(r.groups, groupID)
	route := r.routes[model]
	if route == nil {
		return nil, service.ErrSystemCustomGroupRouteNotFound
	}
	copy := *route
	return &copy, nil
}

func (r *systemCustomProductionRouteRepo) resolvedModels() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *systemCustomProductionRouteRepo) resolvedGroupIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.groups...)
}

type systemCustomProductionGroupRepo struct {
	service.GroupRepository
	mu     sync.Mutex
	groups map[int64]*service.Group
	calls  []int64
}

func (r *systemCustomProductionGroupRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	group := r.groups[id]
	if group == nil {
		return nil, service.ErrGroupNotFound
	}
	copy := *group
	return &copy, nil
}

func (r *systemCustomProductionGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, id)
	group := r.groups[id]
	if group == nil {
		return nil, service.ErrGroupNotFound
	}
	copy := *group
	return &copy, nil
}

func (r *systemCustomProductionGroupRepo) loadedGroupIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.calls...)
}

type systemCustomProductionAccountRepo struct {
	service.AccountRepository
	mu       sync.Mutex
	accounts map[int64][]service.Account
	listIDs  []int64
}

func (r *systemCustomProductionAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listIDs = append(r.listIDs, groupID)
	var result []service.Account
	for _, account := range r.accounts[groupID] {
		if account.Platform == platform && account.IsSchedulable() {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *systemCustomProductionAccountRepo) ListSchedulableByGroupIDAndPlatforms(_ context.Context, groupID int64, platforms []string) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listIDs = append(r.listIDs, groupID)
	allowed := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = true
	}
	var result []service.Account
	for _, account := range r.accounts[groupID] {
		if allowed[account.Platform] && account.IsSchedulable() {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *systemCustomProductionAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, _ bool) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if groupID == nil {
		return nil, nil
	}
	allowed := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = true
	}
	var result []service.Account
	for _, account := range r.accounts[*groupID] {
		if account.IsActive() && account.Schedulable && allowed[account.Platform] {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *systemCustomProductionAccountRepo) selectedGroupIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.listIDs...)
}

type systemCustomProductionUsageRepo struct {
	service.UsageLogRepository
	mu      sync.Mutex
	entries []*service.UsageLog
}

func (r *systemCustomProductionUsageRepo) Create(_ context.Context, entry *service.UsageLog) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *entry
	r.entries = append(r.entries, &copy)
	return true, nil
}

func (r *systemCustomProductionUsageRepo) logs() []*service.UsageLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*service.UsageLog(nil), r.entries...)
}

type systemCustomProductionBillingRepo struct {
	service.UsageBillingRepository
	mu      sync.Mutex
	entries []*service.UsageBillingCommand
}

func (r *systemCustomProductionBillingRepo) Apply(_ context.Context, command *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *command
	r.entries = append(r.entries, &copy)
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

func (r *systemCustomProductionBillingRepo) commands() []*service.UsageBillingCommand {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*service.UsageBillingCommand(nil), r.entries...)
}

type systemCustomProductionUpstream struct {
	mu         sync.Mutex
	accountIDs []int64
	models     []string
}

type systemCustomGeminiMessagesUpstream struct {
	mu         sync.Mutex
	accountIDs []int64
	models     []string
	url        string
	body       string
}

func (u *systemCustomGeminiMessagesUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	model := ""
	const marker = "/models/"
	if start := bytes.Index([]byte(req.URL.Path), []byte(marker)); start >= 0 {
		model = req.URL.Path[start+len(marker):]
		if end := bytes.IndexByte([]byte(model), ':'); end >= 0 {
			model = model[:end]
		}
	}
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.models = append(u.models, model)
	u.url = req.URL.String()
	u.body = string(body)
	u.mu.Unlock()

	responseBody := `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello from gemini"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":4,"totalTokenCount":16}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-gemini"}},
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Request:    req,
	}, nil
}

func (u *systemCustomGeminiMessagesUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *systemCustomGeminiMessagesUpstream) calls() ([]int64, []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...), append([]string(nil), u.models...)
}

func (u *systemCustomGeminiMessagesUpstream) lastURL() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.url
}

func (u *systemCustomGeminiMessagesUpstream) lastBody() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.body
}

func (u *systemCustomProductionUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	model := gjson.GetBytes(body, "model").String()
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.models = append(u.models, model)
	requestNumber := len(u.models)
	u.mu.Unlock()
	responseBody, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-production-" + model,
		"object":  "chat.completion",
		"created": requestNumber,
		"model":   model,
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-" + model}},
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
		Request:    req,
	}, nil
}

func (u *systemCustomProductionUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *systemCustomProductionUpstream) calls() ([]int64, []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...), append([]string(nil), u.models...)
}

type systemCustomProductionChannelRepo struct {
	service.ChannelRepository
	channels  []service.Channel
	platforms map[int64]string
}

func (r *systemCustomProductionChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	return append([]service.Channel(nil), r.channels...), nil
}

func (r *systemCustomProductionChannelRepo) GetGroupPlatforms(_ context.Context, _ []int64) (map[int64]string, error) {
	return r.platforms, nil
}

type systemCustomProductionConcurrencyCache struct {
	service.ConcurrencyCache
}

func (*systemCustomProductionConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (*systemCustomProductionConcurrencyCache) ReleaseAccountSlot(context.Context, int64, string) error {
	return nil
}
func (*systemCustomProductionConcurrencyCache) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (*systemCustomProductionConcurrencyCache) ReleaseUserSlot(context.Context, int64, string) error {
	return nil
}

type systemCustomProductionHarness struct {
	router       *gin.Engine
	key          string
	billingCache *service.BillingCacheService
	billingGroup *service.Group
	subscription *service.UserSubscription
	subRepo      *systemCustomIntegrationSubscriptionRepo
	routeRepo    *systemCustomProductionRouteRepo
	groupRepo    *systemCustomProductionGroupRepo
	accountRepo  *systemCustomProductionAccountRepo
	usageRepo    *systemCustomProductionUsageRepo
	billingRepo  *systemCustomProductionBillingRepo
	userRepo     *systemCustomIntegrationUserRepo
	upstream     *systemCustomProductionUpstream
}

type systemCustomGeminiMessagesProductionHarness struct {
	router       *gin.Engine
	key          string
	billingCache *service.BillingCacheService
	routeRepo    *systemCustomProductionRouteRepo
	accountRepo  *systemCustomProductionAccountRepo
	usageRepo    *systemCustomProductionUsageRepo
	billingRepo  *systemCustomProductionBillingRepo
	userRepo     *systemCustomIntegrationUserRepo
	upstream     *systemCustomGeminiMessagesUpstream
}

func newSystemCustomProductionHarness(t *testing.T) *systemCustomProductionHarness {
	t.Helper()
	const (
		billingGroupID = int64(25)
		sourceAID      = int64(42)
		sourceBID      = int64(43)
		userID         = int64(9)
	)
	inputA, outputA := 0.001, 0.002
	inputB, outputB := 0.003, 0.004
	groupA := &service.Group{ID: sourceAID, Name: "source-a", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1}
	groupB := &service.Group{ID: sourceBID, Name: "source-b", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1.5}
	billingGroup := &service.Group{
		ID: billingGroupID, Name: "tavern monthly", Platform: service.PlatformComposite,
		Status: service.StatusActive, Hydrated: true, RateMultiplier: 1,
		SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	routeRepo := &systemCustomProductionRouteRepo{routes: map[string]*service.SystemCustomGroupModel{
		"tavern-a": {GroupID: billingGroupID, PublicModel: "tavern-a", SourceGroupID: sourceAID, SourceModel: "source-a-model", Enabled: true},
		"tavern-b": {GroupID: billingGroupID, PublicModel: "tavern-b", SourceGroupID: sourceBID, SourceModel: "source-b-model", Enabled: true},
	}}
	groupRepo := &systemCustomProductionGroupRepo{groups: map[int64]*service.Group{sourceAID: groupA, sourceBID: groupB}}
	accountRepo := &systemCustomProductionAccountRepo{accounts: map[int64][]service.Account{
		sourceAID: {newSystemCustomProductionAccount(501, sourceAID, "source-a-model")},
		sourceBID: {newSystemCustomProductionAccount(502, sourceBID, "source-b-model")},
	}}
	channelRepo := &systemCustomProductionChannelRepo{
		channels: []service.Channel{
			{ID: 1001, Status: service.StatusActive, GroupIDs: []int64{sourceAID}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"source-a-model"}, BillingMode: service.BillingModeToken, InputPrice: &inputA, OutputPrice: &outputA}}},
			{ID: 1002, Status: service.StatusActive, GroupIDs: []int64{sourceBID}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"source-b-model"}, BillingMode: service.BillingModeToken, InputPrice: &inputB, OutputPrice: &outputB}}},
		},
		platforms: map[int64]string{sourceAID: service.PlatformOpenAI, sourceBID: service.PlatformOpenAI},
	}
	now := time.Now()
	subscription := &service.UserSubscription{
		ID: 88, UserID: userID, GroupID: billingGroupID, Group: billingGroup,
		Status: service.SubscriptionStatusActive, StartsAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour),
		DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
	}
	user := &service.User{ID: userID, Role: service.RoleUser, Status: service.StatusActive, Balance: 999}
	apiKey := &service.APIKey{ID: 7, UserID: userID, Key: "system-custom-production-key", Status: service.StatusActive, GroupID: &billingGroup.ID, Group: billingGroup, User: user}
	authRepo := &systemCustomUnsupportedAuthRepo{apiKey: apiKey}
	subRepo := &systemCustomIntegrationSubscriptionRepo{sub: subscription}
	usageRepo := &systemCustomProductionUsageRepo{}
	billingRepo := &systemCustomProductionBillingRepo{}
	userRepo := &systemCustomIntegrationUserRepo{user: user}
	upstream := &systemCustomProductionUpstream{}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.MaxBodySize = 1024 * 1024
	cfg.Gateway.TextMaxBodySize = 1024 * 1024
	apiKeyService := service.NewAPIKeyService(authRepo, nil, groupRepo, nil, nil, nil, cfg)
	apiKeyService.SetSystemCustomGroupRepository(routeRepo)
	subscriptionService := service.NewSubscriptionService(nil, subRepo, nil, nil, cfg)
	billingCache := service.NewBillingCacheService(nil, userRepo, subRepo, authRepo, nil, nil, cfg, nil)
	concurrencyService := service.NewConcurrencyService(&systemCustomProductionConcurrencyCache{})
	billingService := service.NewBillingService(cfg, nil)
	channelService := service.NewChannelService(channelRepo, groupRepo, nil, nil)
	pricingResolver := service.NewModelPricingResolver(channelService, billingService)
	gateway := service.NewOpenAIGatewayService(
		accountRepo, usageRepo, billingRepo, userRepo, subRepo, nil, nil, cfg, nil, concurrencyService,
		billingService, nil, billingCache, upstream, &service.DeferredService{}, nil, nil,
		pricingResolver, channelService, nil, nil, nil,
	)
	gatewayHandler := handler.NewOpenAIGatewayHandler(gateway, concurrencyService, billingCache, apiKeyService, nil, nil, nil, nil, cfg)
	router := gin.New()
	router.Use(gin.HandlerFunc(servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg)))
	router.Use(systemCustomGroupTargetMiddleware(apiKeyService))
	router.POST("/v1/chat/completions", gatewayHandler.ChatCompletions)
	return &systemCustomProductionHarness{
		router: router, key: apiKey.Key, billingCache: billingCache, billingGroup: billingGroup, subscription: subscription, subRepo: subRepo,
		routeRepo: routeRepo, groupRepo: groupRepo, accountRepo: accountRepo,
		usageRepo: usageRepo, billingRepo: billingRepo, userRepo: userRepo, upstream: upstream,
	}
}

func newSystemCustomGeminiMessagesProductionHarness(t *testing.T) *systemCustomGeminiMessagesProductionHarness {
	t.Helper()
	const (
		billingGroupID = int64(25)
		sourceGroupID  = int64(42)
		userID         = int64(9)
	)
	inputPrice, outputPrice := 0.001, 0.002
	sourceGroup := &service.Group{
		ID: sourceGroupID, Name: "gemini-source", Platform: service.PlatformGemini,
		Status: service.StatusActive, Hydrated: true, RateMultiplier: 1,
	}
	billingGroup := &service.Group{
		ID: billingGroupID, Name: "tavern monthly", Platform: service.PlatformComposite,
		Status: service.StatusActive, Hydrated: true, RateMultiplier: 1,
		SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	routeRepo := &systemCustomProductionRouteRepo{routes: map[string]*service.SystemCustomGroupModel{
		"tavern-gemini": {
			GroupID: billingGroupID, PublicModel: "tavern-gemini", SourceGroupID: sourceGroupID,
			SourceModel: "gemini-2.5-flash", Enabled: true,
		},
	}}
	groupRepo := &systemCustomProductionGroupRepo{groups: map[int64]*service.Group{sourceGroupID: sourceGroup}}
	accountRepo := &systemCustomProductionAccountRepo{accounts: map[int64][]service.Account{
		sourceGroupID: {newSystemCustomGeminiProductionAccount(601, sourceGroupID, "gemini-2.5-flash")},
	}}
	channelRepo := &systemCustomProductionChannelRepo{
		channels: []service.Channel{{
			ID: 2001, Status: service.StatusActive, GroupIDs: []int64{sourceGroupID},
			ModelPricing: []service.ChannelModelPricing{{
				Platform: service.PlatformGemini, Models: []string{"gemini-2.5-flash"},
				BillingMode: service.BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice,
			}},
		}},
		platforms: map[int64]string{sourceGroupID: service.PlatformGemini},
	}
	now := time.Now()
	subscription := &service.UserSubscription{
		ID: 88, UserID: userID, GroupID: billingGroupID, Group: billingGroup,
		Status: service.SubscriptionStatusActive, StartsAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour),
		DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
	}
	user := &service.User{ID: userID, Role: service.RoleUser, Status: service.StatusActive, Balance: 999}
	apiKey := &service.APIKey{
		ID: 7, UserID: userID, Key: "system-custom-gemini-messages-key", Status: service.StatusActive,
		GroupID: &billingGroup.ID, Group: billingGroup, User: user,
	}
	authRepo := &systemCustomUnsupportedAuthRepo{apiKey: apiKey}
	subRepo := &systemCustomIntegrationSubscriptionRepo{sub: subscription}
	usageRepo := &systemCustomProductionUsageRepo{}
	billingRepo := &systemCustomProductionBillingRepo{}
	userRepo := &systemCustomIntegrationUserRepo{user: user}
	upstream := &systemCustomGeminiMessagesUpstream{}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.MaxBodySize = 1024 * 1024
	cfg.Gateway.TextMaxBodySize = 1024 * 1024
	apiKeyService := service.NewAPIKeyService(authRepo, nil, groupRepo, nil, nil, nil, cfg)
	apiKeyService.SetSystemCustomGroupRepository(routeRepo)
	subscriptionService := service.NewSubscriptionService(nil, subRepo, nil, nil, cfg)
	billingCache := service.NewBillingCacheService(nil, userRepo, subRepo, authRepo, nil, nil, cfg, nil)
	concurrencyService := service.NewConcurrencyService(&systemCustomProductionConcurrencyCache{})
	billingService := service.NewBillingService(cfg, nil)
	channelService := service.NewChannelService(channelRepo, groupRepo, nil, nil)
	pricingResolver := service.NewModelPricingResolver(channelService, billingService)
	gateway := service.NewGatewayService(
		accountRepo, groupRepo, usageRepo, billingRepo, userRepo, subRepo, nil, nil, cfg, nil,
		concurrencyService, billingService, nil, billingCache, nil, upstream, &service.DeferredService{},
		nil, nil, nil, nil, nil, nil, channelService, pricingResolver, nil, nil, nil,
	)
	geminiCompat := service.NewGeminiMessagesCompatService(
		accountRepo, groupRepo, nil, nil, nil, nil, upstream, nil, cfg,
	)
	gatewayHandler := handler.NewGatewayHandler(
		gateway, nil, geminiCompat, nil, nil, concurrencyService, billingCache, nil,
		apiKeyService, nil, nil, nil, nil, cfg, nil,
	)
	router := gin.New()
	router.Use(gin.HandlerFunc(servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg)))
	router.Use(systemCustomGroupTargetMiddleware(apiKeyService))
	router.POST("/v1/messages", gatewayHandler.Messages)
	return &systemCustomGeminiMessagesProductionHarness{
		router: router, key: apiKey.Key, billingCache: billingCache, routeRepo: routeRepo,
		accountRepo: accountRepo, usageRepo: usageRepo, billingRepo: billingRepo,
		userRepo: userRepo, upstream: upstream,
	}
}

func newSystemCustomProductionAccount(id, groupID int64, model string) service.Account {
	return service.Account{
		ID: id, Name: model, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{groupID}, RateMultiplier: float64Pointer(1),
		Credentials: map[string]any{"api_key": "upstream-key", "base_url": "https://upstream.invalid", "model_mapping": map[string]any{model: model}},
		Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}
}

func newSystemCustomGeminiProductionAccount(id, groupID int64, model string) service.Account {
	return service.Account{
		ID: id, Name: model, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{groupID},
		RateMultiplier: float64Pointer(1), Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "gemini-upstream-key", "base_url": "https://gemini.invalid",
			"model_mapping": map[string]any{model: model},
		},
	}
}

func (h *systemCustomProductionHarness) close() {
	if h != nil && h.billingCache != nil {
		h.billingCache.Stop()
	}
}

func (h *systemCustomGeminiMessagesProductionHarness) close() {
	if h != nil && h.billingCache != nil {
		h.billingCache.Stop()
	}
}

func float64Pointer(value float64) *float64 { return &value }

func TestSystemCustomSubscriptionOpenAIProductionChainBillsTwoSources(t *testing.T) {
	harness := newSystemCustomProductionHarness(t)
	defer harness.close()

	publicModels := []string{"tavern-a", "tavern-b"}
	for _, model := range publicModels {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+model+`","messages":[{"role":"user","content":"hello"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+harness.key)
		harness.router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}

	commands := harness.billingRepo.commands()
	logs := harness.usageRepo.logs()
	require.Len(t, commands, 2)
	require.Len(t, logs, 2)
	require.Equal(t, publicModels, harness.routeRepo.resolvedModels())
	require.Equal(t, []int64{25, 25}, harness.routeRepo.resolvedGroupIDs(), "public aliases must resolve inside the billing container")
	requireSystemCustomSubscriptionLookups(t, harness.subRepo.subscriptionLookups())
	require.Equal(t, []int64{42, 43}, harness.accountRepo.selectedGroupIDs(), "the production scheduler must select only inside each resolved source group")
	accountIDs, upstreamModels := harness.upstream.calls()
	require.Equal(t, []int64{501, 502}, accountIDs)
	require.Equal(t, []string{"source-a-model", "source-b-model"}, upstreamModels)

	wantCosts := []float64{0.14, 0.57}
	wantSourceGroups := []int64{42, 43}
	wantSourceModels := []string{"source-a-model", "source-b-model"}
	var appliedTotal float64
	for i := range commands {
		require.NotNil(t, commands[i].SubscriptionID)
		require.Equal(t, int64(88), *commands[i].SubscriptionID, "both source routes must consume the same active subscription")
		require.InDelta(t, wantCosts[i], commands[i].SubscriptionCost, 1e-9)
		require.Zero(t, commands[i].BalanceCost)
		appliedTotal += commands[i].SubscriptionCost

		require.NotNil(t, logs[i].GroupID)
		require.Equal(t, int64(25), *logs[i].GroupID)
		require.NotNil(t, logs[i].SourceGroupID)
		require.Equal(t, wantSourceGroups[i], *logs[i].SourceGroupID)
		require.NotNil(t, logs[i].SubscriptionID)
		require.Equal(t, int64(88), *logs[i].SubscriptionID)
		require.Equal(t, publicModels[i], logs[i].RequestedModel)
		require.Equal(t, wantSourceModels[i], logs[i].Model)
		require.NotNil(t, logs[i].UpstreamModel)
		require.Equal(t, wantSourceModels[i], *logs[i].UpstreamModel)
		require.NotNil(t, logs[i].ModelMappingChain)
		require.Equal(t, publicModels[i]+"→"+wantSourceModels[i], *logs[i].ModelMappingChain)
		require.InDelta(t, wantCosts[i], logs[i].ActualCost, 1e-9)
	}
	require.InDelta(t, 0.71, appliedTotal, 1e-9)
	require.Zero(t, harness.userRepo.walletCalls.Load())
}

func TestSystemCustomSubscriptionClaudeMessagesRoutesToGeminiSource(t *testing.T) {
	harness := newSystemCustomGeminiMessagesProductionHarness(t)
	defer harness.close()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/messages",
		bytes.NewBufferString(`{"model":"tavern-gemini","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+harness.key)
	harness.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "message", gjson.Get(recorder.Body.String(), "type").String())
	require.Equal(t, "hello from gemini", gjson.Get(recorder.Body.String(), "content.0.text").String())
	require.Equal(t, []string{"tavern-gemini"}, harness.routeRepo.resolvedModels())
	selectedGroupIDs := harness.accountRepo.selectedGroupIDs()
	require.NotEmpty(t, selectedGroupIDs)
	for _, groupID := range selectedGroupIDs {
		require.Equal(t, int64(42), groupID, "every scheduler lookup must stay inside the resolved Gemini source")
	}

	accountIDs, upstreamModels := harness.upstream.calls()
	require.Equal(t, []int64{601}, accountIDs)
	require.Equal(t, []string{"gemini-2.5-flash"}, upstreamModels)
	require.Contains(t, harness.upstream.lastURL(), "/models/gemini-2.5-flash:generateContent")
	require.Contains(t, harness.upstream.lastBody(), `"contents"`)

	commands := harness.billingRepo.commands()
	logs := harness.usageRepo.logs()
	require.Len(t, commands, 1)
	require.Len(t, logs, 1)
	require.NotNil(t, commands[0].SubscriptionID)
	require.Equal(t, int64(88), *commands[0].SubscriptionID)
	require.Zero(t, commands[0].BalanceCost)
	require.Positive(t, commands[0].SubscriptionCost)
	require.NotNil(t, logs[0].GroupID)
	require.Equal(t, int64(25), *logs[0].GroupID)
	require.NotNil(t, logs[0].SourceGroupID)
	require.Equal(t, int64(42), *logs[0].SourceGroupID)
	require.Equal(t, "tavern-gemini", logs[0].RequestedModel)
	require.Equal(t, "gemini-2.5-flash", logs[0].Model)
	require.Zero(t, harness.userRepo.walletCalls.Load())
}

func TestSystemCustomSubscriptionClosedFailureTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		configure      func(*systemCustomProductionHarness)
		wantStatus     int
		wantType       string
		wantCode       string
		wantMessage    string
		wantScheduling bool
	}{
		{name: "unknown public alias", configure: func(h *systemCustomProductionHarness) { delete(h.routeRepo.routes, "tavern-a") }, wantStatus: http.StatusForbidden, wantType: "permission_error", wantCode: "SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED", wantMessage: "The requested model is not enabled for this subscription group"},
		{name: "disabled route", configure: func(h *systemCustomProductionHarness) { h.routeRepo.routes["tavern-a"].Enabled = false }, wantStatus: http.StatusForbidden, wantType: "permission_error", wantCode: "SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED", wantMessage: "The requested model is not enabled for this subscription group"},
		{name: "disabled source", configure: func(h *systemCustomProductionHarness) { h.groupRepo.groups[42].Status = service.StatusDisabled }, wantStatus: http.StatusServiceUnavailable, wantType: "service_unavailable_error", wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE", wantMessage: "The selected model source is temporarily unavailable"},
		{name: "deleted source", configure: func(h *systemCustomProductionHarness) { delete(h.groupRepo.groups, 42) }, wantStatus: http.StatusServiceUnavailable, wantType: "service_unavailable_error", wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE", wantMessage: "The selected model source is temporarily unavailable"},
		{name: "removed source model", configure: func(h *systemCustomProductionHarness) {
			h.accountRepo.accounts[42] = []service.Account{newSystemCustomProductionAccount(501, 42, "another-model")}
		}, wantStatus: http.StatusNotFound, wantType: "model_not_found", wantMessage: `Model "source-a-model" is not supported by any configured account in this group`, wantScheduling: true},
		{name: "no available account", configure: func(h *systemCustomProductionHarness) {
			h.accountRepo.accounts[42] = nil
		}, wantStatus: http.StatusServiceUnavailable, wantType: "api_error", wantMessage: "Service temporarily unavailable", wantScheduling: true},
		{name: "missing pricing", configure: func(h *systemCustomProductionHarness) {
			h.routeRepo.routes["tavern-a"].SourceModel = "unpriced-system-custom-integration-model"
			h.accountRepo.accounts[42] = []service.Account{newSystemCustomProductionAccount(501, 42, "unpriced-system-custom-integration-model")}
		}, wantStatus: http.StatusServiceUnavailable, wantType: "api_error", wantMessage: "Service temporarily unavailable", wantScheduling: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newSystemCustomProductionHarness(t)
			defer harness.close()
			tt.configure(harness)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"tavern-a","messages":[{"role":"user","content":"hello"}]}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+harness.key)
			harness.router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, tt.wantType, gjson.Get(recorder.Body.String(), "error.type").String(), recorder.Body.String())
			require.Equal(t, tt.wantMessage, gjson.Get(recorder.Body.String(), "error.message").String(), recorder.Body.String())
			if tt.wantCode != "" {
				require.Equal(t, tt.wantCode, gjson.Get(recorder.Body.String(), "error.code").String(), recorder.Body.String())
			}
			accountIDs, upstreamModels := harness.upstream.calls()
			require.Empty(t, accountIDs, "closed failures must not call any upstream or switch source")
			require.Empty(t, upstreamModels)
			require.Empty(t, harness.usageRepo.logs(), "failures must not create a zero-price usage row")
			require.Empty(t, harness.billingRepo.commands(), "failures must not apply subscription or balance billing")
			require.Zero(t, harness.userRepo.walletCalls.Load(), "system custom failures must never fall back to wallet balance")
			require.Equal(t, []string{"tavern-a"}, harness.routeRepo.resolvedModels())
			require.Equal(t, []int64{25}, harness.routeRepo.resolvedGroupIDs())
			requireSystemCustomSubscriptionLookups(t, harness.subRepo.subscriptionLookups())
			if tt.wantScheduling {
				for _, groupID := range harness.accountRepo.selectedGroupIDs() {
					require.Equal(t, int64(42), groupID, "scheduler must not switch away from the resolved source group")
				}
				require.NotEmpty(t, harness.accountRepo.selectedGroupIDs(), "failure must be produced by the real scheduler")
			} else {
				require.Empty(t, harness.accountRepo.selectedGroupIDs())
			}
		})
	}
}

func TestSystemCustomSubscriptionQuotaStopsBeforeResolverAndUpstream(t *testing.T) {
	limit := 10.0
	tests := []struct {
		name      string
		configure func(*service.Group, *service.UserSubscription)
		message   string
	}{
		{name: "daily", message: service.ErrDailyLimitExceeded.Error(), configure: func(group *service.Group, sub *service.UserSubscription) {
			group.DailyLimitUSD = &limit
			sub.DailyUsageUSD = limit + 0.01
		}},
		{name: "monthly", message: service.ErrMonthlyLimitExceeded.Error(), configure: func(group *service.Group, sub *service.UserSubscription) {
			group.MonthlyLimitUSD = &limit
			sub.MonthlyUsageUSD = limit + 0.01
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newSystemCustomProductionHarness(t)
			defer harness.close()
			tt.configure(harness.billingGroup, harness.subscription)

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"tavern-a","messages":[{"role":"user","content":"hello"}]}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+harness.key)
			harness.router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusTooManyRequests, recorder.Code)
			require.Equal(t, "USAGE_LIMIT_EXCEEDED", gjson.Get(recorder.Body.String(), "code").String())
			require.Equal(t, tt.message, gjson.Get(recorder.Body.String(), "message").String())
			require.Empty(t, harness.routeRepo.resolvedModels(), "auth quota must fail before the real system resolver middleware")
			require.Empty(t, harness.routeRepo.resolvedGroupIDs())
			require.Empty(t, harness.groupRepo.loadedGroupIDs())
			require.Empty(t, harness.accountRepo.selectedGroupIDs())
			accountIDs, upstreamModels := harness.upstream.calls()
			require.Empty(t, accountIDs)
			require.Empty(t, upstreamModels)
			require.Empty(t, harness.usageRepo.logs())
			require.Empty(t, harness.billingRepo.commands())
			requireSystemCustomSubscriptionLookups(t, harness.subRepo.subscriptionLookups())
			require.Zero(t, harness.userRepo.lookupCalls.Load(), "subscription quota failure must not fall back to balance lookup")
			require.Zero(t, harness.userRepo.walletCalls.Load())
		})
	}
}

func requireSystemCustomSubscriptionLookups(t *testing.T, lookups [][2]int64) {
	t.Helper()
	require.NotEmpty(t, lookups)
	for _, lookup := range lookups {
		require.Equal(t, [2]int64{9, 25}, lookup, "subscription identity must stay on the authenticated user and billing container")
	}
}
