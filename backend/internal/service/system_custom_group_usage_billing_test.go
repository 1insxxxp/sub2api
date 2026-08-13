//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func systemCustomUsageContext(billingGroupID, sourceGroupID int64, publicModel, sourceModel, sourcePlatform string) context.Context {
	return WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: billingGroupID,
		SourceGroupID:  sourceGroupID,
		PublicModel:    publicModel,
		SourceModel:    sourceModel,
		SourcePlatform: sourcePlatform,
	})
}

func TestSystemCustomUsageFieldBillingModels_AreFieldAware(t *testing.T) {
	ctx := systemCustomUsageContext(101, 202, "public-alias", "source-model", PlatformOpenAI)

	require.Equal(t, "public-alias", billingModelFromOriginalUsageField(context.Background(), "public-alias"))
	require.Equal(t, "source-model", billingModelFromOriginalUsageField(ctx, "public-alias"))
	require.Equal(t, "source-model", billingModelFromOriginalUsageField(ctx, "real-channel-target"),
		"OriginalModel is display identity in every billing mode")
	require.Equal(t, "source-model", billingModelFromMappedUsageField(ctx, "public-alias"))
	require.Equal(t, "source-model", billingModelFromMappedUsageField(ctx, "PUBLIC-ALIAS"))
	require.Equal(t, "real-channel-target", billingModelFromMappedUsageField(ctx, "real-channel-target"))
	require.Equal(t, "public-alias", billingModelFromMappedUsageField(context.Background(), "public-alias"))
}

func TestOpenAISystemCustomRecordUsage_RejectsEveryBillingIdentityMismatch(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
		userID         = int64(22)
	)
	type mutateFunc func(*context.Context, **APIKey, **User, **UserSubscription)
	tests := []struct {
		name    string
		mutate  mutateFunc
		wantErr error
	}{
		{name: "valid control"},
		{name: "invalid resolution billing id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = context.WithValue(context.Background(), ctxkey.SystemCustomGroupResolution, SystemCustomGroupResolution{SourceGroupID: sourceGroupID, PublicModel: "monthly-gpt", SourceModel: "gpt-5.1", SourcePlatform: PlatformOpenAI})
		}},
		{name: "invalid resolution source id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = context.WithValue(context.Background(), ctxkey.SystemCustomGroupResolution, SystemCustomGroupResolution{BillingGroupID: billingGroupID, PublicModel: "monthly-gpt", SourceModel: "gpt-5.1", SourcePlatform: PlatformOpenAI})
		}},
		{name: "invalid resolution public model", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = context.WithValue(context.Background(), ctxkey.SystemCustomGroupResolution, SystemCustomGroupResolution{BillingGroupID: billingGroupID, SourceGroupID: sourceGroupID, SourceModel: "gpt-5.1", SourcePlatform: PlatformOpenAI})
		}},
		{name: "invalid resolution source model", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = context.WithValue(context.Background(), ctxkey.SystemCustomGroupResolution, SystemCustomGroupResolution{BillingGroupID: billingGroupID, SourceGroupID: sourceGroupID, PublicModel: "monthly-gpt", SourcePlatform: PlatformOpenAI})
		}},
		{name: "invalid resolution platform", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = context.WithValue(context.Background(), ctxkey.SystemCustomGroupResolution, SystemCustomGroupResolution{BillingGroupID: billingGroupID, SourceGroupID: sourceGroupID, PublicModel: "monthly-gpt", SourceModel: "gpt-5.1"})
		}},
		{name: "same billing and source id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(ctx *context.Context, _ **APIKey, _ **User, _ **UserSubscription) {
			*ctx = systemCustomUsageContext(billingGroupID, billingGroupID, "monthly-gpt", "gpt-5.1", PlatformOpenAI)
		}},
		{name: "missing key", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { *key = nil }},
		{name: "missing key source id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { (*key).GroupID = nil }},
		{name: "wrong key source id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) {
			(*key).GroupID = i64p(sourceGroupID + 1)
		}},
		{name: "missing key group", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { (*key).Group = nil }},
		{name: "wrong key group id", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) {
			(*key).Group.ID = sourceGroupID + 1
		}},
		{name: "wrong source platform", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) {
			(*key).Group.Platform = PlatformAnthropic
		}},
		{name: "composite source", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) {
			(*key).Group.Platform = PlatformComposite
		}},
		{name: "system container source", wantErr: ErrSystemCustomGroupSourceUnavailable, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) {
			(*key).Group.SystemCustomRoutingEnabled = true
		}},
		{name: "missing subscription", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) { *subscription = nil }},
		{name: "missing subscription id", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).ID = 0
		}},
		{name: "missing subscription group", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).Group = nil
		}},
		{name: "wrong subscription group id", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).GroupID = billingGroupID + 1
		}},
		{name: "wrong loaded subscription group id", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).Group.ID = billingGroupID + 1
		}},
		{name: "subscription group not subscription", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).Group.SubscriptionType = SubscriptionTypeStandard
		}},
		{name: "billing group not system custom container", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).Group.SystemCustomRoutingEnabled = false
		}},
		{name: "missing user", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, user **User, _ **UserSubscription) { *user = nil }},
		{name: "subscription user mismatch", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).UserID = userID + 1
		}},
		{name: "key user id mismatch", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { (*key).UserID = userID + 1 }},
		{name: "missing loaded key user", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { (*key).User = nil }},
		{name: "loaded key user mismatch", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, key **APIKey, _ **User, _ **UserSubscription) { (*key).User.ID = userID + 1 }},
		{name: "loaded subscription user mismatch", wantErr: ErrSubscriptionInvalid, mutate: func(_ *context.Context, _ **APIKey, _ **User, subscription **UserSubscription) {
			(*subscription).User = &User{ID: userID + 1}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "monthly-gpt", "gpt-5.1", " OPENAI ")
			user := &User{ID: userID}
			key := &APIKey{
				ID: 11, UserID: userID, User: &User{ID: userID}, GroupID: i64p(sourceGroupID),
				Group: &Group{ID: sourceGroupID, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 1},
			}
			subscription := &UserSubscription{
				ID: 303, UserID: userID, GroupID: billingGroupID,
				Group: &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true},
			}
			if tt.mutate != nil {
				tt.mutate(&ctx, &key, &user, &subscription)
			}

			var err error
			require.NotPanics(t, func() {
				err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
					Result:       &OpenAIForwardResult{RequestID: "identity-table-" + tt.name, Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100}},
					APIKey:       key,
					User:         user,
					Account:      &Account{ID: 33, Platform: PlatformOpenAI},
					Subscription: subscription,
				})
			})
			if tt.wantErr == nil {
				require.NoError(t, err)
				require.Equal(t, 1, billingRepo.calls)
				require.Equal(t, 1, usageRepo.calls)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			require.Zero(t, billingRepo.calls)
			require.Zero(t, usageRepo.calls)
			require.Zero(t, userRepo.deductCalls)
		})
	}
}

