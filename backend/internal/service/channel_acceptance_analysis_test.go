package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func acceptanceAnalysisSample(id, category, hash string, size int, tokens int64) AcceptanceSample {
	protocol, path, source := "gemini", "/models/{model}:generateContent", "upstream_gemini_reported"
	if category == "count" {
		protocol, path, source = "count_tokens", "/models/{model}:countTokens", "upstream_count_tokens"
	}
	return AcceptanceSample{
		ID: id, Category: category, Protocol: protocol, InputSize: size, Prompt: "same original prompt",
		Snapshot: AcceptanceSnapshot{AccountID: 1, Model: "alias", Platform: PlatformGemini, Fingerprint: "config"},
		Result: &AcceptanceResult{
			Status: "passed", HTTPStatus: 200, Completed: true, UpstreamUsageFinal: true,
			Source:              AcceptanceSource{Host: "up.test", UpstreamModel: "mapped", Path: path, Channel: "relay", AuthKind: "api_key"},
			RawUsage:            AcceptanceUsage{Source: source, Input: &tokens},
			EffectiveParameters: map[string]any{"input_fingerprint": hash},
		},
	}
}

func TestAcceptanceAnalysisMatchesActualInputOnly(t *testing.T) {
	r := &AcceptanceRun{RuleVersion: "channel-acceptance-v3", Samples: []AcceptanceSample{
		acceptanceAnalysisSample("generation", "input", "same", 256, 100),
		acceptanceAnalysisSample("counter", "count", "different", 256, 120),
	}}
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 0 {
		t.Fatal("different effective inputs compared")
	}
	r.Samples[1].Result.EffectiveParameters["input_fingerprint"] = "same"
	r.Samples[1].Result.RawUsage.Source = ""
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 0 {
		t.Fatal("unverified counter used as a measurement")
	}
	r.Samples[1].Result.RawUsage.Source = "upstream_count_tokens"
	analyzeAcceptanceRun(r)
	if !acceptanceHasFinding(r.Samples[0].Result, "count_endpoint_mismatch") {
		t.Fatal("count mismatch missed")
	}
	analyzeAcceptanceRun(r)
	if len(r.Samples[0].Result.Findings) != 1 {
		t.Fatal("findings duplicated")
	}
}
func TestAcceptanceAnalysisInputGrowth(t *testing.T) {
	n := int64(100)
	r := &AcceptanceRun{RuleVersion: "channel-acceptance-v3"}
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
	run := &AcceptanceRun{RuleVersion: "channel-acceptance-v3", Samples: samples}
	analyzeAcceptanceRun(run)
	if len(run.Samples[1].Result.Findings) != 0 {
		t.Fatal("partial upstream statistics compared as final")
	}
}

