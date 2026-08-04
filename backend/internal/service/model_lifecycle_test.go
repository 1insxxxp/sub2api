package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const retiredClaudeHaiku35 = "claude-3-5-haiku-20241022"
const shutdownGemini20Flash = "gemini-2.0-flash"

func TestGetAvailableModels_FiltersRetiredAnthropicDirectModel(t *testing.T) {
	groupID := int64(81)
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {
			{
				ID:       1,
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{"model_mapping": map[string]any{
					retiredClaudeHaiku35:        retiredClaudeHaiku35,
					"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001",
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic)

	require.Equal(t, []string{"claude-haiku-4-5-20251001"}, models)
}

func TestGetAvailableModels_KeepsRetiredAnthropicIDFromBedrock(t *testing.T) {
	groupID := int64(82)
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {
			{
				ID:       2,
				Platform: PlatformAnthropic,
				Type:     AccountTypeBedrock,
				Credentials: map[string]any{"model_mapping": map[string]any{
					retiredClaudeHaiku35: retiredClaudeHaiku35,
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic)

	require.Equal(t, []string{retiredClaudeHaiku35}, models)
}

func TestGetAvailableModels_KeepsModelWhenBedrockAndDirectAccountsOverlap(t *testing.T) {
	groupID := int64(83)
	mapping := map[string]any{retiredClaudeHaiku35: retiredClaudeHaiku35}
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {
			{
				ID:          3,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{"model_mapping": mapping},
			},
			{
				ID:          4,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeBedrock,
				Credentials: map[string]any{"model_mapping": mapping},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic)

	require.Equal(t, []string{retiredClaudeHaiku35}, models)
}

func TestFetchUpstreamSupportedModels_FiltersRetiredAnthropicDirectModel(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"id":"claude-3-5-haiku-20241022"},
			{"id":"claude-haiku-4-5-20251001"}
		]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       3,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com/v1",
		},
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"claude-haiku-4-5-20251001"}, models)
}

func TestGetAvailableModels_FiltersShutdownGeminiModel(t *testing.T) {
	groupID := int64(84)
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {
			{
				ID:       5,
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{"model_mapping": map[string]any{
					shutdownGemini20Flash: shutdownGemini20Flash,
					"gemini-3.5-flash":    "gemini-3.5-flash",
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformGemini)

	require.Equal(t, []string{"gemini-3.5-flash"}, models)
}

func TestGetAvailableModels_DoesNotApplyGeminiShutdownsToAntigravity(t *testing.T) {
	groupID := int64(85)
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {
			{
				ID:       6,
				Platform: PlatformAntigravity,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{"model_mapping": map[string]any{
					shutdownGemini20Flash: shutdownGemini20Flash,
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformAntigravity)

	require.Contains(t, models, shutdownGemini20Flash)
}

func TestFetchUpstreamSupportedModels_FiltersShutdownGeminiModel(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"models":[
			{"name":"models/gemini-2.0-flash"},
			{"name":"models/gemini-3.5-flash"}
		]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       7,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com/v1beta",
		},
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-3.5-flash"}, models)
}
