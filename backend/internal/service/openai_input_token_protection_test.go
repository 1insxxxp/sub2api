package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIInputProtectionSettlement(t *testing.T) {
	for _, tc := range []struct {
		name         string
		continuation bool
		platform     string
	}{
		{"openai", false, PlatformOpenAI}, {"grok", false, PlatformGrok}, {"continuation", true, PlatformOpenAI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			ledger := &openAIRecordUsageBillingRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			model := "gpt-5.1"
			if tc.platform == PlatformGrok {
				model = "grok-4"
			}
			raw := OpenAIUsage{InputTokens: 200, CacheReadInputTokens: 50, CacheCreationInputTokens: 10, OutputTokens: 8}
			result := &OpenAIForwardResult{RequestID: "one", Model: model, Usage: raw}
			if tc.continuation {
				var err error
				result, err = AggregateOpenAIContinuationResults([]*OpenAIForwardResult{
					{RequestID: "one", Model: model, Usage: OpenAIUsage{InputTokens: 140, OutputTokens: 5}},
					{RequestID: "two", Model: model, Usage: OpenAIUsage{InputTokens: 60, CacheReadInputTokens: 50, CacheCreationInputTokens: 10, OutputTokens: 3}},
				})
				require.NoError(t, err)
			}
			result.InputTokenProtection = &InputTokenProtection{Limit: 60}
			group := &Group{ID: 4, RateMultiplier: 1, Platform: tc.platform}
			input := &OpenAIRecordUsageInput{Result: result, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 1, GroupID: &group.ID, Group: group, Quota: 100},
				Account: &Account{ID: 3, Platform: tc.platform, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}}
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			log := repo.lastLog
			require.NotNil(t, log)
			require.Equal(t, 10, log.InputTokens)
			require.Equal(t, 50, log.CacheReadTokens)
			require.Zero(t, log.CacheCreationTokens)
			require.Equal(t, 8, log.OutputTokens)
			require.Equal(t, raw, result.Usage)
			want := expectedOpenAICost(t, svc, model, OpenAIUsage{InputTokens: 60, CacheReadInputTokens: 50, OutputTokens: 8}, 1)
			provider := expectedOpenAICost(t, svc, model, raw, 1)
			require.InDelta(t, want.ActualCost, ledger.lastCmd.BalanceCost, 1e-8)
			require.InDelta(t, want.ActualCost, ledger.lastCmd.APIKeyQuotaCost, 1e-8)
			require.InDelta(t, provider.TotalCost, ledger.lastCmd.AccountQuotaCost, 1e-8)
			require.InDelta(t, provider.TotalCost, *log.AccountStatsCost, 1e-8)
			require.Equal(t, 200, *log.Outcome.UpstreamInputTokens)
			if tc.continuation {
				require.Equal(t, 140, log.Outcome.ContinuationUsage[0].InputTokens)
				require.Equal(t, 10, log.Outcome.ContinuationUsage[0].CustomerInputTokens)
				require.Zero(t, log.Outcome.ContinuationUsage[1].CustomerInputTokens)
			}
		})
	}
}

func TestOpenAIInputProtectionPendingAndPerRequest(t *testing.T) {
	for _, perRequest := range []bool{false, true} {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		pending := &pendingUsageWriterStub{}
		ledger := &openAIRecordUsageBillingRepoStub{}
		svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		group := &Group{ID: 4, RateMultiplier: 1}
		model := "gpt-5.1"
		if perRequest {
			svc.resolver = newOpenAITextChannelPricingResolverForTest(t, group.ID, model, 0.25)
		} else {
			svc.usageLogRepo = pending
		}
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{RequestID: "pending", Model: model, InputTokenProtection: &InputTokenProtection{Pending: true}, Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 3}, InputTokensSource: UsageTokenSourceUpstream},
			APIKey: &APIKey{ID: 2, UserID: 1, GroupID: &group.ID, Group: group}, User: &User{ID: 1}, Account: &Account{ID: 3},
		})
		require.NoError(t, err)
		if perRequest {
			require.Equal(t, 1000, repo.lastLog.InputTokens)
			require.Equal(t, 0.25, ledger.lastCmd.BalanceCost)
		} else {
			require.Zero(t, ledger.calls)
			require.Equal(t, UsageBillingStatusPendingReview, pending.log.BillingStatus)
			require.Zero(t, pending.log.ActualCost)
			require.Zero(t, pending.log.InputTokens)
		}
	}
}