func TestDetachedBillingContextRetainsSystemCustomResolutionAndRequestIDs(t *testing.T) {
	resolution := SystemCustomGroupResolution{
		BillingGroupID: 101,
		SourceGroupID:  202,
		PublicModel:    "monthly-gpt",
		SourceModel:    "gpt-5.1",
		SourcePlatform: PlatformOpenAI,
	}
	parent, cancelParent := context.WithCancel(WithSystemCustomGroupResolution(context.Background(), resolution))
	parent = context.WithValue(parent, ctxkey.ClientRequestID, "detached-client")
	parent = context.WithValue(parent, ctxkey.RequestID, "detached-local")
	cancelParent()

	detached, cancelDetached := detachedBillingContext(parent)
	defer cancelDetached()
	require.NoError(t, detached.Err(), "context.WithoutCancel must detach cancellation")
	got, ok := SystemCustomGroupResolutionFromContext(detached)
	require.True(t, ok)
	require.Equal(t, resolution, got)
	require.Equal(t, "detached-client", detached.Value(ctxkey.ClientRequestID))
	require.Equal(t, "detached-local", detached.Value(ctxkey.RequestID))
	publicModel, ok := RequestedPublicModelFromContext(detached)
	require.True(t, ok)
	require.Equal(t, resolution.PublicModel, publicModel)
	upstreamModel, ok := ResolvedUpstreamModelFromContext(detached)
	require.True(t, ok)
	require.Equal(t, resolution.SourceModel, upstreamModel)
}

