package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type rawChatAntiTruncationPolicyContextKey struct{}

func WithRawChatAntiTruncationPolicy(ctx context.Context, policy domain.AntiTruncationPolicy) context.Context {
	if ctx == nil || !policy.Enabled {
		return ctx
	}
	return context.WithValue(ctx, rawChatAntiTruncationPolicyContextKey{}, policy)
}

func WithGroupAliasRawChatAntiTruncationPolicy(ctx context.Context, group *Group, requestedModel string) context.Context {
	if group == nil {
		return ctx
	}
	for _, alias := range group.ModelAliases {
		if strings.EqualFold(strings.TrimSpace(alias.Alias), strings.TrimSpace(requestedModel)) && alias.AntiTruncation.Enabled {
			return WithRawChatAntiTruncationPolicy(ctx, alias.AntiTruncation)
		}
	}
	return ctx
}

func RawChatAntiTruncationPolicyFromContext(ctx context.Context) (domain.AntiTruncationPolicy, bool) {
	if ctx == nil {
		return domain.AntiTruncationPolicy{}, false
	}
	policy, ok := ctx.Value(rawChatAntiTruncationPolicyContextKey{}).(domain.AntiTruncationPolicy)
	return policy, ok && policy.Enabled
}

func buildRawChatContinuationBody(body []byte, assistantText string, remainingOutputTokens int) ([]byte, error) {
	messages := gjson.GetBytes(body, "messages").Array()
	updated := body
	assistant := []byte(`{"role":"assistant","content":""}`)
	var err error
	assistant, err = sjson.SetBytes(assistant, "content", assistantText)
	if err != nil {
		return nil, err
	}
	updated, err = sjson.SetRawBytes(updated, "messages."+itoa(len(messages)), assistant)
	if err != nil {
		return nil, err
	}
	user := []byte(`{"role":"user","content":"Continue the response from where you left off. Do not repeat any earlier text."}`)
	updated, err = sjson.SetRawBytes(updated, "messages."+itoa(len(messages)+1), user)
	if err != nil {
		return nil, err
	}
	tokenPath := "max_tokens"
	if gjson.GetBytes(body, "max_completion_tokens").Exists() {
		tokenPath = "max_completion_tokens"
	}
	if remainingOutputTokens > 0 {
		updated, err = sjson.SetBytes(updated, tokenPath, remainingOutputTokens)
	}
	return updated, err
}

func clampRawChatOutputTokens(body []byte, maxOutputTokens int) ([]byte, error) {
	if maxOutputTokens <= 0 {
		return body, nil
	}
	path := "max_tokens"
	if gjson.GetBytes(body, "max_completion_tokens").Exists() {
		path = "max_completion_tokens"
	}
	current := gjson.GetBytes(body, path)
	if current.Exists() && current.Int() <= int64(maxOutputTokens) {
		return body, nil
	}
	return sjson.SetBytes(body, path, maxOutputTokens)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
