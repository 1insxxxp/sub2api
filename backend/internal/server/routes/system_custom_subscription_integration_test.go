//go:build unit

package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type systemCustomIntegrationRouteRepo struct {
	service.SystemCustomGroupRepository
	route       *service.SystemCustomGroupModel
	err         error
	calls       atomic.Int32
	lastGroupID atomic.Int64
	lastModel   string
}

func (r *systemCustomIntegrationRouteRepo) ResolveModel(_ context.Context, groupID int64, model string) (*service.SystemCustomGroupModel, error) {
	r.calls.Add(1)
	r.lastGroupID.Store(groupID)
	r.lastModel = model
	return r.route, r.err
}

type systemCustomIntegrationGroupRepo struct {
	service.GroupRepository
	group *service.Group
	err   error
	calls atomic.Int32
}

func (r *systemCustomIntegrationGroupRepo) GetByIDLite(_ context.Context, _ int64) (*service.Group, error) {
	r.calls.Add(1)
	if r.err != nil {
		return nil, r.err
	}
	if r.group == nil {
		return nil, service.ErrGroupNotFound
	}
	copy := *r.group
	return &copy, nil
}

type systemCustomIntegrationSubscriptionRepo struct {
	service.UserSubscriptionRepository
	sub *service.UserSubscription
}

func (r *systemCustomIntegrationSubscriptionRepo) GetActiveByUserIDAndGroupID(_ context.Context, _, _ int64) (*service.UserSubscription, error) {
	copy := *r.sub
	return &copy, nil
}

type systemCustomIntegrationAccountRepo struct {
	service.AccountRepository
	accounts     []service.Account
	listCalls    atomic.Int32
	lastGroupID  atomic.Int64
	lastPlatform string
}

func (r *systemCustomIntegrationAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	r.listCalls.Add(1)
	r.lastGroupID.Store(groupID)
	r.lastPlatform = platform
	return append([]service.Account(nil), r.accounts...), nil
}

type systemCustomIntegrationUsageRepo struct {
	service.UsageLogRepository
	calls atomic.Int32
}

func (r *systemCustomIntegrationUsageRepo) Create(context.Context, *service.UsageLog) (bool, error) {
	r.calls.Add(1)
	return true, nil
}

type systemCustomIntegrationBillingRepo struct {
	service.UsageBillingRepository
	calls atomic.Int32
}