func TestGatewayServiceRecordUsage_SystemCustomRouteBillsSubscriptionAndPersistsBothGroups(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo)

	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1.75}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	subscription := &UserSubscription{ID: subscriptionID, UserID: 22, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)
	user := &User{ID: 22}

	err := svc.RecordUsage(ctx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:     "system-custom-anthropic",
			Model:         "claude-sonnet-4",
			UpstreamModel: "claude-sonnet-4-20250514",
			Usage:         ClaudeUsage{InputTokens: 1200, OutputTokens: 300},
			Duration:      time.Second,
		},
		APIKey:       &APIKey{ID: 11, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         user,
		Account:      &Account{ID: 33, Platform: PlatformAnthropic},
		Subscription: subscription,
	})

	require.NoError(t, err)
	require.NotNil(t, billingRepo.lastCmd)
	require.Greater(t, billingRepo.lastCmd.SubscriptionCost, 0.0)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.NotNil(t, billingRepo.lastCmd.SubscriptionID)
	require.Equal(t, subscriptionID, *billingRepo.lastCmd.SubscriptionID)
	require.Zero(t, userRepo.deductCalls, "system custom monthly access must never fall back to wallet billing")

	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, BillingTypeSubscription, usageRepo.lastLog.BillingType)
	require.Equal(t, "tavern-sonnet", usageRepo.lastLog.RequestedModel)
	require.Equal(t, "claude-sonnet-4", usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, "claude-sonnet-4-20250514", *usageRepo.lastLog.UpstreamModel)
	require.Equal(t, billingGroupID, *usageRepo.lastLog.GroupID)
	require.Equal(t, sourceGroupID, *usageRepo.lastLog.SourceGroupID)
	require.Equal(t, sourceGroup.RateMultiplier, usageRepo.lastLog.RateMultiplier)
}

type systemCustomAccumulatingBillingRepo struct {
	UsageBillingRepository
	commands []*UsageBillingCommand
	total    float64
}

func (r *systemCustomAccumulatingBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	copy := *cmd
	r.commands = append(r.commands, &copy)
	r.total += cmd.SubscriptionCost
	return &UsageBillingApplyResult{Applied: true}, nil
}

type systemCustomUsageLogCapture struct {
	UsageLogRepository
	logs []*UsageLog
}

func (r *systemCustomUsageLogCapture) Create(_ context.Context, log *UsageLog) (bool, error) {
	copy := *log
	r.logs = append(r.logs, &copy)
	return true, nil
}

func TestOpenAIRecordUsage_SystemCustomRoutesShareOneSubscriptionAcrossSources(t *testing.T) {
	const (
		billingGroupID = int64(101)
		subscriptionID = int64(303)
		userID         = int64(22)
		apiKeyID       = int64(11)
	)
	billingRepo := &systemCustomAccumulatingBillingRepo{}
	usageRepo := &systemCustomUsageLogCapture{}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	billingGroup := &Group{
		ID: billingGroupID, Platform: PlatformComposite,
		SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	subscription := &UserSubscription{ID: subscriptionID, UserID: userID, GroupID: billingGroupID, Group: billingGroup}
	user := &User{ID: userID}

	routes := []struct {
		requestID    string
		publicModel  string
		sourceModel  string
		upstream     string
		sourceGroup  int64
		multiplier   float64
		inputTokens  int
		outputTokens int
	}{
		{requestID: "shared-sub-source-a", publicModel: "tavern-fast", sourceModel: "gpt-5.4-mini", upstream: "gpt-5.4-mini-20260801", sourceGroup: 201, multiplier: 0.8, inputTokens: 1200, outputTokens: 200},
		{requestID: "shared-sub-source-b", publicModel: "tavern-pro", sourceModel: "gpt-5.4", upstream: "gpt-5.4-20260801", sourceGroup: 202, multiplier: 1.7, inputTokens: 800, outputTokens: 300},
	}
	var expectedTotal float64
	for _, route := range routes {
		expected, err := svc.billingService.CalculateCost(route.sourceModel, UsageTokens{InputTokens: route.inputTokens, OutputTokens: route.outputTokens}, route.multiplier)
		require.NoError(t, err)
		expectedTotal += expected.ActualCost

		ctx := systemCustomUsageContext(billingGroupID, route.sourceGroup, route.publicModel, route.sourceModel, PlatformOpenAI)
		err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID: route.requestID, Model: route.sourceModel, BillingModel: route.sourceModel,
				UpstreamModel: route.upstream,
				Usage:         OpenAIUsage{InputTokens: route.inputTokens, OutputTokens: route.outputTokens}, Duration: time.Second,
			},
			APIKey: &APIKey{
				ID: apiKeyID, UserID: userID, User: user, GroupID: i64p(route.sourceGroup),
				Group: &Group{ID: route.sourceGroup, Platform: PlatformOpenAI, RateMultiplier: route.multiplier},
			},
			User: user, Account: &Account{ID: route.sourceGroup + 1000, Platform: PlatformOpenAI}, Subscription: subscription,
			ChannelUsageFields: ChannelUsageFields{
				OriginalModel: route.publicModel, ChannelMappedModel: route.sourceModel,
				BillingModelSource: BillingModelSourceRequested,
				ModelMappingChain:  route.publicModel + "→" + route.sourceModel + "→" + route.upstream,
			},
		})
		require.NoError(t, err)
	}

	require.Len(t, billingRepo.commands, 2)
	require.Len(t, usageRepo.logs, 2)
	require.InDelta(t, expectedTotal, billingRepo.total, 1e-12)
	require.Zero(t, userRepo.deductCalls, "both source routes must stay on the shared subscription and never use wallet balance")
	for index, route := range routes {
		cmd := billingRepo.commands[index]
		require.NotNil(t, cmd.SubscriptionID)
		require.Equal(t, subscriptionID, *cmd.SubscriptionID)
		require.Zero(t, cmd.BalanceCost)

		log := usageRepo.logs[index]
		require.Equal(t, billingGroupID, requirePointerValue(t, log.GroupID))
		require.Equal(t, route.sourceGroup, requirePointerValue(t, log.SourceGroupID))
		require.Equal(t, route.publicModel, log.RequestedModel)
		require.Equal(t, route.sourceModel, log.Model)
		require.NotNil(t, log.UpstreamModel)
		require.Equal(t, route.upstream, *log.UpstreamModel)
		require.NotNil(t, log.ModelMappingChain)
		require.Equal(t, route.publicModel+"→"+route.sourceModel+"→"+route.upstream, *log.ModelMappingChain)
		require.InDelta(t, cmd.SubscriptionCost, log.ActualCost, 1e-12)
	}
}

