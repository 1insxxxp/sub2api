package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type UsageBillingStatus string

const (
	UsageBillingStatusSettled         UsageBillingStatus = "settled"
	UsageBillingStatusPendingReview   UsageBillingStatus = "pending_review"
	UsagePendingReasonUpstreamMissing                    = "upstream_usage_missing"
)

func (s UsageBillingStatus) Normalize() UsageBillingStatus {
	if s == UsageBillingStatusPendingReview {
		return s
	}
	return UsageBillingStatusSettled
}

type pendingUsageWriter interface {
	UpsertPendingUsage(context.Context, *UsageLog) error
}

// RecordPendingUsage stores a request-history snapshot without entering any
// billing, quota, outbox, or deduplication path. Its identity is exactly the
// normal settlement identity, so a later measured settlement supersedes it.
func (s *GatewayService) RecordPendingUsage(ctx context.Context, input *RecordUsageInput) error {
	if s == nil || input == nil || input.Result == nil || input.User == nil || input.APIKey == nil || input.Account == nil {
		return errors.New("pending usage requires request and authenticated identity")
	}
	if input.User.ID <= 0 || input.APIKey.ID <= 0 || input.Account.ID <= 0 || input.APIKey.UserID != input.User.ID {
		return errors.New("pending usage identity mismatch")
	}
	writer, ok := s.usageLogRepo.(pendingUsageWriter)
	if !ok {
		return errors.New("pending usage persistence unavailable")
	}
	result := input.Result
	if strings.TrimSpace(result.Model) == "" {
		return errors.New("pending usage model missing")
	}
	isSubscription, custom, err := resolveRecordUsageBillingIdentity(ctx, input.APIKey, input.User, input.Subscription)
	if err != nil {
		return err
	}
	durationMs := max(0, int(result.Duration.Milliseconds()))
	requestedModel := result.Model
	if strings.TrimSpace(input.OriginalModel) != "" {
		requestedModel = input.OriginalModel
	}
	requestedModel = requestedModelForUsage(ctx, requestedModel)
	log := &UsageLog{
		UserID: input.User.ID, APIKeyID: input.APIKey.ID, AccountID: input.Account.ID,
		RequestID: resolveUsageBillingRequestID(ctx, result.RequestID),
		Model:     result.Model, RequestedModel: requestedModel,
		UpstreamModel: optionalTrimmedStringPtr(result.UpstreamModel),
		GroupID:       input.APIKey.GroupID, CustomGroupID: input.APIKey.CustomGroupID,
		SubscriptionID: optionalSubscriptionID(input.Subscription),
		BillingStatus:  UsageBillingStatusPendingReview, BillingPendingReason: UsagePendingReasonUpstreamMissing,
		InputTokensSource: UsageTokenSourceUnreported, OutputTokensSource: UsageTokenSourceUnreported,
		Stream: result.Stream, RequestType: RequestTypeFromLegacy(result.Stream, false),
		DurationMs: &durationMs, FirstTokenMs: result.FirstTokenMs,
		InboundEndpoint:   optionalTrimmedStringPtr(input.InboundEndpoint),
		UpstreamEndpoint:  optionalTrimmedStringPtr(input.UpstreamEndpoint),
		UpstreamRequestID: usageUpstreamRequestIDForResult(input.Account, result),
		UserAgent:         optionalTrimmedStringPtr(input.UserAgent), IPAddress: optionalTrimmedStringPtr(input.IPAddress),
		SessionID: optionalTrimmedStringPtr(input.SessionID),
		ChannelID: optionalInt64Ptr(input.ChannelID), ModelMappingChain: optionalTrimmedStringPtr(input.ModelMappingChain),
		ServiceTier: result.ServiceTier, ReasoningEffort: result.ReasoningEffort,
		RequestedReasoningEffort: coalesceRequestedReasoningEffort(result.RequestedReasoningEffort, result.ReasoningEffort),
		CreatedAt:                time.Now(),
	}
	if !input.PricingAt.IsZero() {
		log.CreatedAt = input.PricingAt
	}
	if isSubscription {
		log.BillingType = BillingTypeSubscription
	}
	if custom != nil {
		log.GroupID = optionalInt64Ptr(custom.BillingGroupID)
		log.SourceGroupID = optionalInt64Ptr(custom.SourceGroupID)
	}
	if result.InputTokensSource == UsageTokenSourceUpstream {
		log.InputTokensSource = UsageTokenSourceUpstream
		log.InputTokens = max(0, result.Usage.InputTokens)
		log.CacheReadTokens = max(0, result.Usage.CacheReadInputTokens)
		log.CacheCreationTokens = max(0, result.Usage.CacheCreationInputTokens)
		log.CacheCreation5mTokens = max(0, result.Usage.CacheCreation5mTokens)
		log.CacheCreation1hTokens = max(0, result.Usage.CacheCreation1hTokens)
	}
	if result.OutputTokensSource == UsageTokenSourceUpstream {
		log.OutputTokensSource = UsageTokenSourceUpstream
		log.OutputTokens = max(0, result.Usage.OutputTokens)
		log.ImageOutputTokens = max(0, result.Usage.ImageOutputTokens)
	}
	persistCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	if err := writer.UpsertPendingUsage(persistCtx, log); err != nil {
		return fmt.Errorf("persist pending usage: %w", err)
	}
	return nil
}