func (r *systemCustomIntegrationBillingRepo) Apply(context.Context, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.calls.Add(1)
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

type systemCustomIntegrationUserRepo struct {
	service.UserRepository
	walletCalls atomic.Int32
}

func (r *systemCustomIntegrationUserRepo) DeductBalance(context.Context, int64, float64) error {
	r.walletCalls.Add(1)
	return nil
}

func TestSystemCustomSubscriptionClosedFailureTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		billingGroupID = int64(25)
		sourceGroupID  = int64(42)
	)
	activeSource := &service.Group{ID: sourceGroupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1}
	enabledRoute := &service.SystemCustomGroupModel{
		GroupID: billingGroupID, PublicModel: "tavern-gpt", SourceGroupID: sourceGroupID,
		SourceModel: "gpt-5.4-mini", Enabled: true,
	}
	tests := []struct {
		name       string
		route      *service.SystemCustomGroupModel
		routeErr   error
		source     *service.Group
		sourceErr  error
		accounts   []service.Account
		handler    string
		wantStatus int
		wantCode   string
	}{
		{name: "unknown public alias", routeErr: service.ErrSystemCustomGroupRouteNotFound, source: activeSource, wantStatus: http.StatusForbidden, wantCode: "SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED"},
		{name: "disabled route", route: &service.SystemCustomGroupModel{GroupID: billingGroupID, PublicModel: "tavern-gpt", SourceGroupID: sourceGroupID, SourceModel: "gpt-5.4-mini"}, source: activeSource, wantStatus: http.StatusForbidden, wantCode: "SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED"},
		{name: "disabled source", route: enabledRoute, source: &service.Group{ID: sourceGroupID, Platform: service.PlatformOpenAI, Status: service.StatusDisabled, Hydrated: true}, wantStatus: http.StatusServiceUnavailable, wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"},
		{name: "deleted source", route: enabledRoute, sourceErr: service.ErrGroupNotFound, wantStatus: http.StatusServiceUnavailable, wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"},
		{name: "removed source model", route: enabledRoute, source: activeSource, handler: "scheduler", accounts: []service.Account{{
			ID: 501, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-other": "gpt-other"}},
		}}, wantStatus: http.StatusServiceUnavailable, wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"},
		{name: "no available account", route: enabledRoute, source: activeSource, handler: "scheduler", wantStatus: http.StatusServiceUnavailable, wantCode: "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE"},
		{name: "missing pricing", route: &service.SystemCustomGroupModel{
			GroupID: billingGroupID, PublicModel: "tavern-unpriced", SourceGroupID: sourceGroupID,
			SourceModel: "unpriced-system-custom-integration-model", Enabled: true,
		}, source: activeSource, handler: "record_usage", wantStatus: http.StatusInternalServerError, wantCode: "MODEL_PRICING_UNAVAILABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routeRepo := &systemCustomIntegrationRouteRepo{route: tt.route, err: tt.routeErr}
			groupRepo := &systemCustomIntegrationGroupRepo{group: tt.source, err: tt.sourceErr}
			accountRepo := &systemCustomIntegrationAccountRepo{accounts: tt.accounts}
			usageRepo := &systemCustomIntegrationUsageRepo{}
			billingRepo := &systemCustomIntegrationBillingRepo{}
			userRepo := &systemCustomIntegrationUserRepo{}
			var upstreamCalls atomic.Int32

			router, key := newSystemCustomIntegrationRouter(t, routeRepo, groupRepo, nil, func(c *gin.Context) {
				apiKey, ok := servermiddleware.GetAPIKeyFromContext(c)
				require.True(t, ok)
				switch tt.handler {
				case "scheduler":
					gateway := newSystemCustomIntegrationOpenAIGateway(accountRepo, nil, nil, nil, nil)
					_, err := gateway.SelectAccountForModel(c.Request.Context(), apiKey.GroupID, "", tt.route.SourceModel)
					require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
					writeSystemCustomRootError(c, http.StatusServiceUnavailable, "service_unavailable_error", "SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE", "The selected model source is temporarily unavailable")
				case "record_usage":
					gateway := newSystemCustomIntegrationOpenAIGateway(nil, usageRepo, billingRepo, userRepo, service.NewBillingService(&config.Config{}, nil))
					subscription, ok := servermiddleware.GetSubscriptionFromContext(c)
					require.True(t, ok)
					err := gateway.RecordUsage(c.Request.Context(), &service.OpenAIRecordUsageInput{
						Result: &service.OpenAIForwardResult{
							RequestID: "unpriced", Model: tt.route.SourceModel, BillingModel: tt.route.SourceModel,
							UpstreamModel: tt.route.SourceModel, Usage: service.OpenAIUsage{InputTokens: 100},
						},
						APIKey: apiKey, User: apiKey.User, Account: &service.Account{ID: 501, Platform: service.PlatformOpenAI}, Subscription: subscription,
					})
					require.ErrorIs(t, err, service.ErrModelPricingUnavailable)
					writeSystemCustomRootError(c, http.StatusInternalServerError, "api_error", "MODEL_PRICING_UNAVAILABLE", "Model pricing is unavailable")
				default:
					upstreamCalls.Add(1)
					c.Status(http.StatusNoContent)
				}
			})

			model := "tavern-gpt"
			if tt.route != nil {
				model = tt.route.PublicModel
			}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+model+`"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+key)
			router.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, tt.wantCode, gjson.Get(recorder.Body.String(), "error.code").String())
			require.Zero(t, upstreamCalls.Load())
			require.Zero(t, usageRepo.calls.Load(), "failures must not create a zero-price usage row")
			require.Zero(t, billingRepo.calls.Load(), "failures must not apply subscription or balance billing")
			require.Zero(t, userRepo.walletCalls.Load(), "system custom failures must never fall back to wallet balance")
			require.Equal(t, int32(1), routeRepo.calls.Load(), "one public alias must resolve to exactly one configured route")
			require.Equal(t, billingGroupID, routeRepo.lastGroupID.Load(), "public aliases are scoped to the billing container")
			require.Equal(t, model, routeRepo.lastModel)
			if tt.name == "unknown public alias" || tt.name == "disabled route" {
				require.Zero(t, groupRepo.calls.Load(), "rejected routes must not inspect or switch to a source group")
			} else {
				require.Equal(t, int32(1), groupRepo.calls.Load(), "the selected source must be loaded once without fallback lookup")
			}
			if tt.handler == "scheduler" {
				require.Equal(t, int32(1), accountRepo.listCalls.Load(), "scheduler must inspect only the resolved source group once")
				require.Equal(t, sourceGroupID, accountRepo.lastGroupID.Load())
				require.Equal(t, service.PlatformOpenAI, accountRepo.lastPlatform)
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
			routeRepo := &systemCustomIntegrationRouteRepo{route: &service.SystemCustomGroupModel{
				GroupID: 25, PublicModel: "tavern-gpt", SourceGroupID: 42, SourceModel: "gpt-5.4-mini", Enabled: true,
			}}
			groupRepo := &systemCustomIntegrationGroupRepo{group: &service.Group{
				ID: 42, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true,
			}}
			var upstreamCalls atomic.Int32
			router, key := newSystemCustomIntegrationRouter(t, routeRepo, groupRepo, tt.configure, func(c *gin.Context) {
				upstreamCalls.Add(1)
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"tavern-gpt"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+key)
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusTooManyRequests, recorder.Code)
			require.Equal(t, "USAGE_LIMIT_EXCEEDED", gjson.Get(recorder.Body.String(), "code").String())
			require.Equal(t, tt.message, gjson.Get(recorder.Body.String(), "message").String())
			require.Zero(t, routeRepo.calls.Load(), "auth quota must fail before the real system resolver middleware")
			require.Zero(t, groupRepo.calls.Load())
			require.Zero(t, upstreamCalls.Load())
		})
	}
}

func newSystemCustomIntegrationRouter(
	t *testing.T,
	routeRepo service.SystemCustomGroupRepository,
	groupRepo service.GroupRepository,
	configure func(*service.Group, *service.UserSubscription),
	next gin.HandlerFunc,
) (*gin.Engine, string) {
	t.Helper()
	const (
		billingGroupID = int64(25)
		userID         = int64(9)
	)
	billingGroup := &service.Group{
		ID: billingGroupID, Name: "tavern monthly", Platform: service.PlatformComposite,
		Status: service.StatusActive, Hydrated: true,
		SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	user := &service.User{ID: userID, Role: service.RoleUser, Status: service.StatusActive, Balance: 999, Concurrency: 3}
	now := time.Now()
	subscription := &service.UserSubscription{
		ID: 88, UserID: userID, GroupID: billingGroupID, Group: billingGroup,
		Status: service.SubscriptionStatusActive, StartsAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour),
		DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
	}
	if configure != nil {
		configure(billingGroup, subscription)
	}
	apiKey := &service.APIKey{
		ID: 7, UserID: userID, Key: "system-custom-integration-key", Status: service.StatusActive,
		GroupID: &billingGroup.ID, Group: billingGroup, User: user,
	}
	authRepo := &systemCustomUnsupportedAuthRepo{apiKey: apiKey}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(authRepo, nil, groupRepo, nil, nil, nil, cfg)
	apiKeyService.SetSystemCustomGroupRepository(routeRepo)
	subscriptionService := service.NewSubscriptionService(nil, &systemCustomIntegrationSubscriptionRepo{sub: subscription}, nil, nil, cfg)

	router := gin.New()
	router.Use(gin.HandlerFunc(servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg)))
	router.Use(systemCustomGroupTargetMiddleware(apiKeyService))
	router.POST("/v1/chat/completions", next)
	return router, apiKey.Key
}

func newSystemCustomIntegrationOpenAIGateway(
	accountRepo service.AccountRepository,
	usageRepo service.UsageLogRepository,
	billingRepo service.UsageBillingRepository,
	userRepo service.UserRepository,
	billingService *service.BillingService,
) *service.OpenAIGatewayService {
	cfg := &config.Config{RunMode: config.RunModeStandard}
	return service.NewOpenAIGatewayService(
		accountRepo,
		usageRepo,
		billingRepo,
		userRepo,
		nil, // userSubRepo
		nil, // userGroupRateRepo
		nil, // cache
		cfg,
		nil, // schedulerSnapshot
		nil, // concurrencyService
		billingService,
		nil, // rateLimitService
		nil, // billingCacheService
		nil, // httpUpstream
		nil, // deferredService
		nil, // openAITokenProvider
		nil, // grokTokenProvider
		nil, // resolver
		nil, // channelService
		nil, // balanceNotifyService
		nil, // settingService
		nil, // userPlatformQuotaRepo
	)
}
