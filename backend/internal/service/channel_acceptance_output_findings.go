package service

import (
	"fmt"

	"github.com/tidwall/gjson"
)

// Parsers store ordinary output when a thought breakdown is available. Messages
// and unsplit Chat/Responses counters can instead contain the inclusive total.
func acceptanceOutputCounts(u AcceptanceUsage) (ordinary, inclusive *int64) {
	if u.Output == nil || *u.Output < 0 || u.Thoughts != nil && *u.Thoughts < 0 {
		return nil, nil
	}
	if u.OutputIncludesThoughts {
		if u.Thoughts == nil {
			return nil, u.Output
		}
		if *u.Thoughts > *u.Output {
			return nil, nil
		}
		n := *u.Output - *u.Thoughts
		return &n, u.Output
	}
	if u.Thoughts == nil {
		return u.Output, nil
	}
	total := acceptanceInputSum(u.Output, u.Thoughts)
	if total == nil {
		return nil, nil
	}
	return u.Output, total
}

// Gemini and Chat parsers retain cumulative maxima. A provider can correct a
// preliminary counter downwards. Use these snapshots for a discrepancy finding
// only if the retained counters agree with the last reported values.
func acceptanceTerminalOutputMatchesSnapshot(p *acceptanceEvidenceParser) bool {
	if len(p.frames) < 2 {
		return true
	}
	latestMatches := func(expected *int64, paths ...string) bool {
		for i := len(p.frames) - 1; i >= 0; i-- {
			for _, path := range paths {
				if v := gjson.GetBytes(p.frames[i], path); v.Exists() {
					return expected != nil && *expected == v.Int()
				}
			}
		}
		return expected == nil
	}
	switch p.usage.Source {
	case "upstream_gemini_reported":
		return latestMatches(p.usage.Output, "candidatesTokenCount") && latestMatches(p.usage.Thoughts, "thoughtsTokenCount")
	case "upstream_chat_reported":
		_, total := acceptanceOutputCounts(p.usage)
		return latestMatches(total, "output_tokens", "completion_tokens") &&
			latestMatches(p.usage.Thoughts, "output_tokens_details.reasoning_tokens", "completion_tokens_details.reasoning_tokens")
	default:
		return true
	}
}

// Keep the independent cache/input check. A partial output observation cannot
// establish a contradiction between final input, output and total tokens.
func acceptanceObservedUsageContradictory(raw *acceptanceEvidenceParser) bool {
	u := raw.usage
	if !raw.usageFinal || raw.truncated || !acceptanceTerminalOutputMatchesSnapshot(raw) {
		u.Output = nil
	}
	return acceptanceUsageContradictory(u)
}

func acceptanceCheckOutputUsage(result *AcceptanceResult, raw, client *acceptanceEvidenceParser) {
	// A terminal transport event does not promote counters seen only in a
	// preliminary frame. A dropped oversized frame may contain their correction.
	inconclusive := func() {
		if result.Status == "passed" {
			result.Status = "inconclusive"
		}
	}
	if !raw.completed || !raw.usageFinal || !client.completed || raw.truncated || client.truncated {
		inconclusive()
		return
	}
	if !acceptanceTerminalOutputMatchesSnapshot(raw) || client.usageFinal && !acceptanceTerminalOutputMatchesSnapshot(client) {
		inconclusive()
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "output_usage_snapshot_unconfirmed", Level: "warning", Message: "保留的终态用量帧无法确认累计输出或思考计数，需复核本次输出"})
		return
	}
	rawOrdinary, rawTotal := acceptanceOutputCounts(raw.usage)
	if (rawOrdinary != nil && *rawOrdinary == 0 || rawTotal != nil && *rawTotal == 0) && (raw.hasText || raw.hasTool) {
		result.Status = "anomaly"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "upstream_zero_output_with_content", Level: "warning", Message: "上游返回了正文或工具调用，同时在终态明确报告输出为零，需要复核计量"})
	}
	if !client.usageFinal && (client.usage.Output != nil || client.usage.Thoughts != nil) {
		inconclusive()
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "client_output_usage_not_final", Level: "warning", Message: "客户端已结束，但用量证据不完整；已观察到的计数仅供参考"})
		return
	}
	if client.usage.Output == nil {
		// A completed response can demonstrably omit an output counter even
		// though its missing counter prevents the parser from setting usageFinal.
		if rawOrdinary != nil && *rawOrdinary > 0 || rawTotal != nil && *rawTotal > 0 {
			result.Status = "anomaly"
			result.Findings = append(result.Findings, AcceptanceFinding{Code: "client_output_usage_lost", Level: "error", Message: "上游终态已报告输出用量，客户端完整响应缺少可用的输出计数"})
		} else {
			inconclusive()
			result.Findings = append(result.Findings, AcceptanceFinding{Code: "client_output_usage_missing", Level: "warning", Message: "客户端完整响应缺少可用的输出计数，无法核对输出用量"})
		}
		return
	}
	clientOrdinary, clientTotal := acceptanceOutputCounts(client.usage)
	const inclusive, ordinary = "输出总量（含思考）", "普通输出（不含思考）"
	rawLabel, clientLabel := inclusive, inclusive
	upstream, downstream := rawTotal, clientTotal
	switch {
	case rawTotal != nil && clientTotal != nil && (*rawTotal != *clientTotal || rawOrdinary == nil || clientOrdinary == nil):
		// A gross difference is comparable even without a reasoning breakdown.
	case rawOrdinary != nil && clientOrdinary != nil:
		// Equal totals can still conceal a changed ordinary/reasoning split.
		upstream, downstream, rawLabel, clientLabel = rawOrdinary, clientOrdinary, ordinary, ordinary
	case rawOrdinary != nil && clientTotal != nil && *rawOrdinary > *clientTotal:
		// Even the known ordinary component exceeds the other side's entire total.
		upstream, rawLabel = rawOrdinary, ordinary
	case clientOrdinary != nil && rawTotal != nil && *clientOrdinary > *rawTotal:
		downstream, clientLabel = clientOrdinary, ordinary
	default:
		// Valid final counters on different bases leave only the usage comparison
		// open. Preserve the result of the sample's own capability challenge.
		if (rawOrdinary == nil && rawTotal == nil) || (clientOrdinary == nil && clientTotal == nil) {
			inconclusive()
		}
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "output_usage_not_comparable", Level: "info", Message: "两端的输出与思考用量缺少可直接比较的计数，无法确认输出用量一致"})
		return
	}
	if *upstream == *downstream {
		return
	}
	code, level := "client_output_usage_mismatch", "warning"
	if *upstream > 0 && *downstream == 0 {
		code, level = "client_output_usage_lost", "error"
	}
	result.Status = "anomaly"
	result.Findings = append(result.Findings, AcceptanceFinding{Code: code, Level: level, Message: fmt.Sprintf("同一请求的终态用量存在差异。上游%s为 %d Token，客户端%s为 %d Token", rawLabel, *upstream, clientLabel, *downstream)})
}