func TestOpenAIInputProtectionPassthroughUsesEachOriginalTurn(t *testing.T) {
	upstream := newStagedPassthroughConn()
	results := make(chan *OpenAIForwardResult, 2)
	hooks := &OpenAIWSIngressHooks{
		InputProtectionForTurn: func(_ int, original []byte, _ string) *InputTokenProtection {
			return EstimateCustomerInputTokenProtection("response.create", original)
		},
		AfterTurn: func(_ int, r *OpenAIForwardResult, _ error) {
			if r != nil {
				results <- r
			}
		},
	}
	server, _ := startPassthroughHookRecordingServer(t, context.Background(), newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount(), hooks)
	defer server.Close()
	frames := []string{`{"type":"response.create","model":"gpt-5.1","input":"short"}`, `{"type":"response.create","model":"gpt-5.1","input":"more distinct customer words for the next turn"}`}
	client := dialPassthroughLifecycleClientWithPayload(t, server, frames[0])
	defer client.CloseNow()
	for i, frame := range frames {
		if i > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err := client.Write(ctx, coderws.MessageText, []byte(frame))
			cancel()
			require.NoError(t, err)
		}
		requirePassthroughUpstreamWrite(t, upstream, time.Second)
		usageDetails := `"input_tokens_details":{"cached_tokens":1000,"cache_write_tokens":80,"cache_write_tokens_details":{"ttl_30m_tokens":30,"ttl_12h_tokens":50}}`
		cachedPath := "response.usage.input_tokens_details.cached_tokens"
		if i == 1 {
			usageDetails = `"cache_read_input_tokens":1000,"input_tokens_details":{"cache_write_tokens":80,"cache_write_tokens_details":{"ttl_30m_tokens":30,"ttl_12h_tokens":50}}`
			cachedPath = "response.usage.cache_read_input_tokens"
		}
		upstream.Send(fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_%d","usage":{"input_tokens":10000,%s,"output_tokens":7,"total_tokens":10007}}}`, i, usageDetails))
		event, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
		require.NoError(t, err)
		expected := EstimateCustomerInputTokenProtection("response.create", []byte(frame))
		require.False(t, expected.Pending)
		require.EqualValues(t, expected.Limit, gjson.GetBytes(event, "response.usage.input_tokens").Int())
		require.EqualValues(t, expected.Limit, gjson.GetBytes(event, cachedPath).Int())
		require.EqualValues(t, 7+expected.Limit, gjson.GetBytes(event, "response.usage.total_tokens").Int())
		select {
		case r := <-results:
			require.Equal(t, expected, r.InputTokenProtection)
			require.Equal(t, 10000, r.Usage.InputTokens)
			require.Equal(t, 1000, r.Usage.CacheReadInputTokens)
			require.Equal(t, 30, r.Usage.CacheCreation30mInputTokens)
			require.Equal(t, 50, r.Usage.CacheCreation12hInputTokens)
		case <-time.After(time.Second):
			t.Fatal("turn result missing")
		}
	}
}

func TestOpenAIInputProtectionMixedContinuationPreservesPerCall(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{
		{Model: "gpt-5.1", RequestID: "per-call", WebSearchCalls: 1, Usage: OpenAIUsage{InputTokens: 400}},
		{Model: "gpt-5.1", RequestID: "tokens", Usage: OpenAIUsage{InputTokens: 300, OutputTokens: 3}},
	})
	require.NoError(t, err)
	result.InputTokenProtection = &InputTokenProtection{Limit: 10}
	input := &OpenAIRecordUsageInput{Result: result, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 1}, Account: &Account{ID: 3}}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	log := repo.lastLog
	require.Equal(t, 410, log.InputTokens)
	require.Equal(t, 400, log.Outcome.ContinuationUsage[0].CustomerInputTokens)
	require.Equal(t, 10, log.Outcome.ContinuationUsage[1].CustomerInputTokens)
	want := expectedOpenAICost(t, svc, result.Model, OpenAIUsage{InputTokens: 10, OutputTokens: 3}, 1.1)
	require.InDelta(t, 0.01+want.TotalCost, log.TotalCost, 1e-10)
}