func TestGatewayServiceRecordUsage_SystemCustomRequestedBillingUsesConcreteSourcePricing(t *testing.T) {
	const (
		billingGroupID = int64(111)
		sourceGroupID  = int64(212)
		subscriptionID = int64(313)
		publicModel    = "gpt-5.4-mini"
	)
	tests := []struct {
		name           string
		platform       string
		sourceModel    string
		upstreamModel  string
		billingSource  string
		mappedModel    string
		useLongContext bool
	}{
		{name: "anthropic_requested", platform: PlatformAnthropic, sourceModel: "claude-sonnet-4", upstreamModel: "claude-sonnet-4-20250514", billingSource: BillingModelSourceRequested, mappedModel: "claude-sonnet-4"},
		{name: "gemini_requested", platform: PlatformGemini, sourceModel: "gemini-2.5-pro", upstreamModel: "gemini-2.5-pro-preview", billingSource: BillingModelSourceRequested, mappedModel: "gemini-2.5-pro", useLongContext: true},
		{name: "anthropic_default_channel_mapped", platform: PlatformAnthropic, sourceModel: "claude-sonnet-4", upstreamModel: "claude-sonnet-4-20250514", billingSource: BillingModelSourceChannelMapped, mappedModel: publicModel},
		{name: "gemini_default_channel_mapped", platform: PlatformGemini, sourceModel: "gemini-2.5-pro", upstreamModel: "gemini-2.5-pro-preview", billingSource: BillingModelSourceChannelMapped, mappedModel: publicModel, useLongContext: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{})
			svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, sourceGroupID, tt.sourceModel)

			user := &User{ID: 32}
			sourceGroup := &Group{ID: sourceGroupID, Platform: tt.platform, RateMultiplier: 1.75}
			billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
			subscription := &UserSubscription{ID: subscriptionID, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup}
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, tt.sourceModel, tt.platform)
			result := &ForwardResult{
				RequestID:     "system-custom-requested-" + tt.name,
				Model:         tt.sourceModel,
				UpstreamModel: tt.upstreamModel,
				Usage:         ClaudeUsage{InputTokens: 1200, OutputTokens: 300},
				Duration:      time.Second,
			}
			fields := ChannelUsageFields{
				ChannelID:          77,
				OriginalModel:      publicModel,
				ChannelMappedModel: tt.mappedModel,
				BillingModelSource: tt.billingSource,
				ModelMappingChain:  publicModel + "→" + tt.sourceModel + "→" + tt.upstreamModel,
			}
			key := &APIKey{ID: 21, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup}
			account := &Account{ID: 43, Platform: tt.platform}

			var err error
			if tt.useLongContext {
				err = svc.RecordUsageWithLongContext(ctx, &RecordUsageLongContextInput{
					Result: result, APIKey: key, User: user, Account: account, Subscription: subscription,
					LongContextThreshold: 200_000, LongContextMultiplier: 2, ChannelUsageFields: fields,
				})
			} else {
				err = svc.RecordUsage(ctx, &RecordUsageInput{
					Result: result, APIKey: key, User: user, Account: account, Subscription: subscription,
					ChannelUsageFields: fields,
				})
			}
			require.NoError(t, err)

			// newOpenAITokenImageChannelPricingResolverForTest configures the source
			// channel at $3/M input and $15/M output. The cloned source group's
			// multiplier must still apply to the monthly subscription cost.
			wantSourceCost := (1200*3e-6 + 300*15e-6) * sourceGroup.RateMultiplier
			publicCost, calcErr := svc.billingService.CalculateCost(publicModel, UsageTokens{InputTokens: 1200, OutputTokens: 300}, sourceGroup.RateMultiplier)
			require.NoError(t, calcErr)
			require.NotEqual(t, wantSourceCost, publicCost.ActualCost, "fixture must distinguish public alias pricing from source channel pricing")

			require.NotNil(t, billingRepo.lastCmd)
			require.InDelta(t, wantSourceCost, billingRepo.lastCmd.SubscriptionCost, 1e-12)
			require.Zero(t, billingRepo.lastCmd.BalanceCost)
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, wantSourceCost, usageRepo.lastLog.ActualCost, 1e-12)
			require.NotEqual(t, publicCost.ActualCost, usageRepo.lastLog.ActualCost)
			require.Equal(t, publicModel, usageRepo.lastLog.RequestedModel)
			require.Equal(t, tt.sourceModel, usageRepo.lastLog.Model)
			require.NotNil(t, usageRepo.lastLog.UpstreamModel)
			require.Equal(t, tt.upstreamModel, *usageRepo.lastLog.UpstreamModel)
			require.NotNil(t, usageRepo.lastLog.ModelMappingChain)
			require.Equal(t, fields.ModelMappingChain, *usageRepo.lastLog.ModelMappingChain)
		})
	}
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomRouteBillsSameSubscriptionFromAnotherSource(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)

	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 0.8}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	subscription := &UserSubscription{ID: subscriptionID, UserID: 22, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-gpt", "gpt-5.1", PlatformOpenAI)
	user := &User{ID: 22}

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "system-custom-openai",
			Model:         "gpt-5.1",
			UpstreamModel: "gpt-5.1-2026-01-01",
			Usage:         OpenAIUsage{InputTokens: 900, OutputTokens: 100},
			Duration:      time.Second,
		},
		APIKey:       &APIKey{ID: 12, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         user,
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: subscription,
	})

	require.NoError(t, err)
	require.NotNil(t, billingRepo.lastCmd)
	require.Greater(t, billingRepo.lastCmd.SubscriptionCost, 0.0)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.NotNil(t, billingRepo.lastCmd.SubscriptionID)
	require.Equal(t, subscriptionID, *billingRepo.lastCmd.SubscriptionID,
		"different source groups must accumulate into the same monthly subscription")
	require.Zero(t, userRepo.deductCalls)

	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, BillingTypeSubscription, usageRepo.lastLog.BillingType)
	require.Equal(t, "tavern-gpt", usageRepo.lastLog.RequestedModel)
	require.Equal(t, "gpt-5.1", usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, "gpt-5.1-2026-01-01", *usageRepo.lastLog.UpstreamModel)
	require.Equal(t, billingGroupID, *usageRepo.lastLog.GroupID)
	require.Equal(t, sourceGroupID, *usageRepo.lastLog.SourceGroupID)
	require.Equal(t, sourceGroup.RateMultiplier, usageRepo.lastLog.RateMultiplier)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomRequestedBillingUsesConcreteSourcePricing(t *testing.T) {
	const (
		billingGroupID = int64(121)
		sourceGroupID  = int64(224)
		subscriptionID = int64(323)
		publicModel    = "gpt-5.4-mini"
		sourceModel    = "gpt-5.1"
		upstreamModel  = "gpt-5.1-2026-01-01"
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, sourceGroupID, sourceModel)

	user := &User{ID: 42}
	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1.6}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	subscription := &UserSubscription{ID: subscriptionID, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)
	fields := ChannelUsageFields{
		ChannelID:          88,
		OriginalModel:      publicModel,
		ChannelMappedModel: sourceModel,
		BillingModelSource: BillingModelSourceRequested,
		ModelMappingChain:  publicModel + "→" + sourceModel + "→" + upstreamModel,
	}

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "system-custom-openai-requested",
			Model:         sourceModel,
			BillingModel:  sourceModel,
			UpstreamModel: upstreamModel,
			Usage:         OpenAIUsage{InputTokens: 1200, OutputTokens: 300},
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 22, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:    user,
		Account: &Account{ID: 54, Platform: PlatformOpenAI}, Subscription: subscription,
		ChannelUsageFields: fields,
	})
	require.NoError(t, err)

	wantSourceCost := (1200*3e-6 + 300*15e-6) * sourceGroup.RateMultiplier
	publicCost, calcErr := svc.billingService.CalculateCost(publicModel, UsageTokens{InputTokens: 1200, OutputTokens: 300}, sourceGroup.RateMultiplier)
	require.NoError(t, calcErr)
	require.NotEqual(t, wantSourceCost, publicCost.ActualCost, "fixture must distinguish public alias pricing from source channel pricing")

	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, wantSourceCost, billingRepo.lastCmd.SubscriptionCost, 1e-12)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, wantSourceCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.NotEqual(t, publicCost.ActualCost, usageRepo.lastLog.ActualCost)
	require.Equal(t, publicModel, usageRepo.lastLog.RequestedModel)
	require.Equal(t, sourceModel, usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, upstreamModel, *usageRepo.lastLog.UpstreamModel)
	require.NotNil(t, usageRepo.lastLog.ModelMappingChain)
	require.Equal(t, fields.ModelMappingChain, *usageRepo.lastLog.ModelMappingChain)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomDefaultChannelMappedUsesConcreteSourcePricing(t *testing.T) {
	const (
		billingGroupID = int64(131)
		sourceGroupID  = int64(234)
		subscriptionID = int64(333)
		publicModel    = "gpt-5.4-mini"
		sourceModel    = "gpt-5.1"
		upstreamModel  = "gpt-5.1-2026-01-01"
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, sourceGroupID, sourceModel)

	user := &User{ID: 52}
	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1.6}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	subscription := &UserSubscription{ID: subscriptionID, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)
	fields := ChannelUsageFields{
		ChannelID:          89,
		OriginalModel:      publicModel,
		ChannelMappedModel: publicModel,
		BillingModelSource: BillingModelSourceChannelMapped,
		ModelMappingChain:  publicModel + "→" + sourceModel + "→" + upstreamModel,
	}

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "system-custom-openai-default-channel",
			Model:         sourceModel,
			BillingModel:  sourceModel,
			UpstreamModel: upstreamModel,
			Usage:         OpenAIUsage{InputTokens: 1200, OutputTokens: 300},
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 23, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:    user,
		Account: &Account{ID: 55, Platform: PlatformOpenAI}, Subscription: subscription,
		ChannelUsageFields: fields,
	})
	require.NoError(t, err)

	wantSourceCost := (1200*3e-6 + 300*15e-6) * sourceGroup.RateMultiplier
	publicCost, calcErr := svc.billingService.CalculateCost(publicModel, UsageTokens{InputTokens: 1200, OutputTokens: 300}, sourceGroup.RateMultiplier)
	require.NoError(t, calcErr)
	require.NotEqual(t, wantSourceCost, publicCost.ActualCost)
	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, wantSourceCost, billingRepo.lastCmd.SubscriptionCost, 1e-12)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, wantSourceCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.Equal(t, publicModel, usageRepo.lastLog.RequestedModel)
	require.Equal(t, sourceModel, usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, upstreamModel, *usageRepo.lastLog.UpstreamModel)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomDefaultChannelMappedNeverFallsBackToPricedPublicAlias(t *testing.T) {
	const (
		billingGroupID = int64(141)
		sourceGroupID  = int64(244)
		publicModel    = "gpt-5.4-mini"
		sourceModel    = "unpriced-system-custom-source"
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	user := &User{ID: 62}
	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1.2}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "system-custom-openai-unpriced-source", Model: sourceModel,
			BillingModel: sourceModel, UpstreamModel: sourceModel,
			Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 300}, Duration: time.Second,
		},
		APIKey:       &APIKey{ID: 24, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         user,
		Account:      &Account{ID: 56, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 343, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID: 90, OriginalModel: "GPT-5.4-MINI", ChannelMappedModel: "GPT-5.4-MINI",
			BillingModelSource: BillingModelSourceChannelMapped, ModelMappingChain: publicModel + "→" + sourceModel,
		},
	})

	require.Error(t, err)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, usageRepo.calls)
	require.Zero(t, userRepo.deductCalls)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomDisplayAliasNeverFallbacksInResponseOrUpstreamMode(t *testing.T) {
	const (
		billingGroupID = int64(151)
		sourceGroupID  = int64(254)
		publicModel    = "gpt-5.4-mini"
		sourceModel    = "unpriced-system-display-source"
	)
	for _, billingSource := range []string{BillingModelSourceResponse, BillingModelSourceUpstream} {
		t.Run(billingSource, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			user := &User{ID: 72}
			sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1.2}
			billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)

			err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{
					RequestID: "system-custom-display-alias-" + billingSource, Model: sourceModel,
					BillingModel: sourceModel, UpstreamModel: sourceModel,
					UpstreamResponseModel: "unpriced-upstream-response", Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 300}, Duration: time.Second,
				},
				APIKey:       &APIKey{ID: 25, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
				User:         user,
				Account:      &Account{ID: 57, Platform: PlatformOpenAI},
				Subscription: &UserSubscription{ID: 353, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup},
				ChannelUsageFields: ChannelUsageFields{
					ChannelID: 91, OriginalModel: publicModel, ChannelMappedModel: publicModel,
					BillingModelSource: billingSource, ModelMappingChain: publicModel + "→" + sourceModel,
				},
			})

			require.Error(t, err)
			require.Zero(t, billingRepo.calls)
			require.Zero(t, usageRepo.calls)
			require.Zero(t, userRepo.deductCalls)
		})
	}
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomResponseModelAdoptsRealResponsePrice(t *testing.T) {
	const (
		billingGroupID = int64(161)
		sourceGroupID  = int64(264)
		publicModel    = "gpt-5.6-sol"
		sourceModel    = "gpt-5.4"
		responseModel  = "gpt-5.4-mini"
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	user := &User{ID: 82}
	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)
	tokens := UsageTokens{InputTokens: 1200, OutputTokens: 300}
	wantResponseCost, err := svc.billingService.CalculateCost(responseModel, tokens, sourceGroup.RateMultiplier)
	require.NoError(t, err)
	sourceCost, err := svc.billingService.CalculateCost(sourceModel, tokens, sourceGroup.RateMultiplier)
	require.NoError(t, err)
	require.Less(t, wantResponseCost.ActualCost, sourceCost.ActualCost)

	err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "system-custom-real-response", Model: sourceModel, BillingModel: sourceModel,
			UpstreamModel: sourceModel, UpstreamResponseModel: responseModel,
			Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 300}, Duration: time.Second,
		},
		APIKey:       &APIKey{ID: 26, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         user,
		Account:      &Account{ID: 58, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 363, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID: 92, OriginalModel: publicModel, ChannelMappedModel: publicModel,
			BillingModelSource: BillingModelSourceResponse, ModelMappingChain: publicModel + "→" + sourceModel,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, wantResponseCost.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, wantResponseCost.ActualCost, billingRepo.lastCmd.SubscriptionCost, 1e-12)
	require.NotNil(t, usageRepo.lastLog.UpstreamResponseModel)
	require.Equal(t, responseModel, *usageRepo.lastLog.UpstreamResponseModel)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomKeepsActualAndDistinctMappedCandidates(t *testing.T) {
	const (
		billingGroupID = int64(171)
		sourceGroupID  = int64(274)
		publicModel    = "gpt-5.6-sol"
		sourceModel    = "unpriced-system-actual-source"
		pricedActual   = "gpt-5.4-mini"
	)
	tests := []struct {
		name          string
		resultModel   string
		upstreamModel string
		mappedModel   string
	}{
		{name: "actual result and upstream", resultModel: pricedActual, upstreamModel: pricedActual, mappedModel: publicModel},
		{name: "distinct real channel mapping", resultModel: sourceModel, upstreamModel: sourceModel, mappedModel: pricedActual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			user := &User{ID: 92}
			sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1.1}
			billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, publicModel, sourceModel, PlatformOpenAI)
			wantCost, err := svc.billingService.CalculateCost(pricedActual, UsageTokens{InputTokens: 1200, OutputTokens: 300}, sourceGroup.RateMultiplier)
			require.NoError(t, err)

			err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{
					RequestID: "system-custom-actual-" + tt.name, Model: tt.resultModel,
					BillingModel: tt.resultModel, UpstreamModel: tt.upstreamModel,
					Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 300}, Duration: time.Second,
				},
				APIKey:       &APIKey{ID: 27, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: sourceGroup},
				User:         user,
				Account:      &Account{ID: 59, Platform: PlatformOpenAI},
				Subscription: &UserSubscription{ID: 373, UserID: user.ID, GroupID: billingGroupID, Group: billingGroup},
				ChannelUsageFields: ChannelUsageFields{
					ChannelID: 93, OriginalModel: publicModel, ChannelMappedModel: tt.mappedModel,
					BillingModelSource: BillingModelSourceUpstream, ModelMappingChain: publicModel + "→" + tt.mappedModel,
				},
			})
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, wantCost.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
			require.NotNil(t, billingRepo.lastCmd)
			require.InDelta(t, wantCost.ActualCost, billingRepo.lastCmd.SubscriptionCost, 1e-12)
		})
	}
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomMissingPricingFailsClosed(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-unpriced", "unpriced-system-custom-model", PlatformOpenAI)
	user := &User{ID: 22}

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "system-custom-unpriced",
			Model:     "unpriced-system-custom-model",
			Usage:     OpenAIUsage{InputTokens: 900, OutputTokens: 100},
		},
		APIKey:       &APIKey{ID: 12, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1}},
		User:         user,
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 303, UserID: user.ID, GroupID: billingGroupID, Group: &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}},
	})

	require.Error(t, err)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, usageRepo.calls)
	require.Zero(t, userRepo.deductCalls)
}