func TestAcceptanceAnalysisRequiresDistinctWireInputsForGrowth(t *testing.T) {
	run := &AcceptanceRun{RuleVersion: "channel-acceptance-v3"}
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

func acceptanceOffsetRun(offset int64) *AcceptanceRun {
	run := &AcceptanceRun{RuleVersion: "channel-acceptance-v3"}
	for i, size := range []int{256, 2048, 8192} {
		count := []int64{100, 410, 950}[i]
		hash := fmt.Sprintf("wire-%d", size)
		for repeat := 1; repeat <= 2; repeat++ {
			id := fmt.Sprintf("generation-%d-%d", size, repeat)
			run.Samples = append(run.Samples, acceptanceAnalysisSample(id, "input", hash, size, count+offset))
		}
		run.Samples = append(run.Samples, acceptanceAnalysisSample("count-"+hash, "count", hash, size, count))
	}
	return run
}

func TestAcceptanceAnalysisGetUsesPersistedRuleVersion(t *testing.T) {
	for _, version := range []string{"channel-acceptance-v2", "channel-acceptance-v3"} {
		t.Run(version, func(t *testing.T) {
			stored := acceptanceOffsetRun(17)
			stored.ID, stored.RuleVersion = "historical-run", version
			svc := NewChannelAcceptanceService(&acceptanceRepoStub{run: stored}, &acceptanceAccountsStub{}, nil, nil, nil)
			report, err := svc.Get(context.Background(), stored.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range report.Samples {
				isInput := s.Category == "input"
				if acceptanceHasFinding(s.Result, "count_endpoint_mismatch") != isInput ||
					acceptanceHasFinding(s.Result, "fixed_input_offset") != (isInput && version == "channel-acceptance-v3") {
					t.Fatalf("%s under %s: %+v", s.ID, version, s.Result.Findings)
				}
			}
		})
	}
}

func TestAcceptanceAnalysisV2RetainsLegacyFindings(t *testing.T) {
	run := &AcceptanceRun{RuleVersion: "channel-acceptance-v2", Samples: []AcceptanceSample{
		acceptanceAnalysisSample("small-1", "input", "small", 256, 10),
		acceptanceAnalysisSample("small-2", "input", "small", 256, 11),
		acceptanceAnalysisSample("large-1", "input", "large", 2048, 7),
		acceptanceAnalysisSample("large-2", "input", "large", 2048, 8),
		acceptanceAnalysisSample("count-1", "count", "small", 256, 10),
		acceptanceAnalysisSample("count-2", "count", "small", 256, 11),
	}}
	// These attributes were not grouping keys in v2. The last raw count in
	// persisted order applied even when historical route evidence was absent.
	run.Samples[1].Result.Source.ResponseModel = "another-revision"
	run.Samples[3].Stream = true
	run.Samples[5].Result.Source.Path = ""
	want := [][]string{
		{"count_endpoint_mismatch"},
		{"repeated_input_usage_changed"},
		{"input_growth_not_reflected"},
		{"repeated_input_usage_changed", "input_growth_not_reflected"},
		nil, nil,
	}
	analyzeAcceptanceRun(run)
	for i, s := range run.Samples {
		var codes []string
		for _, f := range s.Result.Findings {
			codes = append(codes, f.Code)
		}
		if !slices.Equal(codes, want[i]) || (s.Result.Status == "anomaly") != (len(want[i]) > 0) {
			t.Fatalf("%s: got %s %v, want findings %v", s.ID, s.Result.Status, codes, want[i])
		}
	}
}

func TestAcceptanceAnalysisFixedInputOffset(t *testing.T) {
	for _, tc := range []struct {
		protocol, generation, counter, source string
		offset                                int64
	}{
		{"gemini", "/models/{model}:generateContent", "/models/{model}:countTokens", "upstream_gemini_reported", 17},
		{"messages", "/v1/messages", "/v1/messages/count_tokens", "upstream_anthropic_reported", -17},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			run := acceptanceOffsetRun(tc.offset)
			for i := range run.Samples {
				s := &run.Samples[i]
				if tc.protocol == "messages" {
					s.Snapshot.Platform = PlatformAnthropic
					s.Result.Source.ResponseModel = "revision"
				}
				if s.Category == "count" {
					s.Result.Source.Path = tc.counter
				} else {
					s.Protocol, s.Result.Source.Path, s.Result.RawUsage.Source = tc.protocol, tc.generation, tc.source
					s.Result.Source.ResponseModel = "revision"
				}
			}
			slices.Reverse(run.Samples)
			analyzeAcceptanceRun(run)
			analyzeAcceptanceRun(run)
			for _, s := range run.Samples {
				if s.Category == "count" {
					if len(s.Result.Findings) != 0 || s.Result.Status != "passed" {
						t.Fatalf("count %s changed: %+v", s.ID, s.Result)
					}
					continue
				}
				if s.Result.Status != "anomaly" || !acceptanceHasFinding(s.Result, "count_endpoint_mismatch") {
					t.Fatalf("%s missing measurement mismatch: %+v", s.ID, s.Result.Findings)
				}
				found := 0
				for _, f := range s.Result.Findings {
					if f.Code == "fixed_input_offset" {
						found++
						if f.Level != "warning" || !strings.Contains(f.Message, fmt.Sprintf("%+d Token", tc.offset)) {
							t.Fatalf("%s has incorrect offset finding: %+v", s.ID, f)
						}
					}
				}
				if found != 1 {
					t.Fatalf("%s has %d fixed offsets", s.ID, found)
				}
			}
		})
	}
}

