//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type inflightPolicyReadCounter struct {
	SettingRepository
	reads int
	fail  bool
}

func (r *inflightPolicyReadCounter) GetValue(ctx context.Context, key string) (string, error) {
	if key == SettingKeyOpenAIFastPolicySettings {
		r.reads++
		if r.fail {
			return "", errors.New("settings unavailable")
		}
	}
	return r.SettingRepository.GetValue(ctx, key)
}

func TestInflightPolicyContextFreezesReadErrorFallback(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	key := newInflightUltrafastKey(group)
	estimators := newInflightUltrafastEstimators(t, group, nil, nil, nil)
	svc := estimators["openai"].(*OpenAIGatewayService)
	counter := &inflightPolicyReadCounter{SettingRepository: svc.settingService.settingRepo, fail: true}
	svc.settingService.settingRepo = counter
	request := inflightUltrafastRequest("gpt-6.1-sol", "")
	ctx := svc.PrepareInflightEstimateContext(inflightUltrafastContext(key), key, request)
	estimate, priced := svc.EstimateInflightReservation(ctx, key, request)
	require.True(t, priced)
	require.InDelta(t, inflightUltrafastStandardCost, estimate, 1e-10)
	body, err := svc.applyOpenAIFastPolicyToBody(ctx,
		&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, request.Model, request.Body)
	require.NoError(t, err)
	require.Nil(t, extractOpenAIServiceTierFromBody(body))
	require.Equal(t, 1, counter.reads, "freeze the existing pass-through fallback instead of rereading settings mid-request")
	require.NotNil(t, openAIFastPolicySettingsFromContext(ctx))
}

func TestInflightPolicyContextUsesOneSnapshotForEstimateAndForward(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	key := newInflightUltrafastKey(group)
	policy := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
		ServiceTier: OpenAIFastTierMissing, Scope: BetaPolicyScopeAll, Action: OpenAIFastPolicyActionForcePriority,
	}}}
	for name, estimator := range newInflightUltrafastEstimators(t, group, nil, policy, nil) {
		t.Run(name, func(t *testing.T) {
			preparer, ok := estimator.(interface {
				PrepareInflightEstimateContext(context.Context, *APIKey, InflightEstimateRequest) context.Context
			})
			require.True(t, ok, "share the policy snapshot with the handler and subsequent forwarding")
			var settings *SettingService
			switch svc := estimator.(type) {
			case *GatewayService:
				settings = svc.settingService
			case *OpenAIGatewayService:
				settings = svc.settingService
			}
			counter := &inflightPolicyReadCounter{SettingRepository: settings.settingRepo}
			settings.settingRepo = counter
			request := inflightUltrafastRequest("gpt-6.1-sol", "")
			ctx := preparer.PrepareInflightEstimateContext(inflightUltrafastContext(key), key, request)
			estimate, priced := estimator.EstimateInflightReservation(ctx, key, request)
			require.True(t, priced)
			require.InDelta(t, inflightUltrafastStandardCost*2, estimate, 1e-10)
			forwarder := &OpenAIGatewayService{settingService: settings}
			body, err := forwarder.applyOpenAIFastPolicyToBody(ctx,
				&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, request.Model, request.Body)
			require.NoError(t, err)
			require.Equal(t, "priority", optionalStringValue(extractOpenAIServiceTierFromBody(body)))
			require.Equal(t, 1, counter.reads, "precharge and forwarding must not query the same settings twice")
			require.Same(t, ctx, preparer.PrepareInflightEstimateContext(ctx, key, request))
			require.Equal(t, 1, counter.reads)
		})
	}
}
