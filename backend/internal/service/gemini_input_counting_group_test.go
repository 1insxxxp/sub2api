//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiInputCountingTestGroup(id int64, enabled bool) *Group {
	return &Group{ID: id, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, GeminiInputCountingEnabled: enabled}
}

func geminiInputCountingGroupAttempt(t *testing.T, group *Group, account *Account) (*GeminiUpstreamAttempt, *gin.Context) {
	t.Helper()
	c := newGeminiAttemptTestContext(t)
	if group != nil {
		c.Set("api_key", &APIKey{GroupID: &group.ID, Group: group})
		ctx, _ := WithGatewayTokenRequestPricing(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Request = c.Request.WithContext(ctx)
	}
	return BeginGeminiUpstreamAttempt(c, account, inputCountingTestModel, true), c
}

func TestGeminiInputCountingGroupAllAdapters(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		for _, stream := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/group=%t", path, stream, enabled), func(t *testing.T) {
					result, rec := callGeminiInputCountingForward(t, path, stream, false,
						`{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9,"thoughtsTokenCount":2,"totalTokenCount":1011}`,
						geminiInputCountingTestGroup(71, enabled))
					require.Equal(t, 1000, result.Usage.InputTokens)
					require.Equal(t, 11, result.Usage.OutputTokens)
					input := geminiInputCountingCustomerInput(t, path, stream, rec.Body.String())
					if enabled {
						require.NotNil(t, result.GeminiInputCounting)
						require.Equal(t, result.GeminiInputCounting.CandidateInputTokens, input)
					} else {
						require.Nil(t, result.GeminiInputCounting)
						require.Equal(t, 1000, input)
					}
				})
			}
		}
	}
}

func TestGeminiInputCountingGroupSharedAccountAndExplicitOff(t *testing.T) {
	account := inputCountingTestAccount() // A legacy account opt-in cannot bypass a group's off switch.
	before, err := json.Marshal(account.Extra)
	require.NoError(t, err)
	for _, enabled := range []bool{true, false, true, false} {
		attempt, _ := geminiInputCountingGroupAttempt(t, geminiInputCountingTestGroup(71, enabled), account)
		attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
		require.Equal(t, enabled, attempt.inputCounting != nil)
		if enabled {
			require.Nil(t, attempt.inputFallback.CountRequest)
			require.Empty(t, attempt.inputFallback.CalibrationKey)
		}
	}
	after, err := json.Marshal(account.Extra)
	require.NoError(t, err)
	require.Equal(t, before, after, "request-local settings must never mutate a shared account")
}

func TestGeminiInputCountingGroupUsesFrozenBillingParent(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			parent := geminiInputCountingTestGroup(71, enabled)
			parent.Platform = PlatformComposite
			c := newGeminiAttemptTestContext(t)
			ctx, _ := WithGatewayTokenRequestPricing(context.WithValue(c.Request.Context(), ctxkey.Group, parent))
			source := geminiInputCountingTestGroup(72, !enabled)
			c.Request = c.Request.WithContext(context.WithValue(ctx, ctxkey.Group, source))
			c.Set("api_key", &APIKey{GroupID: &source.ID, Group: source})
			attempt := BeginGeminiUpstreamAttempt(c, inputCountingTestAccount(), inputCountingTestModel, true)
			attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
			require.Equal(t, enabled, attempt.inputCounting != nil, "fallback/source settings cannot alter the billed parent group's switch")
		})
	}
}

func TestGeminiInputCountingGroupRejectsMissingOrUntrustedGroup(t *testing.T) {
	for _, group := range []*Group{
		nil,
		{ID: 71, Platform: PlatformGemini, GeminiInputCountingEnabled: true},
		{ID: 72, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, GeminiInputCountingEnabled: true},
		{ID: 71, Platform: PlatformGemini, Status: "inactive", Hydrated: true, GeminiInputCountingEnabled: true},
		{ID: 71, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, GeminiInputCountingEnabled: true},
	} {
		c := newGeminiAttemptTestContext(t)
		id := int64(71)
		c.Set("api_key", &APIKey{GroupID: &id, Group: group})
		attempt := BeginGeminiUpstreamAttempt(c, inputCountingTestAccount(), inputCountingTestModel, true)
		attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
		require.Nil(t, attempt.inputCounting)
	}
}