func TestAcceptanceAnalysisFixedInputOffsetRequiresThreeRepeatedMeasurements(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*AcceptanceRun)
	}{
		{"zero offset", func(r *AcceptanceRun) { *r = *acceptanceOffsetRun(0) }},
		{"two sizes", func(r *AcceptanceRun) { r.Samples = r.Samples[:6] }},
		{"missing repeat", func(r *AcceptanceRun) { r.Samples = slices.Delete(r.Samples, 7, 8) }},
		{"duplicate repeat", func(r *AcceptanceRun) { r.Samples[7].ID = r.Samples[6].ID }},
		{"unidentified repeat", func(r *AcceptanceRun) { r.Samples[7].ID = "" }},
		{"inconsistent repeat", func(r *AcceptanceRun) { *r.Samples[7].Result.RawUsage.Input += 1 }},
		{"inconsistent offset across sizes", func(r *AcceptanceRun) {
			*r.Samples[6].Result.RawUsage.Input += 1
			*r.Samples[7].Result.RawUsage.Input += 1
		}},
		{"same wire input across sizes", func(r *AcceptanceRun) {
			for i := 6; i < 9; i++ {
				r.Samples[i].Result.EffectiveParameters["input_fingerprint"] = "wire-2048"
				*r.Samples[i].Result.RawUsage.Input = *r.Samples[i-3].Result.RawUsage.Input
			}
		}},
		{"measured size does not grow", func(r *AcceptanceRun) {
			for i := 6; i < 9; i++ {
				*r.Samples[i].Result.RawUsage.Input = *r.Samples[i-3].Result.RawUsage.Input
			}
		}},
		{"different repeat wire input", func(r *AcceptanceRun) { r.Samples[7].Result.EffectiveParameters["input_fingerprint"] = "other" }},
		{"different repeat protocol", func(r *AcceptanceRun) { r.Samples[7].Protocol = "chat" }},
		{"different repeat revision", func(r *AcceptanceRun) { r.Samples[7].Result.Source.ResponseModel = "other" }},
		{"different repeat transport", func(r *AcceptanceRun) { r.Samples[7].Stream = true }},
		{"estimated or projected input only", func(r *AcceptanceRun) {
			s := r.Samples[7].Result
			s.ClientUsage, s.BillingUsage = s.RawUsage, s.RawUsage
			s.RawUsage.Input = nil
		}},
		{"local generation source", func(r *AcceptanceRun) { r.Samples[7].Result.RawUsage.Source = "local_estimate" }},
		{"partial generation", func(r *AcceptanceRun) { r.Samples[7].Result.UpstreamUsageFinal = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := acceptanceOffsetRun(17)
			tc.change(run)
			analyzeAcceptanceRun(run)
			for _, s := range run.Samples {
				if acceptanceHasFinding(s.Result, "fixed_input_offset") {
					t.Fatalf("insufficient evidence produced a fixed offset on %s", s.ID)
				}
			}
		})
	}
}

