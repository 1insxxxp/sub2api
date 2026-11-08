package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ExecuteForProbe uses production selection and forwarding without entering a
// public handler: those handlers also own customer admission and usage billing.
func (h *GatewayHandler) ExecuteForProbe(ctx context.Context, group *service.Group, model string) (service.ModelProbeExecution, error) {
	if h == nil || h.gatewayService == nil || group == nil {
		return service.ModelProbeExecution{}, service.ErrUserSupportUnavailable
	}
	route, err := h.prepareModelProbe(ctx, group, model)
	if err != nil {
		return service.ModelProbeExecution{}, err
	}
	return h.executeModelProbe(route, h.forwardModelProbe)
}

type modelProbeForwardFunc func(*gin.Context, *modelProbeRoute, *service.Account) (*modelProbeForwardResult, error)

func (h *GatewayHandler) executeModelProbe(r *modelProbeRoute, forward modelProbeForwardFunc) (service.ModelProbeExecution, error) {
	r.sessionHash = "model-probe:" + uuid.NewString()
	r.ctx = service.WithModelProbeExecution(r.ctx)
	if !domain.UsesOpenAIGateway(r.platform) {
		r.ctx = h.gatewayService.WithSingleAntigravityAccountGroupProbe(r.ctx, &r.group.ID)
	}
	root := newModelProbeAttemptContext(r.ctx, r.group, nil, r.sessionHash)
	service.SnapshotModelFirstOutputRequestPolicy(root, h.settingService, r.platform, r.routeModel)
	cleanup := service.StartModelFirstOutputTraversal(root, h.settingService, r.platform, r.routeModel, !r.imageIntent)
	defer cleanup()
	ctx := root.Request.Context()
	openAI := domain.UsesOpenAIGateway(r.platform)
	state := NewFailoverState(h.modelProbeMaxSwitches(r.platform), false)
	var oauth429 service.OpenAIOAuth429FailoverState
	finish := func() (service.ModelProbeExecution, error) {
		return modelProbeExecutionOutcome(ctx, state.LastFailoverErr), nil
	}
	for {
		if ctx.Err() != nil {
			return finish()
		}
		// Do not carry an earlier attempt's collectors or cancellation context
		// into the next attempt. Only request-scoped policy and exclusions persist.
		r.ctx, r.excludedIDs = ctx, state.FailedAccountIDs
		selected, err := h.selectModelProbeAccount(r)
		if err != nil || selected == nil || selected.Account == nil {
			releaseProbeSelection(selected)
			if !openAI && state.HandleSelectionExhausted(ctx) == FailoverContinue {
				ctx = service.WithSingleAccountRetry(ctx, true, h.metadataBridgeEnabled())
				continue
			}
			return finish()
		}
		r.ctx = service.ContextWithSelectionProfitGate(r.ctx, selected)
		// Persist admission's request-scoped pricing gate, not any forwarder-
		// owned attempt state. Later candidates use the same frozen threshold.
		ctx = r.ctx
		release, err := h.acquireModelProbeSlot(r.ctx, selected)
		if err != nil {
			return finish()
		}
		release = wrapReleaseOnDone(ctx, release)
		admitted, err := h.finishModelProbeSelection(r, selected.Account, release)
		if err != nil {
			if release != nil {
				release()
			}
			if errors.Is(err, errModelProbeProfitVeto) && state.RecordProfitVeto(selected.Account.ID) == FailoverContinue {
				continue
			}
			if errors.Is(err, service.ErrModelProbeUnsupported) {
				return service.ModelProbeExecution{Status: "unsupported"}, nil
			}
			return finish()
		}
		account := admitted.Account
		if openAI {
			_ = h.openAIGatewayService.BindStickySessionAfterProfitAdmission(r.ctx, &r.group.ID, r.sessionHash, account.ID)
		} else {
			_ = h.gatewayService.BindStickySessionAfterProfitAdmission(r.ctx, &r.group.ID, r.sessionHash, account.ID)
		}
		c := newModelProbeAttemptContext(r.ctx, r.group, root.Keys, r.sessionHash)
		before := c.Writer.Size()
		result, forwardErr := func() (*modelProbeForwardResult, error) {
			if release != nil {
				defer release()
			}
			defer service.StopModelFirstOutputKeepaliveCommitted(c)
			return forward(c, r, account)
		}()
		service.CopyModelFirstOutputRequestState(c, root)
		if ctx.Err() != nil {
			return finish()
		}
		if forwardErr == nil && modelProbeForwardCompleted(result) && c.Writer.Status() < 400 {
			if openAI {
				h.openAIGatewayService.ReportOpenAIAccountScheduleResult(account, openAIAccountScheduleModel(c, account, r.forwardModel, false, nil), true, result.FirstTokenMs)
			}
			return service.ModelProbeExecution{Status: "success"}, nil
		}
		if errors.Is(forwardErr, service.ErrModelProbeUnsupported) {
			return service.ModelProbeExecution{Status: "unsupported"}, nil
		}
		if errors.Is(forwardErr, context.DeadlineExceeded) {
			return service.ModelProbeExecution{Status: "timeout"}, nil
		}
		var failure *service.UpstreamFailoverError
		if !errors.As(forwardErr, &failure) {
			if openAI && forwardErr != nil {
				h.openAIGatewayService.ReportOpenAIAccountScheduleResult(account, openAIAccountScheduleModel(c, account, r.forwardModel, false, nil), false, nil, forwardErr)
			}
			return modelProbeExecutionOutcome(ctx, nil), nil
		}
		state.LastFailoverErr = failure
		size := c.Writer.Size()
		if !openAI {
			size = service.ModelFirstOutputKeepaliveAdjustedWrittenSize(c)
		}
		if service.IsResponseCommitted(c) || !gatewayForwardMayFailoverAfterWrite(before, size, failure) {
			if openAI {
				h.openAIGatewayService.ObserveOpenAIAccountHealthFailure(ctx, account, forwardErr)
			}
			return finish()
		}
		if openAI {
			if failure.ShouldReportAccountScheduleFailure() {
				h.openAIGatewayService.ReportOpenAIAccountScheduleResult(account, openAIAccountScheduleModel(c, account, r.forwardModel, false, nil), false, nil, forwardErr)
			}
			if !h.handleModelProbeOpenAIFailure(ctx, state, account, failure, &oauth429) {
				return finish()
			}
		} else if state.HandleFailoverError(ctx, h.gatewayService, account.ID, account.Platform, account.GetPoolModeRetryCount(), failure) != FailoverContinue {
			return finish()
		}
	}
}

