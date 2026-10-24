//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

// These structural interfaces let the regression fail on the missing behavior,
// rather than failing to compile before the optional estimator is implemented.
type imageStudioPricingEstimatorTest interface {
	EstimateImageStudioCost(context.Context, int64, *Group, string, string, int) (float64, error)
}

type imageStudioUsageEstimatorTest interface {
	EstimateImageStudioUsageCost(context.Context, *OpenAIRecordUsageInput) (float64, error)
}

type imageStudioPricingGatewayTest struct {
	*imageStudioGatewayStub
	pricing    *OpenAIGatewayService
	quoteErr   error
	quoteCalls int
}

func (g *imageStudioPricingGatewayTest) EstimateImageStudioCost(ctx context.Context, userID int64, group *Group, model, size string, count int) (float64, error) {
	g.quoteCalls++
	if g.quoteErr != nil {
		return 0, g.quoteErr
	}
	estimator, ok := any(g.pricing).(imageStudioPricingEstimatorTest)
	if !ok {
		return 0, errors.New("image studio pricing estimator is not implemented")
	}
	return estimator.EstimateImageStudioCost(ctx, userID, group, model, size, count)
}

func (g *imageStudioPricingGatewayTest) EstimateImageStudioUsageCost(ctx context.Context, input *OpenAIRecordUsageInput) (float64, error) {
	estimator, ok := any(g.pricing).(imageStudioUsageEstimatorTest)
	if !ok {
		return 0, errors.New("image studio usage estimator is not implemented")
	}
	return estimator.EstimateImageStudioUsageCost(ctx, input)
}

func newImageStudioPricingTestGateway(t *testing.T, group *Group) *OpenAIGatewayService {
	t.Helper()
	pricing := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{inserted: true}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	pricing.resolver = newOpenAIImageChannelPricingResolverForTest(t, group.ID, "nai-diffusion-4-5-full", 0.06)
	pricing.channelService = pricing.resolver.channelService
	cache := pricing.channelService.cache.Load().(*channelCache)
	cache.pricingByGroupModel[channelModelKey{groupID: group.ID, model: "nai-diffusion-5-full"}] = &ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.25)}
	return pricing
}

func newImageStudioPricingTestService(t *testing.T) (*ImageStudioService, *ImageStudioGatewayExecutor, *imageStudioAPIKeyProviderStub, *Group) {
	t.Helper()
	group := &Group{ID: 144, Name: "NAI", Platform: PlatformOpenAI, Status: StatusActive, AllowImageGeneration: true, RateMultiplier: 1}
	pricing := newImageStudioPricingTestGateway(t, group)
	provider := &imageStudioAPIKeyProviderStub{apiKey: &APIKey{ID: 9, UserID: 7, GroupID: &group.ID, Group: group, User: &User{ID: 7}}}
	executor := NewImageStudioGatewayExecutor(provider, nil, nil, &imageStudioPricingGatewayTest{imageStudioGatewayStub: &imageStudioGatewayStub{}, pricing: pricing})
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{*group}})
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"nai-diffusion-4-5-full": "nai-diffusion-4-5-full", "nai-diffusion-5-full": "nai-diffusion-5-full"}}}}}, pricing.channelService)
	svc.SetExecutor(executor)
	return svc, executor, provider, group
}

func TestImageStudioPricingOptionsUsePerModelChannelPricesWithoutCreatingKeys(t *testing.T) {
	svc, executor, provider, _ := newImageStudioPricingTestService(t)
	options, err := svc.GetOptions(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, options.Groups, 1)
	for _, option := range options.Groups[0].Models {
		raw, err := json.Marshal(option)
		require.NoError(t, err)
		var model struct {
			Qualities []ImageStudioQualityOption    `json:"qualities"`
			Prices    []ImageStudioPricePreviewItem `json:"prices"`
		}
		require.NoError(t, json.Unmarshal(raw, &model))
		require.Len(t, model.Qualities, 3, option.Model)
		require.Len(t, model.Prices, len(SupportedImageStudioAspectRatios())*3, option.Model)
		want := map[string]float64{"nai-diffusion-4-5-full": 0.06, "nai-diffusion-5-full": 0.25}[option.Model]
		for _, quality := range model.Qualities {
			require.InDelta(t, want, quality.EstimatedCost, 1e-12, option.Model+"/"+quality.Quality)
		}
		for _, price := range model.Prices {
			require.InDelta(t, want, price.EstimatedCost, 1e-12, option.Model+"/"+price.Quality)
		}
	}
	require.Zero(t, provider.defaultCalls)
	require.Empty(t, provider.groupCalls)
	require.Empty(t, provider.keyCalls)
	require.NotEmpty(t, options.Groups[0].Prices, "legacy group prices stay available")
	require.Equal(t, 6, executor.gateway.(*imageStudioPricingGatewayTest).quoteCalls, "only three billing tiers are resolved per model")
}

