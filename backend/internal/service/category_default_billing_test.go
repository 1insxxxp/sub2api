//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultRecordedCostMatchesDirectSourceAndKeepsAudit(t *testing.T) {
	for _, platform := range []string{PlatformGemini, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			sourceID, entryID := int64(104), int64(105)
			inputPrice, outputPrice := 0.001, 0.002
			group := &Group{ID: sourceID, Platform: platform, RateMultiplier: 0.8, ModelPricing: []ChannelModelPricing{{Models: []string{"gemini-4-argon"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice}}}
			var costs []float64
			for _, throughDefault := range []bool{false, true} {
				repo := &openAIRecordUsageLogRepoStub{inserted: true}
				svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
				svc.usageBillingRepo = billingRepo
				key := &APIKey{ID: 101, UserID: 102, Quota: 100, GroupID: &sourceID, Group: group}
				if throughDefault {
					key.CategoryDefaultGroupID = &entryID
				}
				result := &ForwardResult{RequestID: "default-cost", Model: "gemini-4-argon", UpstreamModel: "gemini-4-argon", Usage: ClaudeUsage{InputTokens: 673, OutputTokens: 10}, InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUpstream, Duration: time.Second}
				if platform == PlatformGemini {
					result.RequireMeasuredUsage = true
					result.MeasuredUsageComplete = true
					result.GeminiUsageFinalized = true
					result.GeminiUsageFallback = &GeminiUsageFallback{UsageFields: 15}
					result.GeminiInputCounting = &GeminiInputCountingEvidence{AccountID: 103, Policy: GeminiInputCountingVertexLikeDiscountV1, CandidateInputTokens: 421}
				}
				err := svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: key, User: &User{ID: 102}, Account: &Account{ID: 103, Platform: platform, Type: AccountTypeAPIKey}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceRequested, OriginalModel: "gemini-4-argon"}})
				require.NoError(t, err)
				require.NotNil(t, repo.lastLog)
				require.NotNil(t, billingRepo.lastCmd)
				require.Equal(t, sourceID, *repo.lastLog.GroupID)
				require.Equal(t, int64(101), repo.lastLog.APIKeyID)
				if throughDefault {
					require.Equal(t, entryID, *repo.lastLog.CategoryDefaultGroupID)
				} else {
					require.Nil(t, repo.lastLog.CategoryDefaultGroupID)
				}
				require.InDelta(t, repo.lastLog.ActualCost, billingRepo.lastCmd.BalanceCost, 1e-8)
				costs = append(costs, repo.lastLog.ActualCost)
			}
			require.InDelta(t, costs[0], costs[1], 1e-8, "binding the category entry cannot alter source billing or Gemini customer projection")
		})
	}
}

