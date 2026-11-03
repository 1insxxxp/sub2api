package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func accountContinuationBodyModel(body []byte) string { return gjson.GetBytes(body, "model").String() }
func accountContinuationBodyStream(body []byte) bool  { return gjson.GetBytes(body, "stream").Bool() }

type accountContinuationManagedKey struct{}

func accountContinuationShouldManage(ctx context.Context, account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	if ctx != nil {
		if active, _ := ctx.Value(accountContinuationManagedKey{}).(bool); active {
			return false
		}
	}
	_, configured := account.Extra["anti_truncation"]
	return configured
}

// runForwardWithAccountContinuation keeps authentication, model selection and
// measured usage capture in the existing one-round forwarder. Only the outer
// client protocol lifecycle and the internal continuation rounds are managed
// here. Each result is retained independently for recovery and tier pricing.
func runForwardWithAccountContinuation(
	ctx context.Context, c *gin.Context, account *Account, body []byte,
	protocol AccountContinuationProtocol, routeModel string, stream bool,
	forward func(context.Context, []byte) (*ForwardResult, error),
) (*ForwardResult, error) {
	return runAccountContinuation(ctx, c, account, body, protocol, routeModel, stream, forward,
		AggregateForwardContinuationResults,
		func(r *ForwardResult) AccountContinuationUsage {
			return AccountContinuationUsage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens,
				CacheReadInputTokens: r.Usage.CacheReadInputTokens, CacheCreationInputTokens: r.Usage.CacheCreationInputTokens}
		},
		func(r *ForwardResult, rounds int, elapsed time.Duration) {
			r.AntiTruncationRounds = rounds
			r.Duration = elapsed
		})
}

func runOpenAIWithAccountContinuation(
	ctx context.Context, c *gin.Context, account *Account, body []byte,
	protocol AccountContinuationProtocol, routeModel string, stream bool,
	forward func(context.Context, []byte) (*OpenAIForwardResult, error),
) (*OpenAIForwardResult, error) {
	return runAccountContinuation(ctx, c, account, body, protocol, routeModel, stream, forward,
		AggregateOpenAIContinuationResults,
		func(r *OpenAIForwardResult) AccountContinuationUsage {
			return AccountContinuationUsage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens,
				CacheReadInputTokens: r.Usage.CacheReadInputTokens, CacheCreationInputTokens: r.Usage.CacheCreationInputTokens}
		},
		func(r *OpenAIForwardResult, rounds int, elapsed time.Duration) {
			r.AntiTruncationRounds = rounds
			r.Duration = elapsed
		})
}