func TestImageStudioPricingQueuedTaskUsesSelectedModelPrice(t *testing.T) {
	svc, _, _, group := newImageStudioPricingTestService(t)
	svc.SetTaskRepository(&imageStudioTaskRepoStub{})
	svc.workersStarted.Store(true) // Keep this task queued; this test never sends an upstream request.
	svc.taskQueue = make(chan int64, 1)
	task, err := svc.CreateTask(context.Background(), ImageStudioTaskCreateInput{Generate: &ImageStudioGenerateInput{UserID: 7, GroupID: &group.ID, Model: "nai-diffusion-5-full", Prompt: "cat", Quality: "4K", AspectRatio: "1:1"}})
	require.NoError(t, err)
	require.InDelta(t, 0.25, task.EstimatedCost, 1e-12)
}

func TestImageStudioPricingExecutionCostUsesActualModelAndImageCount(t *testing.T) {
	_, executor, _, _ := newImageStudioPricingTestService(t)
	gateway := executor.gateway.(*imageStudioPricingGatewayTest)
	gateway.forwardResult = &OpenAIForwardResult{Model: "nai-diffusion-5-full", BillingModel: "nai-diffusion-5-full", UpstreamModel: "nai-diffusion-5-full", ImageCount: 2, ImageSize: "4K"}
	result, err := executor.Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "nai-diffusion-5-full", Prompt: "cat", Size: "3840x2160", BillingTier: "4K"})
	require.NoError(t, err)
	require.InDelta(t, 0.5, result.Cost, 1e-12)
	require.Zero(t, gateway.recordCalls, "pricing must not charge")
}

func TestImageStudioPricingUsesBillingPrecedenceAndMultipliers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Group, *OpenAIGatewayService)
		model  string
		want   float64
	}{
		{name: "channel default", want: 0.06},
		{name: "channel tier", change: func(g *Group, s *OpenAIGatewayService) {
			s.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: g.ID, model: "nai-diffusion-4-5-full"}].Intervals = []PricingInterval{{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.08)}}
		}, want: 0.08},
		{name: "group legacy", change: func(g *Group, _ *OpenAIGatewayService) { g.ImagePrice2K = testPtrFloat64(0.04) }, want: 0.04},
		{name: "group legacy zero", change: func(g *Group, _ *OpenAIGatewayService) { g.ImagePrice2K = testPtrFloat64(0) }, want: 0},
		{name: "group model beats legacy and channel", change: func(g *Group, _ *OpenAIGatewayService) {
			g.ImagePrice2K = testPtrFloat64(0.04)
			g.ModelPricing = []ChannelModelPricing{{Models: []string{"nai-diffusion-4-5-full"}, BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.02)}}
		}, want: 0.02},
		{name: "channel free", change: func(g *Group, s *OpenAIGatewayService) {
			s.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: g.ID, model: "nai-diffusion-4-5-full"}].PerRequestPrice = testPtrFloat64(0)
		}, want: 0},
		{name: "group multiplier", change: func(g *Group, _ *OpenAIGatewayService) { g.RateMultiplier = 2 }, want: 0.12},
		{name: "user multiplier", change: func(g *Group, s *OpenAIGatewayService) {
			s.userGroupRateResolver = newUserGroupRateResolver(&openAIUserGroupRateRepoStub{rate: testPtrFloat64(0.5)}, nil, 0, nil, "test")
		}, want: 0.03},
		{name: "independent multiplier", change: func(g *Group, _ *OpenAIGatewayService) {
			g.RateMultiplier = 2
			g.ImageRateIndependent = true
			g.ImageRateMultiplier = 0.5
		}, want: 0.03},
		{name: "independent free", change: func(g *Group, _ *OpenAIGatewayService) { g.ImageRateIndependent = true; g.ImageRateMultiplier = 0 }, want: 0},
		{name: "group alias", model: "nai-diffusion-custom", change: func(g *Group, _ *OpenAIGatewayService) {
			g.ModelAliases = []domain.ModelAlias{{Alias: "nai-diffusion-custom", UpstreamModel: "nai-diffusion-4-5-full"}}
		}, want: 0.06},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &Group{ID: 144, Platform: PlatformOpenAI, RateMultiplier: 1}
			gateway := newImageStudioPricingTestGateway(t, group)
			if tc.change != nil {
				tc.change(group, gateway)
			}
			estimator, ok := any(gateway).(imageStudioPricingEstimatorTest)
			require.True(t, ok, "real gateway must provide the same pricing used by usage settlement")
			model := firstNonEmptyTrimmed(tc.model, "nai-diffusion-4-5-full")
			cost, err := estimator.EstimateImageStudioCost(context.Background(), 7, group, model, "2K", 1)
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost, 1e-12)
		})
	}
}