func TestOpenAIInputProtectionCacheDetailsCannotIncreaseGrossInput(t *testing.T) {
	for _, u := range []OpenAIUsage{
		{InputTokens: 3, CacheReadInputTokens: 30, CacheCreationInputTokens: 20},
		{InputTokens: 3, CacheCreationInputTokens: 20, CacheCreation30mInputTokens: 1, CacheCreation12hInputTokens: 19},
	} {
		result := &OpenAIForwardResult{Usage: u}
		customer := projectOpenAIInputForBilling(result, &InputTokenProtection{Limit: 100}, nil)
		require.Equal(t, u, result.Usage)
		require.Equal(t, 3, customer.Usage.InputTokens)
		require.Equal(t, 3, customer.Usage.CacheReadInputTokens+customer.Usage.CacheCreationInputTokens)
	}
}

func TestOpenAIInputProtectionSearchTariffs(t *testing.T) {
	const perThousand, perCall = 7.0, 0.13
	group := &Group{ID: 4, RateMultiplier: 1, SearchPricePer1k: f64p(perThousand), WebSearchPricePerCall: f64p(perCall)}
	for _, tc := range []struct {
		name                    string
		searchCount, alphaCalls int
		protection              *InputTokenProtection
		wantInput               int
	}{
		{"generation with tool calls", 3, 0, &InputTokenProtection{Limit: 8}, 8},
		{"standalone alpha search", 0, 1, &InputTokenProtection{Pending: true}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			ledger := &openAIRecordUsageBillingRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			result := &OpenAIForwardResult{Model: "grok-4", RequestID: "search", SearchCount: tc.searchCount, WebSearchCalls: tc.alphaCalls, InputTokenProtection: tc.protection}
			customer, provider := perCall, perCall
			if tc.searchCount > 0 {
				result.Usage = OpenAIUsage{InputTokens: 800, OutputTokens: 10}
				fee := float64(tc.searchCount) * perThousand / 1000
				customer = expectedOpenAICost(t, svc, result.Model, OpenAIUsage{InputTokens: tc.wantInput, OutputTokens: 10}, 1).TotalCost + fee
				provider = expectedOpenAICost(t, svc, result.Model, result.Usage, 1).TotalCost + fee
			}
			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result,
				APIKey: &APIKey{ID: 2, UserID: 1, Group: group, GroupID: &group.ID, Quota: 100}, User: &User{ID: 1},
				Account: &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}})
			require.NoError(t, err)
			require.Equal(t, tc.wantInput, repo.lastLog.InputTokens)
			require.Equal(t, result.Usage.OutputTokens, repo.lastLog.OutputTokens)
			require.InDelta(t, customer, repo.lastLog.TotalCost, 1e-10)
			require.InDelta(t, customer, ledger.lastCmd.BalanceCost, 1e-8)
			require.InDelta(t, customer, ledger.lastCmd.APIKeyQuotaCost, 1e-8)
			require.InDelta(t, provider, ledger.lastCmd.AccountQuotaCost, 1e-8)
		})
	}
}

func TestOpenAIInputProtectionComparesResponseTariffAtEachUsage(t *testing.T) {
	const a, b = "gpt-a", "gpt-b"
	group := &Group{ID: 4, Platform: PlatformOpenAI, RateMultiplier: 1, ModelPricing: []ChannelModelPricing{
		{Models: []string{a}, BillingMode: BillingModeToken, InputPrice: f64p(0.001), OutputPrice: f64p(0.01)},
		{Models: []string{b}, BillingMode: BillingModeToken, InputPrice: f64p(0.004), OutputPrice: f64p(0.001)},
	}}
	for _, tc := range []struct {
		name, baseline, response string
		providerOutput           int
		wantProvider             float64
	}{
		{"response becomes cheaper", a, b, 10, 1.1},
		{"baseline becomes cheaper", b, a, 10, 1.1},
		{"interrupted delivery changes provider tariff", a, b, 2000, 6.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			ledger := &openAIRecordUsageBillingRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.resolver = NewModelPricingResolver(nil, svc.billingService)
			delivered := 10
			result := &OpenAIForwardResult{RequestID: "crossing", Model: tc.baseline, UpstreamModel: tc.baseline, UpstreamResponseModel: tc.response,
				Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: tc.providerOutput}, InputTokenProtection: &InputTokenProtection{Limit: 2},
				ClientDisconnect: tc.providerOutput != delivered, DeliveredOutputTokens: &delivered}
			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result,
				User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 1, Group: group, GroupID: &group.ID, Quota: 100}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
				Account:            &Account{ID: 3, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Extra: map[string]any{"quota_limit": 100.0}},
				ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceResponse, OriginalModel: tc.baseline, ChannelMappedModel: tc.baseline}})
			require.NoError(t, err)
			require.Equal(t, 2, repo.lastLog.InputTokens)
			require.InDelta(t, 0.018, ledger.lastCmd.BalanceCost, 1e-8)
			require.InDelta(t, 0.018, ledger.lastCmd.APIKeyQuotaCost, 1e-8)
			require.InDelta(t, tc.wantProvider, ledger.lastCmd.AccountQuotaCost, 1e-8)
			require.InDelta(t, tc.wantProvider, *repo.lastLog.AccountStatsCost, 1e-8)
		})
	}
}

