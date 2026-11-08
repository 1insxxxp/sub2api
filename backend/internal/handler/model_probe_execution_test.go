//go:build unit

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func probeExecutionFixture(t *testing.T, platform string, count int) (*GatewayHandler, *modelProbeRoute, *probeGatewaySlots) {
	t.Helper()
	group := &service.Group{ID: 7, Platform: platform, Status: service.StatusActive, Hydrated: true}
	accounts := make([]service.Account, count)
	for i := range accounts {
		accounts[i] = probeGatewayAccount(int64(71+i), 7, platform, "public-model", "native-model")
		accounts[i].Priority = i
		accounts[i].Credentials["api_key"] = "fixture-only"
		accounts[i].Credentials["base_url"] = "https://probe.example.invalid"
	}
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: accounts, 8: {probeGatewayAccount(81, 8, platform, "public-model", "private-other")}}}
	slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true, true, true, true, true, true, true, true}}}
	h := probeGatewayFixture(repo, nil, nil, slots)
	h.maxAccountSwitches = 3
	h.maxAccountSwitchesGemini = 3
	route, err := h.prepareModelProbe(context.Background(), group, "public-model")
	require.NoError(t, err)
	return h, route, slots
}

func completeProbeForward() *modelProbeForwardResult {
	return &modelProbeForwardResult{Outcome: &service.ResponseOutcome{CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200, HasText: true, StreamCompleted: true}}
}

func TestModelProbeExecutionSwitchesWithinSelectedGroup(t *testing.T) {
	for _, platform := range []string{service.PlatformGemini, service.PlatformAnthropic, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h, r, slots := probeExecutionFixture(t, platform, 2)
			var attempted []int64
			result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, a *service.Account) (*modelProbeForwardResult, error) {
				attempted = append(attempted, a.ID)
				if len(attempted) == 1 {
					return nil, &service.UpstreamFailoverError{StatusCode: 502}
				}
				return completeProbeForward(), nil
			})
			require.NoError(t, err)
			require.Equal(t, "success", result.Status)
			require.Equal(t, []int64{71, 72}, attempted)
			require.Equal(t, 2, slots.accountReleaseCalls)
		})
	}
}

func TestModelProbeExecutionNonRetryableFailureStops(t *testing.T) {
	for _, failure := range []error{errors.New("request validation failed"), &service.UpstreamFailoverError{StatusCode: 400, NextAccountAction: service.NextAccountStop}} {
		h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 2)
		calls := 0
		result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
			calls++
			return nil, failure
		})
		require.NoError(t, err)
		require.Equal(t, "failed", result.Status)
		require.Equal(t, 1, calls)
		require.Equal(t, 1, slots.accountReleaseCalls)
	}
}

func TestModelProbeExecutionDoesNotReplayPartialOutput(t *testing.T) {
	h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 2)
	calls := 0
	result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, _ *service.Account) (*modelProbeForwardResult, error) {
		calls++
		_, _ = fmt.Fprint(c.Writer, "data: visible content\n\n")
		return nil, &service.UpstreamFailoverError{StatusCode: 502}
	})
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, slots.accountReleaseCalls)
}

func TestModelProbeExecutionFirstOutputTraversesBeyondOrdinaryLimit(t *testing.T) {
	for _, platform := range []string{service.PlatformGemini, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h, r, slots := probeExecutionFixture(t, platform, 3)
			h.maxAccountSwitches, h.maxAccountSwitchesGemini = 1, 1
			var attempted []int64
			result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, a *service.Account) (*modelProbeForwardResult, error) {
				attempted = append(attempted, a.ID)
				if len(attempted) < 3 {
					return nil, &service.UpstreamFailoverError{StatusCode: 504, Reason: "first_output_timeout", PreferNextAccount: true, SafeToFailoverAfterWrite: true}
				}
				return completeProbeForward(), nil
			})
			require.NoError(t, err)
			require.Equal(t, "success", result.Status)
			require.Equal(t, []int64{71, 72, 73}, attempted)
			require.Equal(t, 3, slots.accountReleaseCalls)
		})
	}
}

func TestModelProbeExecutionHonorsSwitchLimit(t *testing.T) {
	for _, platform := range []string{service.PlatformGemini, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h, r, slots := probeExecutionFixture(t, platform, 3)
			h.maxAccountSwitches, h.maxAccountSwitchesGemini = 1, 1
			calls := 0
			result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
				calls++
				return nil, &service.UpstreamFailoverError{StatusCode: 502}
			})
			require.NoError(t, err)
			require.Equal(t, "failed", result.Status)
			require.Equal(t, 2, calls)
			require.Equal(t, 2, slots.accountReleaseCalls)
		})
	}
}

