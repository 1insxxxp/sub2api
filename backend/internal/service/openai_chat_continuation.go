package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type rawChatAntiTruncationPolicyContextKey struct{}

func WithRawChatAntiTruncationPolicy(ctx context.Context, policy domain.AntiTruncationPolicy) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, rawChatAntiTruncationPolicyContextKey{}, policy)
}

func WithGroupAliasRawChatAntiTruncationPolicy(ctx context.Context, group *Group, requestedModel string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if group == nil {
		return WithRawChatAntiTruncationPolicy(ctx, domain.AntiTruncationPolicy{})
	}
	for _, alias := range group.ModelAliases {
		if strings.EqualFold(strings.TrimSpace(alias.Alias), strings.TrimSpace(requestedModel)) {
			return WithRawChatAntiTruncationPolicy(ctx, alias.AntiTruncation)
		}
	}
	return WithRawChatAntiTruncationPolicy(ctx, domain.AntiTruncationPolicy{})
}

func RawChatAntiTruncationPolicyFromContext(ctx context.Context) (domain.AntiTruncationPolicy, bool) {
	if ctx == nil {
		return domain.AntiTruncationPolicy{}, false
	}
	policy, ok := ctx.Value(rawChatAntiTruncationPolicyContextKey{}).(domain.AntiTruncationPolicy)
	return policy, ok && policy.Enabled
}

func withRawChatContinuationDeadline(parent context.Context, policy domain.AntiTruncationPolicy, start time.Time) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	maxDuration := time.Duration(policy.MaxDurationSeconds) * time.Second
	if maxDuration <= 0 {
		maxDuration = 180 * time.Second
	}
	return context.WithDeadline(parent, start.Add(maxDuration))
}

func rawChatContinuationAllowed(body []byte) bool {
	if gjson.GetBytes(body, "response_format").Exists() ||
		gjson.GetBytes(body, "guided_json").Exists() ||
		gjson.GetBytes(body, "guided_regex").Exists() ||
		gjson.GetBytes(body, "grammar").Exists() ||
		gjson.GetBytes(body, "json_schema").Exists() {
		return false
	}
	if n := gjson.GetBytes(body, "n"); n.Exists() && n.Int() > 1 {
		return false
	}
	for _, message := range gjson.GetBytes(body, "messages").Array() {
		if message.Get("reasoning_content").Exists() || message.Get("reasoning").Exists() {
			return false
		}
		for _, part := range message.Get("content").Array() {
			if strings.Contains(strings.ToLower(part.Get("type").String()), "reasoning") {
				return false
			}
		}
	}
	return true
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

func stripChatFinishReasonFromSSELine(line string) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	updated, err := sjson.SetRawBytes([]byte(payload), "choices.0.finish_reason", []byte("null"))
	if err != nil {
		return line
	}
	return "data: " + string(updated)
}

func aggregateCCStreamUsageSSELine(line string, usage OpenAIUsage) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	inputPath := "usage.prompt_tokens"
	if gjson.Get(payload, "usage.input_tokens").Exists() {
		inputPath = "usage.input_tokens"
	}
	outputPath := "usage.completion_tokens"
	if gjson.Get(payload, "usage.output_tokens").Exists() {
		outputPath = "usage.output_tokens"
	}
	updated, err := sjson.SetBytes([]byte(payload), inputPath, usage.InputTokens)
	if err != nil {
		return line
	}
	updated, err = sjson.SetBytes(updated, outputPath, usage.OutputTokens)
	if err != nil {
		return line
	}
	totalPath := "usage.total_tokens"
	updated, err = sjson.SetBytes(updated, totalPath, usage.InputTokens+usage.OutputTokens)
	if err != nil {
		return line
	}
	return "data: " + string(updated)
}

func buildCCStreamUsageSSELine(usage OpenAIUsage) string {
	return fmt.Sprintf("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}", usage.InputTokens, usage.OutputTokens, usage.InputTokens+usage.OutputTokens)
}

func hasCCStreamUsageLine(lines []string) bool {
	for _, line := range lines {
		if payload, ok := extractOpenAISSEDataLine(line); ok && gjson.Get(payload, "usage").Exists() {
			return true
		}
	}
	return false
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