func TestGeminiInputCountingGroupKeepsAccountAndFinalRequestGuards(t *testing.T) {
	for _, mutate := range []func(*Account){
		func(a *Account) { a.ID = 0 },
		func(a *Account) { a.Platform = PlatformAntigravity },
		func(a *Account) { a.Type = AccountTypeServiceAccount },
		func(a *Account) { a.Type = AccountTypeOAuth },
	} {
		account := inputCountingTestAccount()
		mutate(account)
		attempt, _ := geminiInputCountingGroupAttempt(t, geminiInputCountingTestGroup(71, true), account)
		attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
		require.Nil(t, attempt.inputCounting)
	}
	for _, body := range []string{
		strings.TrimSuffix(inputCountingTestBody, "}") + `,"tools":[]}`,
		strings.Replace(inputCountingTestBody, `"text":"请仅回复OK，不要解释。"`, `"inlineData":{"mimeType":"image/png","data":"x"}`, 1),
		strings.TrimSuffix(inputCountingTestBody, "}") + `,"cachedContent":"cachedContents/x"}`,
	} {
		attempt, _ := geminiInputCountingGroupAttempt(t, geminiInputCountingTestGroup(71, true), inputCountingTestAccount())
		attempt.ObserveRequest(inputCountingTestRequest(t, body))
		require.Nil(t, attempt.inputCounting)
	}
	attempt, _ := geminiInputCountingGroupAttempt(t, geminiInputCountingTestGroup(71, true), inputCountingTestAccount())
	wrong, err := http.NewRequest(http.MethodPost, "https://fixture.example/v1beta/models/different:generateContent", strings.NewReader(inputCountingTestBody))
	require.NoError(t, err)
	attempt.ObserveRequest(wrong)
	require.Nil(t, attempt.inputCounting, "the actual final model endpoint must match")
}

func TestGeminiInputCountingGroupPreservesNativeCountRecovery(t *testing.T) {
	for _, body := range []string{inputCountingTestBody, strings.TrimSuffix(inputCountingTestBody, "}") + `,"tools":[]}`} {
		account := inputCountingTestAccount()
		account.Extra = nil // A group discount does not prove the channel's count endpoint is unavailable.
		attempt, _ := geminiInputCountingGroupAttempt(t, geminiInputCountingTestGroup(71, true), account)
		request := inputCountingTestRequest(t, body)
		baseline := prepareGeminiInputUsageFallback(request, account.ID, inputCountingTestModel)
		require.NotNil(t, baseline.CountRequest)
		require.NotEmpty(t, baseline.CalibrationKey)
		attempt.ObserveRequest(request)
		require.NotNil(t, attempt.inputFallback.CountRequest, "group discount must keep same-account native countTokens recovery")
		require.Equal(t, baseline.CalibrationKey, attempt.inputFallback.CalibrationKey)
	}
}

func TestGeminiInputCountingGroupSystemCustomDoesNotInheritSource(t *testing.T) {
	for _, resolution := range []any{
		SystemCustomGroupResolution{BillingGroupID: 71, SourceGroupID: 72, PublicModel: "subscription-model", SourceModel: inputCountingTestModel, SourcePlatform: PlatformGemini},
		"invalid-resolution",
	} {
		c := newGeminiAttemptTestContext(t)
		source := geminiInputCountingTestGroup(72, true)
		ctx := context.WithValue(c.Request.Context(), ctxkey.SystemCustomGroupResolution, resolution)
		ctx, _ = WithGatewayTokenRequestPricing(context.WithValue(ctx, ctxkey.Group, source))
		c.Request = c.Request.WithContext(ctx)
		c.Set("api_key", &APIKey{GroupID: &source.ID, Group: source})
		attempt := BeginGeminiUpstreamAttempt(c, inputCountingTestAccount(), inputCountingTestModel, true)
		attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
		require.Nil(t, attempt.inputCounting, "system-custom containers have no opt-in UI and cannot inherit a source group's discount")
	}
}

func TestGeminiInputCountingGroupFreezesSwitchAtAttemptStart(t *testing.T) {
	group := geminiInputCountingTestGroup(71, true)
	attempt, _ := geminiInputCountingGroupAttempt(t, group, inputCountingTestAccount())
	group.GeminiInputCountingEnabled = false
	attempt.ObserveRequest(inputCountingTestRequest(t, inputCountingTestBody))
	require.NotNil(t, attempt.inputCounting, "only later requests may see a saved switch change")
}