func runAccountContinuation[T any](
	ctx context.Context, c *gin.Context, account *Account, body []byte,
	protocol AccountContinuationProtocol, routeModel string, stream bool,
	forward func(context.Context, []byte) (*T, error),
	aggregate func([]*T) (*T, error), usageOf func(*T) AccountContinuationUsage,
	setSummary func(*T, int, time.Duration),
) (*T, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if managed, _ := ctx.Value(accountContinuationManagedKey{}).(bool); managed || c == nil || c.Request == nil {
		return forward(ctx, body)
	}
	legacy, _ := RawChatAntiTruncationPolicyFromContext(ctx)
	requestedModel := gatewayPublicModel(ctx, routeModel)
	policy, configured := ResolveAccountAntiTruncationPolicy(account, requestedModel, routeModel, legacy)
	if !configured {
		// Existing group policies retain their exact established behavior until
		// an administrator explicitly switches this account to versioned rules.
		return forward(ctx, body)
	}
	ctx = context.WithValue(ctx, accountContinuationManagedKey{}, true)
	ctx = WithRawChatAntiTruncationPolicy(ctx, domain.AntiTruncationPolicy{})
	originalRequest := c.Request
	c.Request = c.Request.WithContext(ctx)
	defer func() { c.Request = originalRequest }()
	if !policy.Enabled || !AccountContinuationRequestAllowed(protocol, body) {
		return forward(ctx, body)
	}
	budget := NewAccountContinuationBudget(policy, protocol, body)
	output := NewAccountContinuationOutput(c.Writer, protocol, stream)
	originalWriter := c.Writer
	defer func() { c.Writer = originalWriter }()
	started := time.Now()
	var rounds []*T
	var delivery []*DownstreamOutputTokenCollector
	var roundTexts []string
	var usage AccountContinuationUsage
	var assistant strings.Builder
	currentBody := body
	var finalErr error
	budgetOutput := 0
	for {
		collector := NewDownstreamOutputTokenCollector(routeModel)
		collector.bindCancellation(ctx.Done())
		output.base = &downstreamOutputTokenResponseWriter{ResponseWriter: originalWriter, collector: collector}
		writer := output.BeginRound()
		c.Writer = writer
		// EnsureResponseOutcomeCollector intentionally reuses a parent collector;
		// internal rounds require independent upstream terminal/usage evidence.
		roundCtx := context.WithValue(ctx, responseOutcomeCollectorContextKey{}, NewResponseOutcomeCollector(0, 0))
		c.Request = originalRequest.WithContext(roundCtx)
		result, err := forward(roundCtx, currentBody)
		c.Writer = originalWriter
		round, finishErr := writer.Finish()
		roundErr := errors.Join(err, finishErr)
		canceled := ctx.Err() != nil || collector.Frozen()
		if roundErr == nil {
			roundErr = ctx.Err()
			if roundErr == nil && canceled {
				roundErr = context.Canceled
			}
		}
		if result != nil {
			applyAccountContinuationOutcome(result, round, roundErr, canceled)
			if roundErr != nil {
				finalizeAccountContinuationResult(result, "", roundErr, canceled)
			}
			rounds = append(rounds, result)
			delivery = append(delivery, collector)
			roundTexts = append(roundTexts, round.Text)
			u := usageOf(result)
			usage.InputTokens += u.InputTokens
			usage.OutputTokens += u.OutputTokens
			usage.CacheReadInputTokens += u.CacheReadInputTokens
			usage.CacheCreationInputTokens += u.CacheCreationInputTokens
			usage.ReasoningTokens += u.ReasoningTokens
			if u.OutputTokens > 0 {
				budgetOutput += u.OutputTokens
			} else {
				budgetOutput += estimateTokensForText(round.Text)
			}
		}
		finalErr = roundErr
		canContinue := !canceled && ctx.Err() == nil && result != nil && !round.Unsafe && round.Truncated && strings.TrimSpace(round.Text) != "" &&
			budget.CanContinue(time.Now(), len(rounds)-1, budgetOutput) && finishErr == nil &&
			(err == nil || accountContinuationCanRepairInterruption(err))
		if canContinue {
			_, _ = assistant.WriteString(round.Text)
			currentBody, finalErr = BuildAccountContinuationBody(protocol, body, assistant.String(), budget.RemainingOutputTokens(budgetOutput))
			if finalErr == nil {
				continue
			}
		}
		if finalErr != nil && len(rounds) > 0 {
			// Once any round has produced a result, account failover would replay
			// already generated output and can bypass partial usage settlement.
			finalErr = fmt.Errorf("account continuation interrupted: %v", finalErr)
		}
		break
	}
	// Buffered rounds have not reached the client until their combined document
	// has been written. Streaming collectors already contain actual writes only.
	bytesBeforeComplete := max(0, originalWriter.Size())
	completeErr := ctx.Err()
	if completeErr == nil {
		completeErr = output.Complete(usage)
	}
	if stream && completeErr != nil && len(rounds) > 0 {
		last := len(rounds) - 1
		finalizeAccountContinuationResult(rounds[last], "", completeErr, ctx.Err() != nil || delivery[last].Frozen())
	}
	finalErr = errors.Join(finalErr, completeErr)
	bufferedDelivered := completeErr == nil && ctx.Err() == nil && output.base.Status() < 400 && originalWriter.Size() > bytesBeforeComplete
	for i, leaf := range rounds {
		if !stream {
			// Complete writes merged JSON through the final wrapper. Allocate
			// per-round evidence only after that one downstream write succeeds.
			perRound := NewDownstreamOutputTokenCollector(routeModel)
			perRound.bindCancellation(ctx.Done())
			if bufferedDelivered {
				perRound.appendAnswerText(roundTexts[i])
			}
			delivery[i] = perRound
			if !bufferedDelivered && finalErr != nil {
				invalidateAccountContinuationDelivery(leaf, finalErr, ctx.Err() != nil)
			}
		}
		applyAccountContinuationDelivery(leaf, delivery[i])
	}
	if len(rounds) == 0 {
		return nil, finalErr
	}
	result, aggregateErr := aggregate(rounds)
	if result != nil {
		setSummary(result, len(rounds)-1, time.Since(started))
		finalizeAccountContinuationResult(result, output.PublicResponseID(), finalErr, ctx.Err() != nil)
	}
	return result, errors.Join(finalErr, aggregateErr)
}

