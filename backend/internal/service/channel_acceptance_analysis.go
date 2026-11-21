package service

import (
	"fmt"
	"sort"
)

// Cross-request findings compare actual wire inputs, never the original prompt
// alone: protocol adapters and injected instructions can change token counts.
func analyzeAcceptanceRun(run *AcceptanceRun) {
	if run == nil {
		return
	}
	// Get recomputes findings from stored sample evidence. Only runs created
	// under v3 use its matching requirements and fixed-offset rule.
	if run.RuleVersion == "channel-acceptance-v3" {
		analyzeAcceptanceRunV3(run)
		return
	}
	analyzeAcceptanceRunV2(run)
}

func acceptanceHasFinalRawInput(r *AcceptanceResult) bool {
	return r != nil && r.Completed && r.UpstreamUsageFinal && r.RawUsage.Input != nil && *r.RawUsage.Input >= 0
}

func analyzeAcceptanceRunV2(run *AcceptanceRun) {
	type target struct {
		account                    int64
		model, host, upstreamModel string
	}
	type group struct {
		inputs []*AcceptanceSample
		counts map[string]int64
	}
	groups := map[target]*group{}
	for i := range run.Samples {
		s := &run.Samples[i]
		r := s.Result
		if !acceptanceHasFinalRawInput(r) {
			continue
		}
		hash, _ := r.EffectiveParameters["input_fingerprint"].(string)
		if hash == "" || (s.Category != "input" && s.Category != "count") {
			continue
		}
		key := target{s.Snapshot.AccountID, s.Snapshot.Model, r.Source.Host, r.Source.UpstreamModel}
		g := groups[key]
		if g == nil {
			g = &group{counts: map[string]int64{}}
			groups[key] = g
		}
		if s.Category == "input" {
			g.inputs = append(g.inputs, s)
		} else if r.RawUsage.Source == "upstream_count_tokens" {
			// v2 used the last measured count for each wire input in run order.
			g.counts[hash] = *r.RawUsage.Input
		}
	}
	for _, g := range groups {
		analyzeAcceptanceInputs(g.inputs, g.counts)
	}
}

func analyzeAcceptanceRunV3(run *AcceptanceRun) {
	type target struct {
		account, groupID                           int64
		groupBound                                 bool
		model, platform, snapshot                  string
		host, upstreamModel, channel, auth, family string
	}
	groups := map[target][]*AcceptanceSample{}
	for i := range run.Samples {
		s := &run.Samples[i]
		if s.Category != "input" && s.Category != "count" {
			continue
		}
		r := s.Result
		if !acceptanceHasFinalRawInput(r) {
			continue
		}
		key := target{
			account: s.Snapshot.AccountID, model: s.Snapshot.Model, platform: s.Snapshot.Platform,
			snapshot: s.Snapshot.Fingerprint, host: r.Source.Host, upstreamModel: r.Source.UpstreamModel,
			channel: r.Source.Channel, auth: r.Source.AuthKind, family: acceptanceCountFamily(s),
		}
		if s.Snapshot.GroupID != nil {
			key.groupID, key.groupBound = *s.Snapshot.GroupID, true
		}
		groups[key] = append(groups[key], s)
	}
	for key, samples := range groups {
		type generation struct {
			protocol, path, responseModel, source string
			stream                                bool
		}
		inputs := map[generation][]*AcceptanceSample{}
		var counters []*AcceptanceSample
		for _, s := range samples {
			r := s.Result
			hash, _ := r.EffectiveParameters["input_fingerprint"].(string)
			if hash == "" {
				continue
			}
			switch s.Category {
			case "count":
				if key.family != "" {
					counters = append(counters, s)
				}
			case "input":
				g := generation{s.Protocol, r.Source.Path, r.Source.ResponseModel, r.RawUsage.Source, s.Stream}
				inputs[g] = append(inputs[g], s)
			}
		}
		for g, samples := range inputs {
			counts := map[string]int64{}
			for _, s := range counters {
				r := s.Result
				if model := r.Source.ResponseModel; model != "" && model != g.responseModel {
					continue
				}
				hash := r.EffectiveParameters["input_fingerprint"].(string)
				n := *r.RawUsage.Input
				if old, ok := counts[hash]; ok && old != n {
					// Valid counts are nonnegative. A disputed baseline stays
					// unusable regardless of the order of subsequent samples.
					n = -1
				}
				counts[hash] = n
			}
			sizes, bySize := analyzeAcceptanceInputs(samples, counts)
			if offset, ok := acceptanceFixedInputOffset(sizes, bySize, counts); ok {
				message := fmt.Sprintf("在 %d 档输入的重复请求中，生成接口报告的输入量减去同源计数接口的实测值均为 %+d Token，请复核上游计量口径", len(sizes), offset)
				for _, s := range samples {
					acceptanceAddFinding(s.Result, "fixed_input_offset", message)
				}
			}
		}
	}
}