func TestAcceptanceAnalysisRequiresMatchedCountProvenance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*AcceptanceSample)
	}{
		{"account", func(s *AcceptanceSample) { s.Snapshot.AccountID++ }},
		{"group", func(s *AcceptanceSample) { s.Snapshot.GroupID = new(int64) }},
		{"public model", func(s *AcceptanceSample) { s.Snapshot.Model = "other" }},
		{"snapshot", func(s *AcceptanceSample) { s.Snapshot.Fingerprint = "other" }},
		{"upstream host", func(s *AcceptanceSample) { s.Result.Source.Host = "other.test" }},
		{"upstream model", func(s *AcceptanceSample) { s.Result.Source.UpstreamModel = "other" }},
		{"upstream revision", func(s *AcceptanceSample) { s.Result.Source.ResponseModel = "other" }},
		{"upstream auth", func(s *AcceptanceSample) { s.Result.Source.AuthKind = "bearer" }},
		{"wire input", func(s *AcceptanceSample) { s.Result.EffectiveParameters["input_fingerprint"] = "other" }},
		{"missing fingerprint", func(s *AcceptanceSample) { delete(s.Result.EffectiveParameters, "input_fingerprint") }},
		{"other family", func(s *AcceptanceSample) { s.Result.Source.Path = "/v1/messages/count_tokens" }},
		{"generation path", func(s *AcceptanceSample) { s.Result.Source.Path = "/models/{model}:generateContent" }},
		{"generation protocol", func(s *AcceptanceSample) { s.Protocol = "gemini" }},
		{"local count", func(s *AcceptanceSample) { s.Result.RawUsage.Source = "local_estimate" }},
		{"non-final count", func(s *AcceptanceSample) { s.Result.UpstreamUsageFinal = false }},
		{"unsuccessful HTTP", func(s *AcceptanceSample) { s.Result.HTTPStatus = 503 }},
		{"not completed", func(s *AcceptanceSample) { s.Result.Completed = false }},
		{"negative count", func(s *AcceptanceSample) { *s.Result.RawUsage.Input = -1 }},
		{"generated output", func(s *AcceptanceSample) { s.Result.RawUsage.Output = new(int64) }},
		{"generated tool", func(s *AcceptanceSample) { s.Result.HasTool = true }},
		{"truncated evidence", func(s *AcceptanceSample) {
			s.Result.Findings = []AcceptanceFinding{{Code: "evidence_truncated"}}
		}},
		{"rejected counter", func(s *AcceptanceSample) { s.Result.Status = "anomaly" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := acceptanceOffsetRun(17)
			tc.change(&run.Samples[8])
			analyzeAcceptanceRun(run)
			for _, s := range run.Samples {
				if acceptanceHasFinding(s.Result, "fixed_input_offset") ||
					(s.InputSize == 8192 && acceptanceHasFinding(s.Result, "count_endpoint_mismatch")) {
					t.Fatalf("unmatched count used on %s: %+v", s.ID, s.Result.Findings)
				}
			}
			if !acceptanceHasFinding(run.Samples[0].Result, "count_endpoint_mismatch") {
				t.Fatal("unrelated valid count lost")
			}
		})
	}
}

func TestAcceptanceAnalysisConflictingCountIsNeverABaseline(t *testing.T) {
	for _, duplicate := range []int64{950, 951} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("duplicate_%d_reverse_%t", duplicate, reverse), func(t *testing.T) {
				run := acceptanceOffsetRun(17)
				run.Samples = append(run.Samples,
					acceptanceAnalysisSample("extra-count", "count", "wire-8192", 8192, duplicate),
					acceptanceAnalysisSample("last-count", "count", "wire-8192", 8192, 950))
				if reverse {
					slices.Reverse(run.Samples)
				}
				analyzeAcceptanceRun(run)
				for _, s := range run.Samples {
					if s.Category != "input" {
						continue
					}
					wantOffset := duplicate == 950
					wantMismatch := wantOffset || s.InputSize != 8192
					if acceptanceHasFinding(s.Result, "fixed_input_offset") != wantOffset ||
						acceptanceHasFinding(s.Result, "count_endpoint_mismatch") != wantMismatch {
						t.Fatalf("%s unexpected findings: %+v", s.ID, s.Result.Findings)
					}
				}
			})
		}
	}
}