func TestOpenAIInputProtectionFreeFastRetainsVolumeDiscount(t *testing.T) {
	const model = "gpt-5.6-sol"
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	ledger := &openAIRecordUsageBillingRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	group := &Group{ID: 4, Platform: PlatformOpenAI, FreeOpenAIFast: true, LongContextPricingEnabled: true, RateMultiplier: 1}
	cache := newEmptyChannelCache()
	cache.channelByGroupID[group.ID] = &Channel{ID: group.ID, Status: StatusActive}
	cache.pricingByGroupModel[channelModelKey{groupID: group.ID, model: model}] = &ChannelModelPricing{
		BillingMode: BillingModeToken, InputPrice: f64p(0.1), FastMultiplier: f64p(3),
		Intervals: []PricingInterval{{MinTokens: 100, InputPrice: f64p(0.0001)}},
	}
	cache.loadedAt = time.Now()
	cs := &ChannelService{}
	cs.cache.Store(cache)
	svc.resolver = NewModelPricingResolver(cs, svc.billingService)
	priority := "priority"
	result := &OpenAIForwardResult{RequestID: "volume", Model: model, ServiceTier: &priority, Usage: OpenAIUsage{InputTokens: 2000}, InputTokenProtection: &InputTokenProtection{Limit: 10}}
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: &APIKey{ID: 2, UserID: 1, Group: group, GroupID: &group.ID, Quota: 100}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
		User: &User{ID: 1}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0, "openai_long_context_billing_enabled": true}}})
	require.NoError(t, err)
	require.Equal(t, 10, repo.lastLog.InputTokens)
	// The short-context quote would cost 1 at Standard; the original quote cost 0.2.
	require.InDelta(t, 0.2, ledger.lastCmd.BalanceCost, 1e-8)
	require.InDelta(t, 0.2, ledger.lastCmd.APIKeyQuotaCost, 1e-8)
	require.InDelta(t, 0.6, ledger.lastCmd.AccountQuotaCost, 1e-8)
}