func TestImageStudioPricingTokenCompletionMatchesSettlementWithCachedInput(t *testing.T) {
	group := &Group{ID: 144, Platform: PlatformOpenAI, RateMultiplier: 1}
	gateway := newImageStudioPricingTestGateway(t, group)
	card := gateway.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: group.ID, model: "nai-diffusion-4-5-full"}]
	*card = ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(0.00001), OutputPrice: testPtrFloat64(0.00002), CacheReadPrice: testPtrFloat64(0.000001), CacheWritePrice: testPtrFloat64(0.000002), ImageInputPrice: testPtrFloat64(0.00003)}
	input := &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "studio-pricing-token-cache", Model: "nai-diffusion-4-5-full", ImageCount: 1, ImageSize: "1K", Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 20, CacheReadInputTokens: 700, CacheCreationInputTokens: 100, ImageInputTokens: 100, ImageCacheReadTokens: 80}},
		APIKey: &APIKey{ID: 9, UserID: 7, GroupID: &group.ID, Group: group}, User: &User{ID: 7, Balance: 10}, Account: &Account{ID: 11, Platform: PlatformOpenAI},
	}
	estimate, err := gateway.EstimateImageStudioUsageCost(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	actual := gateway.usageLogRepo.(*transactionalUsageLogRepoForTest).UsageLogRepository.(*openAIRecordUsageLogRepoStub).lastLog
	require.NotNil(t, actual)
	require.Equal(t, actual.ActualCost, estimate, "completion cost must not count cached tokens again as ordinary input")
}

func TestImageStudioPricingTokenEstimatePreservesZeroUserMultiplier(t *testing.T) {
	group := &Group{ID: 144, Platform: PlatformOpenAI, RateMultiplier: 1}
	gateway := newImageStudioPricingTestGateway(t, group)
	card := gateway.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: group.ID, model: "nai-diffusion-4-5-full"}]
	*card = ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(0.00001), OutputPrice: testPtrFloat64(0.00002)}
	gateway.userGroupRateResolver = newUserGroupRateResolver(&openAIUserGroupRateRepoStub{rate: testPtrFloat64(0)}, nil, 0, nil, "test")
	cost, err := gateway.EstimateImageStudioCost(context.Background(), 7, group, "nai-diffusion-4-5-full", "1K", 1)
	require.NoError(t, err)
	require.Zero(t, cost, "unknown token usage may be estimated, but an explicit free user multiplier must stay free")
}

func TestImageStudioPricingGeminiNativeQuoteUsesRequestedModel(t *testing.T) {
	const model = "gemini-3.1-flash-image-preview"
	group := &Group{ID: 144, Platform: PlatformGemini, RateMultiplier: 1}
	gateway := newImageStudioPricingTestGateway(t, group)
	cache := gateway.channelService.cache.Load().(*channelCache)
	cache.channelByGroupID[group.ID].BillingModelSource = BillingModelSourceChannelMapped
	cache.mappingByGroupModel[channelModelKey{groupID: group.ID, model: model}] = "nai-diffusion-5-full"
	cache.pricingByGroupModel[channelModelKey{groupID: group.ID, model: model}] = &ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.07)}
	cost, err := gateway.EstimateImageStudioCost(context.Background(), 7, group, model, "1K", 1)
	require.NoError(t, err)
	require.Equal(t, 0.07, cost, "native Gemini execution bills the requested model rather than the channel mapping")
}

func TestImageStudioPricingTaskQuoteErrorDoesNotPersistWrongEstimate(t *testing.T) {
	svc, executor, _, group := newImageStudioPricingTestService(t)
	pricingErr := errors.New("pricing temporarily unavailable")
	executor.gateway.(*imageStudioPricingGatewayTest).quoteErr = pricingErr
	repo := &imageStudioTaskRepoStub{}
	svc.SetTaskRepository(repo)
	_, err := svc.CreateTask(context.Background(), ImageStudioTaskCreateInput{Generate: &ImageStudioGenerateInput{UserID: 7, GroupID: &group.ID, Model: "nai-diffusion-5-full", Prompt: "cat", Quality: "1K", AspectRatio: "1:1"}})
	require.ErrorIs(t, err, pricingErr)
	require.Empty(t, repo.tasks)
}

func TestImageStudioPricingSuccessfulFreeTaskKeepsZeroCost(t *testing.T) {
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetExecutor(&imageStudioExecutorStub{result: &ImageStudioExecutionResult{ImageBytes: []byte("png-bytes"), MimeType: "image/png", Cost: 0}})
	svc.SetStorageFactory(func(context.Context, *ImageStudioSettings) (ImageStorage, error) {
		return &imageStudioStorageStub{publicURL: "https://assets.example.com/free.png"}, nil
	})
	image, _, cost, err := svc.generateTaskWithRecovery(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "nai-diffusion-5-full", Prompt: "cat", AspectRatio: "1:1", Quality: "1K"}, 0.25)
	require.NoError(t, err)
	require.Zero(t, image.Cost)
	require.Zero(t, cost, "a successful free image must not be replaced by a legacy nonzero estimate")
}