func TestSystemCustomRecordUsage_FailsClosedOnMissingOrMismatchedSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
	)
	tests := []struct {
		name         string
		subscription *UserSubscription
	}{
		{name: "missing subscription"},
		{name: "wrong subscription group id", subscription: &UserSubscription{ID: 1, UserID: 22, GroupID: billingGroupID + 1, Group: &Group{ID: billingGroupID, SubscriptionType: SubscriptionTypeSubscription}}},
		{name: "wrong loaded subscription group", subscription: &UserSubscription{ID: 1, UserID: 22, GroupID: billingGroupID, Group: &Group{ID: billingGroupID + 1, SubscriptionType: SubscriptionTypeSubscription}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{})
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)
			user := &User{ID: 22}

			err := svc.RecordUsage(ctx, &RecordUsageInput{
				Result:       &ForwardResult{RequestID: "system-custom-mismatch", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100}},
				APIKey:       &APIKey{ID: 11, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1}},
				User:         user,
				Account:      &Account{ID: 33, Platform: PlatformAnthropic},
				Subscription: tt.subscription,
			})

			require.ErrorIs(t, err, ErrSubscriptionInvalid)
			require.Zero(t, billingRepo.calls)
			require.Zero(t, userRepo.deductCalls)
			require.Zero(t, usageRepo.calls)
		})
	}
}