// Pair the observed upstream route and raw measurement, not just the requested
// protocol or the category label. A declared relay can be tested against its
// own count endpoint without treating the relay as an official provider.
func acceptanceCountFamily(s *AcceptanceSample) string {
	r := s.Result
	if s.Snapshot.AccountID <= 0 || s.Snapshot.Model == "" || r.Source.Host == "" || r.Source.UpstreamModel == "" ||
		r.HTTPStatus < 200 || r.HTTPStatus >= 300 {
		return ""
	}
	for _, f := range r.Findings {
		if f.Code == "evidence_truncated" || f.Code == "count_returned_generation" {
			return ""
		}
	}
	if s.Category == "count" {
		if s.Protocol != "count_tokens" || r.Status != "passed" || r.RawUsage.Source != "upstream_count_tokens" ||
			r.RawUsage.Output != nil || r.HasText || r.HasReasoning || r.HasTool {
			return ""
		}
		switch r.Source.Path {
		case "/models/{model}:countTokens":
			return "gemini"
		case "/v1/messages/count_tokens":
			return "messages"
		case "/v1/responses/input_tokens":
			return "responses"
		}
	}
	if s.Category == "input" && (r.Status == "passed" || r.Status == "anomaly") {
		switch r.Source.Path {
		case "/models/{model}:generateContent", "/models/{model}:streamGenerateContent":
			if r.RawUsage.Source == "upstream_gemini_reported" {
				return "gemini"
			}
		case "/v1/messages":
			if r.RawUsage.Source == "upstream_anthropic_reported" {
				return "messages"
			}
		case "/v1/responses":
			if r.RawUsage.Source == "upstream_responses_reported" {
				return "responses"
			}
		}
	}
	return ""
}

func analyzeAcceptanceInputs(samples []*AcceptanceSample, counts map[string]int64) ([]int, map[int][]*AcceptanceSample) {
	repeats := map[string]int64{}
	bySize := map[int][]*AcceptanceSample{}
	for _, s := range samples {
		r := s.Result
		hash := r.EffectiveParameters["input_fingerprint"].(string)
		input := *r.RawUsage.Input
		if count, ok := counts[hash]; ok && count >= 0 && count != input {
			acceptanceAddFinding(r, "count_endpoint_mismatch", fmt.Sprintf("相同实际输入：生成接口报告 %d Token，计数接口报告 %d Token；上游口径不一致，需复核", input, count))
		}
		if previous, ok := repeats[hash]; ok && previous != input {
			acceptanceAddFinding(r, "repeated_input_usage_changed", fmt.Sprintf("相同实际输入的上游输入统计从 %d 变为 %d；需核对上游注入或统计规则", previous, input))
		}
		repeats[hash] = input
		bySize[s.InputSize] = append(bySize[s.InputSize], s)
	}
	sizes := make([]int, 0, len(bySize))
	for size := range bySize {
		sizes = append(sizes, size)
	}
	sort.Ints(sizes)
	for i := 1; i < len(sizes); i++ {
		small, big := bySize[sizes[i-1]], bySize[sizes[i]]
		smallHash := acceptanceRepeatedFingerprint(small)
		bigHash := acceptanceRepeatedFingerprint(big)
		if smallHash == "" || bigHash == "" || smallHash == bigHash {
			continue
		}
		// Require both repeats to show the same non-increasing pattern.
		minSmall := *small[0].Result.RawUsage.Input
		for _, s := range small {
			if n := *s.Result.RawUsage.Input; n < minSmall {
				minSmall = n
			}
		}
		maxBig := *big[0].Result.RawUsage.Input
		for _, s := range big {
			if n := *s.Result.RawUsage.Input; n > maxBig {
				maxBig = n
			}
		}
		if maxBig <= minSmall {
			for _, s := range big {
				acceptanceAddFinding(s.Result, "input_growth_not_reflected", "两轮测试均出现输入文本增加而上游输入 Token 不增；可能存在截断或统计口径差异，不能据此认证模型真假")
			}
		}
	}
	return sizes, bySize
}

// InputSize orders the fixtures, but only an independently measured count
// establishes each baseline. Check every repeat and every available size.
func acceptanceFixedInputOffset(sizes []int, bySize map[int][]*AcceptanceSample, counts map[string]int64) (int64, bool) {
	if len(sizes) < 3 || sizes[0] <= 0 {
		return 0, false
	}
	var offset int64
	previousCount := int64(-1)
	seen := map[string]bool{}
	for i, size := range sizes {
		bucket := bySize[size]
		hash := acceptanceRepeatedFingerprint(bucket)
		count, ok := counts[hash]
		if hash == "" || !ok || count <= previousCount {
			return 0, false
		}
		previousCount = count
		if i == 0 {
			offset = *bucket[0].Result.RawUsage.Input - count
			if offset == 0 {
				return 0, false
			}
		}
		for _, s := range bucket {
			if *s.Result.RawUsage.Input-count != offset || s.ID == "" || seen[s.ID] {
				return 0, false
			}
			seen[s.ID] = true
		}
	}
	return offset, true
}

// InputSize is a pre-dispatch estimate. Compare size buckets only when the
// actual wire requests repeat within each bucket and differ between buckets.
func acceptanceRepeatedFingerprint(samples []*AcceptanceSample) string {
	if len(samples) < 2 {
		return ""
	}
	hash, _ := samples[0].Result.EffectiveParameters["input_fingerprint"].(string)
	for _, sample := range samples[1:] {
		other, _ := sample.Result.EffectiveParameters["input_fingerprint"].(string)
		if other != hash {
			return ""
		}
	}
	return hash
}

func acceptanceAddFinding(r *AcceptanceResult, code, message string) {
	for _, f := range r.Findings {
		if f.Code == code {
			return
		}
	}
	r.Findings = append(r.Findings, AcceptanceFinding{Code: code, Level: "warning", Message: message})
	if r.Status == "passed" {
		r.Status = "anomaly"
	}
}