// Match the OpenAI HTTP handler's policy rather than applying native 503
// pool reopening or temporary-unscheduling semantics to OpenAI accounts.
func (h *GatewayHandler) handleModelProbeOpenAIFailure(ctx context.Context, state *FailoverState, account *service.Account, failure *service.UpstreamFailoverError, oauth429 *service.OpenAIOAuth429FailoverState) bool {
	if !failure.ShouldRetryNextAccount() {
		return false
	}
	if failure.Reason == service.GatewayFailureReason("first_output_timeout") {
		state.firstOutputTimeoutPoolTraversal = true
	}
	if sameAccountRetryAllowed(failure, state.SameAccountRetryCount[account.ID], effectiveSameAccountRetryLimit(failure, account)) {
		state.SameAccountRetryCount[account.ID]++
		return sleepWithContext(ctx, sameAccountRetryDelayFor(failure, state.SameAccountRetryCount[account.ID]))
	}
	h.openAIGatewayService.RecordOpenAIAccountSwitch()
	state.FailedAccountIDs[account.ID] = struct{}{}
	if shouldStopAccountSwitching(state.SwitchCount, state.MaxSwitches, state.firstOutputTimeoutPoolTraversal, failure) {
		return false
	}
	state.SwitchCount++
	return !h.openAIGatewayService.ShouldStopOpenAIOAuth429Failover(account, failure.StatusCode, state.SwitchCount, oauth429)
}

func (h *GatewayHandler) modelProbeMaxSwitches(platform string) int {
	if platform == service.PlatformGemini {
		if h.maxAccountSwitchesGemini > 0 {
			return h.maxAccountSwitchesGemini
		}
		return 9
	}
	if domain.UsesOpenAIGateway(platform) && h.cfg != nil {
		if h.cfg.Gateway.MaxAccountSwitches > 0 {
			return h.cfg.Gateway.MaxAccountSwitches
		}
		return 3 // NewOpenAIGatewayHandler's default.
	}
	if h.maxAccountSwitches > 0 {
		return h.maxAccountSwitches
	}
	if domain.UsesOpenAIGateway(platform) {
		return 3
	}
	return 10
}

func modelProbeExecutionOutcome(ctx context.Context, failure *service.UpstreamFailoverError) service.ModelProbeExecution {
	status := "failed"
	switch {
	case service.ModelFirstOutputTraversalBudgetExceeded(ctx), errors.Is(ctx.Err(), context.DeadlineExceeded):
		status = "timeout"
	case ctx.Err() != nil:
		status = "cancelled"
	case failure != nil && (failure.Reason == service.GatewayFailureReason("first_output_timeout") || failure.Reason == service.GatewayFailureReason("first_output_budget_exhausted")):
		status = "timeout"
	}
	return service.ModelProbeExecution{Status: status}
}

func modelProbeForwardCompleted(result *modelProbeForwardResult) bool {
	if result == nil {
		return false
	}
	if result.MediaSuccess {
		return result.Outcome == nil || (result.Outcome.HTTPStatus < 400 && result.Outcome.UpstreamStatus < 400 && (result.Outcome.DisconnectSource == "" || result.Outcome.DisconnectSource == service.DisconnectSourceNone) && (result.Outcome.UpstreamErrorKind == "" || result.Outcome.UpstreamErrorKind == service.UpstreamErrorNone))
	}
	return service.ClassifyUsageRequestStatus(result.Outcome) == service.UsageRequestCompleted
}

func newModelProbeAttemptContext(ctx context.Context, group *service.Group, keys map[string]any, sessionHash string) *gin.Context {
	w := newModelProbeResponseWriter()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Session_id", sessionHash)
	for key, value := range keys {
		c.Set(key, value)
	}
	middleware.SetAPIKeyAndGroupContext(c, &service.APIKey{Group: group, GroupID: &group.ID})
	return c
}

// Capture only a bounded prefix for async job IDs. Text/media output is private
// and discarded. The writer still reports its full size to prevent replay after
// semantic output, just like a real downstream connection.
const modelProbeResponseLimit = 64 << 10

type modelProbeResponseWriter struct {
	mu       sync.Mutex
	header   http.Header
	body     []byte
	status   int
	overflow bool
}

func newModelProbeResponseWriter() *modelProbeResponseWriter {
	return &modelProbeResponseWriter{header: make(http.Header)}
}
func (w *modelProbeResponseWriter) Header() http.Header { return w.header }
func (w *modelProbeResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = status
	}
}
func (w *modelProbeResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n := min(len(p), modelProbeResponseLimit-len(w.body))
	w.body = append(w.body, p[:n]...)
	w.overflow = w.overflow || n < len(p)
	return len(p), nil
}
func (w *modelProbeResponseWriter) Flush() { w.WriteHeader(http.StatusOK) }