func TestOpenAISystemCustomRecordUsage_FailsClosedOnMismatchedSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-gpt", "gpt-5.1", PlatformOpenAI)
	user := &User{ID: 22}

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:       &OpenAIForwardResult{RequestID: "system-custom-openai-mismatch", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100}},
		APIKey:       &APIKey{ID: 12, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1}},
		User:         user,
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 303, UserID: user.ID, GroupID: billingGroupID + 1, Group: &Group{ID: billingGroupID, SubscriptionType: SubscriptionTypeSubscription}},
	})

	require.ErrorIs(t, err, ErrSubscriptionInvalid)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, userRepo.deductCalls)
	require.Zero(t, usageRepo.calls)
}

func TestGatewayServiceRecordUsage_SystemCustomLegacyFallbackStillBillsSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)
	user := &User{ID: 22}

	err := svc.RecordUsage(ctx, &RecordUsageInput{
		Result:       &ForwardResult{RequestID: "system-custom-legacy", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000}},
		APIKey:       &APIKey{ID: 11, UserID: user.ID, User: user, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1}},
		User:         user,
		Account:      &Account{ID: 33, Platform: PlatformAnthropic},
		Subscription: &UserSubscription{ID: subscriptionID, UserID: user.ID, GroupID: billingGroupID, Group: &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true}},
	})

	require.NoError(t, err)
	require.Equal(t, 1, subRepo.incrementCalls)
	require.Zero(t, userRepo.deductCalls)
}

