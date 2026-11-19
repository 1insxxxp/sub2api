package service

import "testing"

func TestAcceptanceAnalysisMatchesActualInputOnly(t *testing.T) {
	in, count := int64(100), int64(120)
	r := &AcceptanceRun{Samples: []AcceptanceSample{
		{Category: "input", Prompt: "p", Snapshot: AcceptanceSnapshot{AccountID: 1, Model: "m"}, Result: &AcceptanceResult{Status: "passed", Completed: true, UpstreamUsageFinal: true, RawUsage: AcceptanceUsage{Input: &in}, Source: AcceptanceSource{Host: "up.test", UpstreamModel: "m"}, EffectiveParameters: map[string]any{"input_fingerprint": "same"}}},
		{Category: "count", Prompt: "p", Snapshot: AcceptanceSnapshot{AccountID: 1, Model: "m"}, Result: &AcceptanceResult{Status: "passed", Completed: true, UpstreamUsageFinal: true, RawUsage: AcceptanceUsage{Input: &count}, Source: AcceptanceSource{Host: "up.test", UpstreamModel: "m"}, EffectiveParameters: map[string]any{"input_fingerprint": "different"}}},
	}}
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 0 {
		t.Fatal("different effective inputs compared")
	}
	r.Samples[1].Result.EffectiveParameters["input_fingerprint"] = "same"
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 0 {
		t.Fatal("unverified counter used as a measurement")
	}
	r.Samples[1].Result.RawUsage.Source = "upstream_count_tokens"
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 1 {
		t.Fatal("count mismatch missed")
	}
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 1 {
		t.Fatal("findings duplicated")
	}
}
func TestAcceptanceAnalysisInputGrowth(t *testing.T) {
	n := int64(100)
	r := &AcceptanceRun{}
	for i := 0; i < 6; i++ {
		r.Samples = append(r.Samples, AcceptanceSample{Category: "input", InputSize: 256 * (1 + i/2), Snapshot: AcceptanceSnapshot{AccountID: 1, Model: "m"}, Result: &AcceptanceResult{Status: "passed", Completed: true, UpstreamUsageFinal: true, Source: AcceptanceSource{Host: "up.test", UpstreamModel: "m"}, RawUsage: AcceptanceUsage{Input: &n}, EffectiveParameters: map[string]any{"input_fingerprint": string(rune('a' + i/2))}}})
	}
	analyzeAcceptanceRun(r)
	if r.Samples[4].Result.Status != "anomaly" {
		t.Fatal("constant reported tokens for growing input missed")
	}
}

func TestAcceptanceAnalysisRequiresFinalRawEvidence(t *testing.T) {
	samples := []AcceptanceSample{}
	for _, n := range []int64{100, 101} {
		r := &AcceptanceResult{Status: "passed", Completed: true, UpstreamUsageFinal: n == 100,
			Source:              AcceptanceSource{Host: "up.test", UpstreamModel: "m"},
			RawUsage:            AcceptanceUsage{Input: &n},
			EffectiveParameters: map[string]any{"input_fingerprint": "same"}}
		samples = append(samples, AcceptanceSample{Category: "input", Result: r})
	}
	run := &AcceptanceRun{Samples: samples}
	analyzeAcceptanceRun(run)
	if len(run.Samples[1].Result.Findings) != 0 {
		t.Fatal("partial upstream statistics compared as final")
	}
}

func TestAcceptanceAnalysisRequiresDistinctWireInputsForGrowth(t *testing.T) {
	run := &AcceptanceRun{}
	n := int64(100)
	for _, size := range []int{128, 128, 512, 512} {
		run.Samples = append(run.Samples, AcceptanceSample{Category: "input", InputSize: size, Result: &AcceptanceResult{
			Completed: true, UpstreamUsageFinal: true, RawUsage: AcceptanceUsage{Input: &n}, Source: AcceptanceSource{Host: "up.test", UpstreamModel: "m"},
			EffectiveParameters: map[string]any{"input_fingerprint": "identical"},
		}})
	}
	analyzeAcceptanceRun(run)
	if len(run.Samples[2].Result.Findings) != 0 {
		t.Fatal("identical wire requests classified as input growth")
	}
}