// The only HTTP transport in this test targets the isolated local server. Both
// bindings use the production Gemini adapters and the same source pricing path.
func TestCategoryDefaultFakeHTTPMatchesDirectGeminiBillingAndQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, sourcePlatform := range []string{PlatformGemini, PlatformComposite} {
		for _, adapter := range []string{"chat", "messages", "native"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%t", sourcePlatform, adapter, stream), func(t *testing.T) {
					var calls atomic.Int64
					var invalidPath atomic.Bool
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						if !strings.Contains(r.URL.Path, "gemini-4-argon:") || strings.Contains(r.URL.Path, "countTokens") {
							invalidPath.Store(true)
						}
						payload := `{"candidates":[{"content":{"parts":[{"text":"A complete answer."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9,"thoughtsTokenCount":2,"totalTokenCount":1011}}`
						w.Header().Set("X-Request-Id", fmt.Sprintf("isolated-category-test-%d", calls.Load()))
						w.Header().Set("Content-Type", "application/json")
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							payload = "data: " + payload + "\n\n"
						}
						_, _ = io.WriteString(w, payload)
					}))
					t.Cleanup(server.Close)
					target, err := url.Parse(server.URL)
					require.NoError(t, err)
					cfg := &config.Config{}
					cfg.Security.URLAllowlist.AllowInsecureHTTP = true
					forward := &GeminiMessagesCompatService{cfg: cfg, httpUpstream: &geminiRealHTTPUsageUpstream{client: server.Client(), target: target}}
					sourceID, entryID := int64(7001), int64(7002)
					inputPrice, outputPrice := 0.001, 0.002
					source := &Group{ID: sourceID, Platform: sourcePlatform, Status: StatusActive, Hydrated: true, IsExclusive: true, RateMultiplier: 0.75, GeminiInputCountingEnabled: true,
						ModelPricing: []ChannelModelPricing{{Models: []string{"gemini-4-argon"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice}}}
					user := &User{ID: 7003, Status: StatusActive, Balance: 100, RestrictPublicGroups: true, AllowedGroups: []int64{sourceID}}
					account := &Account{ID: 7004, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"base_url": server.URL, "api_key": "isolated-local-key"}}
					var directCost float64
					var directInput int
					for _, throughDefault := range []bool{false, true} {
						key := &APIKey{ID: 7005, UserID: user.ID, User: user, GroupID: &sourceID, Group: source, Quota: 100, RateLimit5h: 100}
						if throughDefault {
							key.GroupID, key.Group = &entryID, &Group{ID: entryID, IsCategoryDefault: true, Platform: PlatformComposite, Hydrated: true, Status: StatusActive}
							keys := &APIKeyService{userRepo: categoryAPIKeyUserRepo{user: user}, categoryDefaultGroups: &categoryAPIKeyResolverFake{source: source}}
							resolved, resolveErr := keys.ResolveCategoryDefaultGroupModel(context.Background(), key, "gemini-4-argon")
							require.NoError(t, resolveErr)
							key = resolved.APIKey
						}
						recorder := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(recorder)
						ctx := context.WithValue(context.WithValue(context.Background(), ctxkey.APIKey, key), ctxkey.Group, key.Group)
						ctx, pricingAt := WithGatewayTokenRequestPricing(ctx)
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+adapter, strings.NewReader("{}")).WithContext(ctx)
						c.Set("api_key", key)
						var result *ForwardResult
						var forwardErr error
						if adapter == "chat" {
							body := []byte(fmt.Sprintf(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"max_tokens":100,"stream":%t,"stream_options":{"include_usage":true}}`, stream))
							result, forwardErr = forward.ForwardAsChatCompletions(ctx, c, account, body)
						} else {
							result, forwardErr = callGeminiTraversalPath(t, adapter, forward, c, account, "gemini-4-argon", stream)
						}
						require.NoError(t, forwardErr)
						require.NotNil(t, result.GeminiInputCounting)
						customerInput := geminiInputCountingCustomerInput(t, adapter, stream, recorder.Body.String())
						require.Positive(t, customerInput)
						require.Less(t, customerInput, 1000)
						require.Equal(t, 1000, result.Usage.InputTokens, "raw upstream evidence remains intact")
						require.Equal(t, 11, result.Usage.OutputTokens)
						logRepo := &openAIRecordUsageLogRepoStub{inserted: true}
						billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
						usage := newGatewayRecordUsageServiceWithBillingRepoForTest(logRepo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
						err = usage.RecordUsage(ctx, &RecordUsageInput{Result: result, APIKey: key, User: key.User, Account: account, PricingAt: pricingAt, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceRequested, OriginalModel: "gemini-4-argon"}})
						require.NoError(t, err)
						require.Equal(t, sourceID, *logRepo.lastLog.GroupID)
						require.Equal(t, customerInput, logRepo.lastLog.InputTokens)
						require.Equal(t, key.ID, billing.lastCmd.APIKeyID)
						require.InDelta(t, logRepo.lastLog.ActualCost, billing.lastCmd.APIKeyQuotaCost, 1e-8)
						require.InDelta(t, billing.lastCmd.BalanceCost, billing.lastCmd.APIKeyRateLimitCost, 1e-8)
						if throughDefault {
							require.Equal(t, entryID, *logRepo.lastLog.CategoryDefaultGroupID)
							require.Equal(t, directInput, customerInput)
							require.InDelta(t, directCost, billing.lastCmd.BalanceCost, 1e-8)
						} else {
							directCost, directInput = billing.lastCmd.BalanceCost, customerInput
						}
					}
					require.EqualValues(t, 2, calls.Load(), "one generation per binding; no extra counting or inference request")
					require.False(t, invalidPath.Load(), "all requests use the selected model generation endpoint")
				})
			}
		}
	}
}
