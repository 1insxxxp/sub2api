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
	type target struct {
		account                    int64
		model, host, upstreamModel string
	}
	groups := map[target][]*AcceptanceSample{}
	for i := range run.Samples {
		s := &run.Samples[i]
		r := s.Result
		if r == nil || !r.Completed || !r.UpstreamUsageFinal || r.RawUsage.Input == nil || *r.RawUsage.Input < 0 {
			continue
		}
		key := target{s.Snapshot.AccountID, s.Snapshot.Model, r.Source.Host, r.Source.UpstreamModel}
		groups[key] = append(groups[key], s)
	}
	for _, samples := range groups {
		counts := map[string]int64{}
		repeats := map[string]int64{}
		for _, s := range samples {
			hash, _ := s.Result.EffectiveParameters["input_fingerprint"].(string)
			if s.Category == "count" && s.Result.RawUsage.Source == "upstream_count_tokens" && hash != "" {
				counts[hash] = *s.Result.RawUsage.Input
			}
		}
		bySize := map[int][]*AcceptanceSample{}
		for _, s := range samples {
			if s.Category != "input" {
				continue
			}
			r := s.Result
			hash, _ := r.EffectiveParameters["input_fingerprint"].(string)
			if hash == "" {
				continue
			}
			input := *r.RawUsage.Input
			if count, ok := counts[hash]; ok && count != input {
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
	}
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