func TestOpenAIInputProtectionPassthroughMissingAndZero(t *testing.T) {
	for _, tc := range []struct {
		name, usage string
		measured    bool
	}{
		{"output only", `"output_tokens":7`, false},
		{"invalid input", `"input_tokens":"0","output_tokens":7`, false},
		{"cache details without gross", `"cache_read_input_tokens":12,"output_tokens":7`, false},
		{"explicit zero", `"input_tokens":0,"output_tokens":7`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := newStagedPassthroughConn()
			results := make(chan *OpenAIForwardResult, 1)
			server, _ := startPassthroughHookRecordingServer(t, context.Background(), newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount(), &OpenAIWSIngressHooks{
				InputProtectionForTurn: func(int, []byte, string) *InputTokenProtection { return &InputTokenProtection{Limit: 2} },
				AfterTurn: func(_ int, r *OpenAIForwardResult, _ error) {
					if r != nil {
						results <- r
					}
				},
			})
			defer server.Close()
			client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-5.1","input":"hello"}`)
			defer client.CloseNow()
			requirePassthroughUpstreamWrite(t, upstream, time.Second)
			upstream.Send(`{"type":"response.completed","response":{"id":"resp_one","usage":{` + tc.usage + `}}}`)
			event, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
			require.NoError(t, err)
			require.EqualValues(t, 7, gjson.GetBytes(event, "response.usage.output_tokens").Int())
			var r *OpenAIForwardResult
			select {
			case r = <-results:
			case <-time.After(time.Second):
				t.Fatal("missing result")
			}
			require.NotNil(t, r.InputTokenProtection)
			require.Equal(t, 7, r.Usage.OutputTokens)
			require.Equal(t, UsageTokenSourceUpstream, r.OutputTokensSource)
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			ledger := &openAIRecordUsageBillingRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			pending := &pendingUsageWriterStub{}
			if !tc.measured {
				svc.usageLogRepo = pending
			}
			group := &Group{ID: 4, RateMultiplier: 1}
			require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: r, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, GroupID: &group.ID, Group: group}, Account: &Account{ID: 3, Platform: PlatformOpenAI},
			}))
			if tc.measured {
				require.Equal(t, UsageTokenSourceUpstream, r.InputTokensSource)
				require.Equal(t, 1, ledger.calls)
				require.Equal(t, 7, repo.lastLog.OutputTokens)
			} else {
				require.Equal(t, UsageTokenSourceUnreported, r.InputTokensSource)
				require.Zero(t, ledger.calls)
				require.Equal(t, UsageBillingStatusPendingReview, pending.log.BillingStatus)
				require.NotNil(t, pending.log.PendingEvidence)
				require.Nil(t, pending.log.PendingEvidence.InputTokens)
				require.Equal(t, 7, *pending.log.PendingEvidence.OutputTokens)
				require.Equal(t, int(gjson.Get("{"+tc.usage+"}", "cache_read_input_tokens").Int()), pending.log.PendingEvidence.CacheReadTokens)
			}
		})
	}
}

func TestOpenAIInputProtectionPassthroughAdoptedTariff(t *testing.T) {
	const fixed, metered = "gpt-fixed", "gpt-metered"
	for _, tc := range []struct {
		name, requested, reported string
		fixedPrice                float64
		wantInput                 int
		wantCost                  float64
	}{
		{"per-call adopts token", fixed, metered, 0.20, 2, 0.009},
		{"token adopts per-call", metered, fixed, 0.01, 50, 0.01},
		{"per-call stays per-call", fixed, metered, 0.01, 50, 0.01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			ledger := &openAIRecordUsageBillingRepoStub{}
			pricing := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			pricing.resolver = NewModelPricingResolver(nil, pricing.billingService)
			group := &Group{ID: 4, RateMultiplier: 1, Platform: PlatformOpenAI, ModelPricing: []ChannelModelPricing{
				{Models: []string{fixed}, BillingMode: BillingModePerRequest, PerRequestPrice: f64p(tc.fixedPrice)},
				{Models: []string{metered}, BillingMode: BillingModeToken, InputPrice: f64p(.001), OutputPrice: f64p(.001)},
			}}
			cache := newEmptyChannelCache()
			cache.loadedAt = time.Now()
			cache.channelByGroupID[group.ID] = &Channel{ID: 1, Status: StatusActive, BillingModelSource: BillingModelSourceResponse}
			pricing.channelService = &ChannelService{}
			pricing.channelService.cache.Store(cache)
			input := OpenAIRecordUsageInput{APIKey: &APIKey{ID: 2, GroupID: &group.ID, Group: group}, User: &User{ID: 1}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
				ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceResponse, OriginalModel: tc.requested, ChannelMappedModel: tc.requested}}
			upstream := newStagedPassthroughConn()
			results := make(chan *OpenAIForwardResult, 1)
			server, _ := startPassthroughHookRecordingServer(t, context.Background(), newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount(), &OpenAIWSIngressHooks{
				InputProtectionForTurn: func(int, []byte, string) *InputTokenProtection { return &InputTokenProtection{Limit: 2} },
				ProjectUsageForTurn: func(_ int, r OpenAIForwardResult) bool {
					quote := input
					quote.Result = &r
					return pricing.InputTokenProtectionBillsForResult(context.Background(), &quote)
				},
				AfterTurn: func(_ int, r *OpenAIForwardResult, _ error) {
					if r != nil {
						results <- r
					}
				},
			})
			defer server.Close()
			client := dialPassthroughLifecycleClientWithPayload(t, server, fmt.Sprintf(`{"type":"response.create","model":%q,"input":"hi"}`, tc.requested))
			defer client.CloseNow()
			requirePassthroughUpstreamWrite(t, upstream, time.Second)
			upstream.Send(`{"type":"response.in_progress","response":{"id":"resp_tariff","usage":{"input_tokens":50,"output_tokens":1}}}`)
			early, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
			require.NoError(t, err)
			require.EqualValues(t, 2, gjson.GetBytes(early, "response.usage.input_tokens").Int(), "provisional metered tariff")
			gross := ""
			if tc.wantInput == 2 {
				gross = `"input_tokens":50,`
			}
			upstream.Send(fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_tariff","model":%q,"usage":{%s"output_tokens":7,"total_tokens":57}}}`, tc.reported, gross))
			wire, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
			require.NoError(t, err)
			require.EqualValues(t, tc.wantInput, gjson.GetBytes(wire, "response.usage.input_tokens").Int())
			require.EqualValues(t, tc.wantInput+7, gjson.GetBytes(wire, "response.usage.total_tokens").Int())
			select {
			case r := <-results:
				require.Equal(t, &InputTokenProtection{Limit: 2}, r.InputTokenProtection)
				require.Equal(t, 50, r.Usage.InputTokens)
				input.Result = r
			case <-time.After(time.Second):
				t.Fatal("missing turn result")
			}
			require.NoError(t, pricing.RecordUsage(context.Background(), &input))
			require.Equal(t, tc.wantInput, repo.lastLog.InputTokens)
			require.InDelta(t, tc.wantCost, ledger.lastCmd.BalanceCost, 1e-9)
		})
	}
}

