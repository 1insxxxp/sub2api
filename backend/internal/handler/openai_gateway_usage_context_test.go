package handler

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func systemCustomWorkerParentContext() (context.Context, service.SystemCustomGroupResolution) {
	resolution := service.SystemCustomGroupResolution{
		BillingGroupID: 101,
		SourceGroupID:  202,
		PublicModel:    "tavern-sonnet",
		SourceModel:    "claude-sonnet-4",
		SourcePlatform: service.PlatformAnthropic,
	}
	ctx := service.WithSystemCustomGroupResolution(context.Background(), resolution)
	ctx = context.WithValue(ctx, ctxkey.ClientRequestID, "client-system-custom")
	ctx = context.WithValue(ctx, ctxkey.RequestID, "request-system-custom")
	return ctx, resolution
}

func requireSystemCustomWorkerContext(t *testing.T, ctx context.Context, want service.SystemCustomGroupResolution) {
	t.Helper()

	got, ok := service.SystemCustomGroupResolutionFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, want, got)
	publicModel, ok := service.RequestedPublicModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, want.PublicModel, publicModel)
	upstreamModel, ok := service.ResolvedUpstreamModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, want.SourceModel, upstreamModel)
	require.Equal(t, "client-system-custom", ctx.Value(ctxkey.ClientRequestID))
	require.Equal(t, "request-system-custom", ctx.Value(ctxkey.RequestID))
}

func TestUsageRecordContextCopiesCompleteSystemCustomResolution(t *testing.T) {
	parent, resolution := systemCustomWorkerParentContext()
	workerCtx := usageRecordContext(parent, context.Background())

	requireSystemCustomWorkerContext(t, workerCtx, resolution)
}

func TestUsageRecordContextOrdinaryParentDoesNotCreateSystemCustomResolution(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "ordinary-client")
	workerCtx := usageRecordContext(parent, context.Background())

	_, ok := service.SystemCustomGroupResolutionFromContext(workerCtx)
	require.False(t, ok)
	_, publicOK := service.RequestedPublicModelFromContext(workerCtx)
	require.False(t, publicOK)
	_, upstreamOK := service.ResolvedUpstreamModelFromContext(workerCtx)
	require.False(t, upstreamOK)
	require.Equal(t, "ordinary-client", workerCtx.Value(ctxkey.ClientRequestID))
}

func TestOpenAISubmitUsageRecordTaskCopiesSystemCustomResolutionThroughRealWorker(t *testing.T) {
	parent, resolution := systemCustomWorkerParentContext()
	pool := newUsageRecordTestPool(t)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	workerCtxCh := make(chan context.Context, 1)

	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		workerCtxCh <- ctx
	})

	select {
	case workerCtx := <-workerCtxCh:
		requireSystemCustomWorkerContext(t, workerCtx, resolution)
	case <-time.After(time.Second):
		t.Fatal("usage record task not executed")
	}
}

type systemCustomHandlerUsageLogRepo struct {
	service.UsageLogRepository
	logs chan *service.UsageLog
}

func (r *systemCustomHandlerUsageLogRepo) CreateBestEffort(_ context.Context, log *service.UsageLog) error {
	clone := *log
	r.logs <- &clone
	return nil
}

func (r *systemCustomHandlerUsageLogRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	clone := *log
	r.logs <- &clone
	return true, nil
}

type systemCustomHandlerBillingRepo struct {
	service.UsageBillingRepository
	commands chan *service.UsageBillingCommand
}

func (r *systemCustomHandlerBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	clone := *cmd
	r.commands <- &clone
	return &service.UsageBillingApplyResult{Applied: false}, nil
}

func TestGatewaySubmitUsageRecordTask_SystemCustomWorkerBillsSubscriptionAndLogsBothGroups(t *testing.T) {
	parent, resolution := systemCustomWorkerParentContext()
	usageRepo := &systemCustomHandlerUsageLogRepo{logs: make(chan *service.UsageLog, 1)}
	billingRepo := &systemCustomHandlerBillingRepo{commands: make(chan *service.UsageBillingCommand, 1)}
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	gatewayService := service.NewGatewayService(
		nil, nil, usageRepo, billingRepo, nil, nil, nil, nil, cfg,
		nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, nil,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	pool := newUsageRecordTestPool(t)
	h := &GatewayHandler{gatewayService: gatewayService, usageRecordWorkerPool: pool}
	errCh := make(chan error, 1)
	user := &service.User{ID: 22}

	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		errCh <- h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
			Result: &service.ForwardResult{
				RequestID:     "handler-worker-system-custom",
				Model:         resolution.SourceModel,
				UpstreamModel: "claude-sonnet-4-20250514",
				Usage:         service.ClaudeUsage{InputTokens: 1000, OutputTokens: 100},
			},
			APIKey: &service.APIKey{
				ID:      11,
				UserID:  user.ID,
				User:    user,
				GroupID: &resolution.SourceGroupID,
				Group: &service.Group{
					ID: resolution.SourceGroupID, Platform: resolution.SourcePlatform, RateMultiplier: 1.25,
				},
			},
			User:    user,
			Account: &service.Account{ID: 33, Platform: resolution.SourcePlatform},
			Subscription: &service.UserSubscription{
				ID: 303, UserID: 22, GroupID: resolution.BillingGroupID,
				Group: &service.Group{ID: resolution.BillingGroupID, Platform: service.PlatformComposite, SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true},
			},
		})
	})

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("usage RecordUsage task not executed")
	}
	select {
	case cmd := <-billingRepo.commands:
		require.Greater(t, cmd.SubscriptionCost, 0.0)
		require.Zero(t, cmd.BalanceCost)
		require.NotNil(t, cmd.SubscriptionID)
		require.Equal(t, int64(303), *cmd.SubscriptionID)
	case <-time.After(time.Second):
		t.Fatal("usage billing command not recorded")
	}
	select {
	case log := <-usageRepo.logs:
		require.Equal(t, service.BillingTypeSubscription, log.BillingType)
		require.Equal(t, resolution.PublicModel, log.RequestedModel)
		require.Equal(t, resolution.SourceModel, log.Model)
		require.NotNil(t, log.UpstreamModel)
		require.Equal(t, "claude-sonnet-4-20250514", *log.UpstreamModel)
		require.NotNil(t, log.GroupID)
		require.Equal(t, resolution.BillingGroupID, *log.GroupID)
		require.NotNil(t, log.SourceGroupID)
		require.Equal(t, resolution.SourceGroupID, *log.SourceGroupID)
	case <-time.After(time.Second):
		t.Fatal("usage log not recorded")
	}
}

func TestSubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "client-request-123", gotClientRequestID)
	require.Equal(t, "request-456", gotRequestID)
}

func TestOpenAISubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "openai-request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &OpenAIGatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "openai-client-request-123", gotClientRequestID)
	require.Equal(t, "openai-request-456", gotRequestID)
}