func TestModelProbeExecutionCancellationReleasesSlot(t *testing.T) {
	h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 2)
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	r.ctx = ctx
	calls := 0
	result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
		calls++
		cancel()
		return nil, context.Canceled
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Status)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, slots.accountReleaseCalls)
}

func TestModelProbeExecutionRequiresCompleteOutput(t *testing.T) {
	for _, outcome := range []*service.ResponseOutcome{nil, {CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200, HasText: true}, {CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true}, {CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 500, HasText: true, StreamCompleted: true}} {
		h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 1)
		result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
			return &modelProbeForwardResult{Outcome: outcome}, nil
		})
		require.NoError(t, err)
		require.Equal(t, "failed", result.Status)
		require.Equal(t, 1, slots.accountReleaseCalls)
	}
}

func TestModelProbeResponseCaptureIsBounded(t *testing.T) {
	w := newModelProbeResponseWriter()
	b := make([]byte, 1024*1024)
	n, err := w.Write(b)
	require.NoError(t, err)
	require.Len(t, w.body, modelProbeResponseLimit)
	require.True(t, w.overflow)
	require.Equal(t, len(b), n)
	require.Equal(t, http.StatusOK, w.status)
}

func TestModelProbeExecutionKeepsActorExclusionsAcrossRetries(t *testing.T) {
	for _, platform := range []string{service.PlatformGemini, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h, r, _ := probeExecutionFixture(t, platform, 3)
			r.ctx = service.WithGroupAccountExclusions(r.ctx, 4, service.GroupAccountExclusions{7: {71}, 8: {72}})
			var attempted []int64
			result, err := h.executeModelProbe(r, func(_ *gin.Context, _ *modelProbeRoute, a *service.Account) (*modelProbeForwardResult, error) {
				attempted = append(attempted, a.ID)
				if len(attempted) == 1 {
					return nil, &service.UpstreamFailoverError{StatusCode: 502}
				}
				return completeProbeForward(), nil
			})
			require.NoError(t, err)
			require.Equal(t, "success", result.Status)
			require.Equal(t, []int64{72, 73}, attempted)
		})
	}
}

func TestModelProbeExecutionSameAccountRetryThenSwitch(t *testing.T) {
	for _, platform := range []string{service.PlatformGemini, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h, r, slots := probeExecutionFixture(t, platform, 2)
			var attempted []int64
			result, err := h.executeModelProbe(r, func(_ *gin.Context, _ *modelProbeRoute, a *service.Account) (*modelProbeForwardResult, error) {
				attempted = append(attempted, a.ID)
				if len(attempted) < 3 {
					return nil, &service.UpstreamFailoverError{StatusCode: 502, RetryableOnSameAccount: true, SameAccountRetryMax: 1, SameAccountRetryDelay: time.Millisecond, RequestScopedTransient: true}
				}
				return completeProbeForward(), nil
			})
			require.NoError(t, err)
			require.Equal(t, "success", result.Status)
			require.Equal(t, []int64{71, 71, 72}, attempted)
			require.Equal(t, 3, slots.accountReleaseCalls)
		})
	}
}

func TestModelProbeExecutionAcceptedJobCannotBeReplayed(t *testing.T) {
	h, r, slots := probeExecutionFixture(t, service.PlatformOpenAI, 2)
	calls := 0
	result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, _ *service.Account) (*modelProbeForwardResult, error) {
		calls++
		service.MarkResponseCommitted(c)
		return nil, &service.UpstreamFailoverError{StatusCode: 502, SafeToFailoverAfterWrite: true}
	})
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, slots.accountReleaseCalls)
}

func TestModelProbeExecutionFirstOutputBudgetIsTimeout(t *testing.T) {
	h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 2)
	h.settingService = service.NewSettingService(&traversalTimeoutSettingsRepo{}, nil)
	calls := 0
	result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, _ *service.Account) (*modelProbeForwardResult, error) {
		calls++
		select {
		case <-c.Request.Context().Done():
		case <-time.After(3 * time.Second):
			t.Fatal("first-output total budget not applied")
		}
		return nil, c.Request.Context().Err()
	})
	require.NoError(t, err)
	require.Equal(t, "timeout", result.Status, "budget cancellation is not a user cancellation")
	require.Equal(t, 1, calls)
	require.Equal(t, 1, slots.accountReleaseCalls)
}