type subscriptionCacheGroupCapture struct {
	billingCacheWorkerStub
	lastGroupID atomic.Int64
}

func (b *subscriptionCacheGroupCapture) UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error {
	b.lastGroupID.Store(groupID)
	return b.billingCacheWorkerStub.UpdateSubscriptionUsage(ctx, userID, groupID, cost)
}

func TestFinalizePostUsageBilling_SystemCustomUsesAuthoritativeBillingGroupCacheKey(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
	)
	cache := &subscriptionCacheGroupCapture{}
	cacheService := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(cacheService.Stop)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)

	finalizePostUsageBilling(ctx, &postUsageBillingParams{
		Cost:               &CostBreakdown{TotalCost: 1, ActualCost: 1},
		User:               &User{ID: 22},
		APIKey:             &APIKey{ID: 11, GroupID: i64p(sourceGroupID)},
		Account:            &Account{ID: 33},
		Subscription:       &UserSubscription{ID: 303, GroupID: billingGroupID},
		IsSubscriptionBill: true,
	}, &billingDeps{
		billingCacheService: cacheService,
		deferredService:     &DeferredService{},
	}, &UsageBillingApplyResult{Applied: true})

	require.Eventually(t, func() bool {
		return cache.lastGroupID.Load() != 0
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, billingGroupID, cache.lastGroupID.Load())
}
