//go:build unit

package service

import (
	"testing"
	"time"
)

func TestGeminiFirstOutputTrackerUsesRecentWindowAndP95LowerBound(t *testing.T) {
	tracker := newGeminiFirstOutputTracker()
	base := time.Unix(100, 0)
	for i := 1; i <= geminiFirstOutputSampleWindow+5; i++ {
		tracker.recordAt(7, "gemini-2.5-flash", 100*time.Millisecond+time.Duration(i)*time.Millisecond, false, base.Add(time.Duration(i)*time.Second))
	}
	stats, ok := tracker.snapshotAt(7, "gemini-2.5-flash", base.Add(10*time.Minute))
	if !ok {
		t.Fatal("expected stats")
	}
	if stats.SampleCount != geminiFirstOutputSampleWindow {
		t.Fatalf("sample count = %d, want %d", stats.SampleCount, geminiFirstOutputSampleWindow)
	}
	if stats.AverageTTFT != 155*time.Millisecond+500*time.Microsecond {
		t.Fatalf("average = %s, want 155.5ms", stats.AverageTTFT)
	}
	if stats.P95TTFT < 198*time.Millisecond {
		t.Fatalf("p95 = %s, want recent high samples", stats.P95TTFT)
	}
	if stats.EffectiveTTFT < stats.P95TTFT || stats.EffectiveTTFT < 2*stats.AverageTTFT {
		t.Fatalf("effective = %s, p95 = %s, average = %s", stats.EffectiveTTFT, stats.P95TTFT, stats.AverageTTFT)
	}
}

func TestGeminiFirstOutputTrackerReliabilityAndStickyEscape(t *testing.T) {
	tracker := newGeminiFirstOutputTracker()
	base := time.Now().Add(-time.Minute)
	for i := 0; i < 3; i++ {
		tracker.recordAt(1, "gemini-2.5-pro", 400*time.Millisecond, false, base.Add(time.Duration(i)*time.Second))
		tracker.recordAt(2, "gemini-2.5-pro", 0, true, base.Add(time.Duration(i)*time.Second))
	}
	fast, _ := tracker.snapshotAt(1, "gemini-2.5-pro", base.Add(time.Minute))
	slow, _ := tracker.snapshotAt(2, "gemini-2.5-pro", base.Add(time.Minute))
	if slow.TimeoutCount != 3 || slow.Reliability >= fast.Reliability {
		t.Fatalf("slow=%+v fast=%+v", slow, fast)
	}
	if !tracker.shouldEscape(2, "gemini-2.5-pro", 30*time.Second) {
		t.Fatal("expected repeated timeout account to escape")
	}
	if tracker.shouldEscape(1, "gemini-2.5-pro", 30*time.Second) {
		t.Fatal("did not expect healthy account to escape")
	}
	if !tracker.prefer(1, 2, "gemini-2.5-pro", 30*time.Second) {
		t.Fatal("expected healthy account to outrank timed-out account")
	}
}

func TestGeminiFirstOutputTrackerBoundsKeysAndExpires(t *testing.T) {
	tracker := newGeminiFirstOutputTracker()
	base := time.Now().Add(-time.Minute)
	for i := int64(0); i < geminiFirstOutputTrackerMaxKeys+10; i++ {
		tracker.recordAt(i+1, "gemini-2.5-flash", time.Second, false, base.Add(time.Duration(i)*time.Millisecond))
	}
	if got := tracker.keyCount(); got > geminiFirstOutputTrackerMaxKeys {
		t.Fatalf("key count = %d, want <= %d", got, geminiFirstOutputTrackerMaxKeys)
	}
	if _, ok := tracker.snapshotAt(1, "gemini-2.5-flash", base.Add(geminiFirstOutputTrackerTTL+time.Hour)); ok {
		t.Fatal("expired sample should not be returned")
	}
}