func TestModelProbeExecutionFreezesPolicyAcrossAttempts(t *testing.T) {
	h, r, _ := probeExecutionFixture(t, service.PlatformGemini, 2)
	h.settingService = service.NewSettingService(&traversalTimeoutSettingsRepo{}, nil)
	var started time.Time
	calls := 0
	result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, _ *service.Account) (*modelProbeForwardResult, error) {
		calls++
		policy, ok := c.Get("sub2api.model_first_output_request_policy")
		require.True(t, ok)
		require.Equal(t, 1, policy.(service.ModelFirstOutputTimeoutPolicy).SwitchSeconds)
		value, _ := c.Get("sub2api.model_first_output_budget_start")
		if calls == 1 {
			started = value.(time.Time)
			h.settingService = nil
			c.Set("attempt-private-key", "first-attempt")
			return nil, &service.UpstreamFailoverError{StatusCode: 504, Reason: "first_output_timeout", PreferNextAccount: true}
		}
		require.Equal(t, started, value)
		_, exists := c.Get("attempt-private-key")
		require.False(t, exists)
		return completeProbeForward(), nil
	})
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Equal(t, 2, calls)
}

func TestModelProbeExecutionDoesNotReusePreviousTimeoutReason(t *testing.T) {
	h, r, _ := probeExecutionFixture(t, service.PlatformGemini, 2)
	calls := 0
	result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
		calls++
		if calls == 1 {
			return nil, &service.UpstreamFailoverError{StatusCode: 504, Reason: "first_output_timeout", PreferNextAccount: true}
		}
		return nil, errors.New("last attempt failed without a timeout")
	})
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
}

func TestModelProbeExecutionNativeBudgetExhaustion(t *testing.T) {
	h, r, _ := probeExecutionFixture(t, service.PlatformAnthropic, 2)
	result, err := h.executeModelProbe(r, func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error) {
		return nil, &service.UpstreamFailoverError{StatusCode: 504, Reason: "first_output_budget_exhausted", NextAccountAction: service.NextAccountStop}
	})
	require.NoError(t, err)
	require.Equal(t, "timeout", result.Status)
}

func TestModelProbeExecutionRetainsNativeBudgetOnly(t *testing.T) {
	h, r, _ := probeExecutionFixture(t, service.PlatformAnthropic, 2)
	start := time.Now()
	calls := 0
	result, err := h.executeModelProbe(r, func(c *gin.Context, _ *modelProbeRoute, _ *service.Account) (*modelProbeForwardResult, error) {
		calls++
		if calls == 1 {
			c.Set("sub2api.model_first_output_budget_start", start)
			c.Set("sub2api.model_first_output_budget_deadline", start.Add(time.Minute))
			c.Set("first-attempt-only", true)
			return nil, &service.UpstreamFailoverError{StatusCode: 502}
		}
		value, ok := c.Get("sub2api.model_first_output_budget_start")
		require.True(t, ok)
		require.Equal(t, start, value)
		value, ok = c.Get("sub2api.model_first_output_budget_deadline")
		require.True(t, ok)
		require.Equal(t, start.Add(time.Minute), value)
		_, ok = c.Get("first-attempt-only")
		require.False(t, ok)
		return completeProbeForward(), nil
	})
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
}

func TestModelProbeExecutionProductionSelectionAndForwardFailover(t *testing.T) {
	h, r, slots := probeExecutionFixture(t, service.PlatformGemini, 2)
	u := &modelProbeForwardHTTP{}
	u.respond = func(*http.Request) (*http.Response, error) {
		if len(u.requests) == 1 {
			return modelProbeHTTPResponse(502, `{"error":{"code":502,"message":"bad gateway"}}`), nil
		}
		resp := modelProbeHTTPResponse(200, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"OK\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":1,\"totalTokenCount\":11}}\n\n")
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	}
	h.geminiCompatService = service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, u, nil, &config.Config{RunMode: config.RunModeSimple})
	result, err := h.ExecuteForProbe(r.ctx, r.group, r.publicModel)
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, u.requests, 2)
	require.Equal(t, []int64{71, 72}, slots.ids)
	require.Equal(t, 2, slots.accountReleaseCalls)
	for _, request := range u.requests {
		require.Equal(t, "/v1beta/models/native-model:streamGenerateContent", request.URL.Path)
	}
}