func TestOpenAIInputProtectionAcceptedChatCacheWithoutGross(t *testing.T) {
	round := &OpenAIChatUsageRound{UpstreamAccepted: true, CacheReadReported: true, CacheWriteReported: true, OutputReported: true, Usage: OpenAIUsage{CacheReadInputTokens: 10, CacheCreationInputTokens: 5, OutputTokens: 7}, InputFallback: &GeminiInputUsageFallback{Tokens: 100, Known: true}}
	pending := &pendingUsageWriterStub{}
	ledger := &openAIRecordUsageBillingRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.usageLogRepo = pending
	group := &Group{ID: 4, RateMultiplier: 1}
	input := &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{Model: "gpt-5.1", InputTokenProtection: &InputTokenProtection{Limit: 2}, ChatUsage: &OpenAIChatUsageEvidence{Rounds: []*OpenAIChatUsageRound{round}}},
		User: &User{ID: 1}, Account: &Account{ID: 3}, APIKey: &APIKey{ID: 2, Group: group, GroupID: &group.ID}}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Zero(t, ledger.calls)
	require.NotNil(t, pending.log)
	e := pending.log.PendingEvidence
	require.NotNil(t, e)
	require.Nil(t, e.InputTokens)
	require.Equal(t, 10, e.CacheReadTokens)
	require.Equal(t, 5, e.CacheCreationTokens)
	require.Equal(t, 7, *e.OutputTokens)
}

func TestOpenAIInputProtectionWSMergedTariffs(t *testing.T) {
	fixed := &OpenAIForwardResult{Model: "fixed", Usage: OpenAIUsage{InputTokens: 100, CacheReadInputTokens: 10}}
	metered := &OpenAIForwardResult{Model: "metered", Usage: OpenAIUsage{InputTokens: 40, CacheReadInputTokens: 5}}
	other := *metered
	for _, leaves := range [][]*OpenAIForwardResult{{fixed, metered, &other}, {metered, &other, fixed}} {
		turn := &openAIWSTurnInputProtection{evidence: &InputTokenProtection{Limit: 12}, merged: leaves, price: func(_ int, r OpenAIForwardResult) bool { return r.Model == "metered" }}
		raw := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":180,"output_tokens":3,"total_tokens":183,"input_tokens_details":{"cached_tokens":20}}}}`)
		wire := turn.project(raw)
		require.EqualValues(t, 112, gjson.GetBytes(wire, "response.usage.input_tokens").Int())
		require.EqualValues(t, 115, gjson.GetBytes(wire, "response.usage.total_tokens").Int())
		require.EqualValues(t, 20, gjson.GetBytes(wire, "response.usage.input_tokens_details.cached_tokens").Int())
	}
}