func applyAccountContinuationOutcome[T any](leaf *T, round AccountContinuationRound, err error, canceled bool) {
	var target **ResponseOutcome
	switch value := any(leaf).(type) {
	case *ForwardResult:
		target = &value.Outcome
	case *OpenAIForwardResult:
		target = &value.Outcome
	}
	if target == nil || *target != nil {
		return
	}
	collector := NewResponseOutcomeCollector(round.HTTPStatus, round.HTTPStatus)
	collector.outcome.HasText = strings.TrimSpace(round.Text) != ""
	collector.outcome.OutputBytes = int64(len(round.Text))
	if err != nil {
		collector.MarkStreamError(err, canceled)
	} else if round.Terminal && !round.Unsafe {
		collector.MarkCompleted(round.FinishReason)
	}
	snapshot := collector.Snapshot()
	*target = &snapshot
}

func invalidateAccountContinuationDelivery[T any](leaf *T, err error, canceled bool) {
	finalizeAccountContinuationResult(leaf, "", err, canceled)
	var outcome *ResponseOutcome
	switch value := any(leaf).(type) {
	case *ForwardResult:
		outcome = value.Outcome
	case *OpenAIForwardResult:
		outcome = value.Outcome
	}
	if outcome != nil {
		outcome.StreamCompleted = false
		outcome.HasText, outcome.HasReasoning, outcome.HasToolCall, outcome.HasMedia = false, false, false, false
		outcome.OutputBytes = 0
	}
}

func accountContinuationCanRepairInterruption(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var failover *UpstreamFailoverError
	if errors.As(err, &failover) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "missing terminal") || strings.Contains(message, "ended without") || strings.Contains(message, "unexpected eof")
}

func applyAccountContinuationDelivery[T any](leaf *T, collector *DownstreamOutputTokenCollector) {
	switch result := any(leaf).(type) {
	case *ForwardResult:
		ApplyDeliveredOutputTokens(result, collector)
	case *OpenAIForwardResult:
		ApplyDeliveredOpenAIOutputTokens(result, collector)
	}
}

func finalizeAccountContinuationResult[T any](result *T, publicID string, err error, canceled bool) {
	var outcome **ResponseOutcome
	switch value := any(result).(type) {
	case *ForwardResult:
		canceled = value.ClientDisconnect || canceled
		value.ClientDisconnect = canceled
		outcome = &value.Outcome
	case *OpenAIForwardResult:
		canceled = value.ClientDisconnect || canceled
		value.ClientDisconnect = canceled
		value.ContinuationPublicResponseID = publicID
		outcome = &value.Outcome
	}
	if err != nil && outcome != nil {
		collector := NewResponseOutcomeCollector(200, 200)
		if *outcome != nil {
			collector.outcome = **outcome
		}
		collector.outcome.StreamCompleted = false
		collector.MarkStreamError(err, canceled)
		snapshot := collector.Snapshot()
		*outcome = &snapshot
	}
}
