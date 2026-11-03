package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// accountContinuationWSSink converts the shared protocol writer's SSE frames
// back into native WS messages without touching the upgraded HTTP connection.
type accountContinuationWSSink struct {
	gin.ResponseWriter
	send         func([]byte) error
	pending      []byte
	header       http.Header
	status, size int
}

func (w *accountContinuationWSSink) Header() http.Header               { return w.header }
func (w *accountContinuationWSSink) WriteHeader(code int)              { w.status = code }
func (w *accountContinuationWSSink) WriteHeaderNow()                   {}
func (w *accountContinuationWSSink) Status() int                       { return w.status }
func (w *accountContinuationWSSink) Size() int                         { return w.size }
func (w *accountContinuationWSSink) Written() bool                     { return w.size >= 0 }
func (w *accountContinuationWSSink) Flush()                            {}
func (w *accountContinuationWSSink) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *accountContinuationWSSink) Write(value []byte) (int, error) {
	w.pending = append(w.pending, value...)
	// The upstream WebSocket transport already enforces its frame limit.
	// Continuation's capture budget only disables additional replay; it must
	// not introduce a smaller limit on the valid event currently being sent.
	for {
		end := bytes.Index(w.pending, []byte("\n\n"))
		if end < 0 {
			break
		}
		frame := w.pending[:end]
		w.pending = w.pending[end+2:]
		for _, line := range bytes.Split(frame, []byte("\n")) {
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if bytes.Equal(payload, []byte("[DONE]")) || len(payload) == 0 {
				continue
			}
			if !gjson.ValidBytes(payload) {
				return 0, errors.New("invalid continuation websocket event")
			}
			if err := w.send(payload); err != nil {
				return 0, err
			}
		}
	}
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(value)
	return len(value), nil
}

func accountContinuationWSAllowed(ctx context.Context, account *Account, body []byte, model string) bool {
	policy, configured := ResolveAccountAntiTruncationPolicy(account, model, model, domain.AntiTruncationPolicy{})
	return configured && policy.Enabled && AccountContinuationRequestAllowed(AccountContinuationResponses, body)
}

func (s *OpenAIGatewayService) runAccountContinuationWSTurn(ctx context.Context, c *gin.Context, account *Account, body []byte, model string, send *func([]byte) error, relay func([]byte) (*OpenAIForwardResult, error)) (*OpenAIForwardResult, error) {
	if !accountContinuationWSAllowed(ctx, account, body, model) {
		return relay(body)
	}
	originalSend, originalWriter := *send, c.Writer
	sink := &accountContinuationWSSink{ResponseWriter: originalWriter, send: originalSend, header: make(http.Header), status: http.StatusOK, size: -1}
	c.Writer = sink
	*send = func(message []byte) error { _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", message); return err }
	defer func() { *send = originalSend; c.Writer = originalWriter }()
	ctx = WithGatewayPublicModel(ctx, model)
	result, err := runOpenAIWithAccountContinuation(ctx, c, account, body, AccountContinuationResponses, model, true, func(_ context.Context, round []byte) (*OpenAIForwardResult, error) { return relay(round) })
	if result != nil && result.AntiTruncationRounds > 0 {
		// The last actual response owns upstream context; the first public ID remains
		// what clients see throughout this logical turn.
		if bindErr := s.bindOpenAIContinuationAlias(ctx, c, account, result); bindErr != nil {
			err = errors.Join(err, bindErr)
		}
	}
	return result, err
}

// resolveWSContinuationAlias scopes durable aliases to the authenticated user.
// Existing WS continuation paths retain their established ownership semantics;
// an alias never widens those semantics to a different user or billing group.
func (s *OpenAIGatewayService) resolveWSContinuationAlias(ctx context.Context, c *gin.Context, body []byte) ([]byte, error) {
	public := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if public == "" {
		return body, nil
	}
	userID, _ := openAIContinuationOwner(c)
	actual, err := s.ResolveOpenAIContinuationAlias(ctx, getOpenAIGroupIDFromContext(c), userID, public)
	if err != nil {
		return nil, err
	}
	if actual == public {
		return body, nil
	}
	return setPreviousResponseIDToRawPayload(body, actual)
}
