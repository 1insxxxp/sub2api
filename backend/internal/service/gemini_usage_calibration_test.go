//go:build unit

package service

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func calibrationTestClock() (func() time.Time, *time.Time) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	return func() time.Time { return now }, &now
}

func observeCalibrationSamples(c *geminiInputCalibrator, key string, count, local, measured int) {
	for i := 0; i < count; i++ {
		c.observe(key, local, measured)
	}
}

func assertCalibrationEstimate(t *testing.T, c *geminiInputCalibrator, key string, local, want int, active bool) {
	t.Helper()
	got, calibrated := c.estimate(key, local)
	if got != want || calibrated != active {
		t.Fatalf("estimate(%q, %d) = (%d, %t), want (%d, %t)", key, local, got, calibrated, want, active)
	}
}

func TestGeminiInputCalibrationRequiresFreshValidatedEvidence(t *testing.T) {
	clock, _ := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	observeCalibrationSamples(c, "account-model-shape", 15, 1024, 512)
	assertCalibrationEstimate(t, c, "account-model-shape", 2048, 2048, false)
	c.observe("account-model-shape", 1024, 512)
	assertCalibrationEstimate(t, c, "account-model-shape", 2048, 1024, true)
	assertCalibrationEstimate(t, c, "other-account-model-shape", 2048, 2048, false)
	assertCalibrationEstimate(t, c, "account-model-shape", 255, 255, false)
}

func TestGeminiInputCalibrationRejectsInvalidSamplesWithoutClamping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		key      string
		local    int
		measured int
	}{
		{"empty_key", "", 1024, 512},
		{"short_input", "k", 255, 128},
		{"zero_input", "k", 0, 512},
		{"negative_input", "k", -1024, 512},
		{"zero_measurement", "k", 1024, 0},
		{"negative_measurement", "k", 1024, -512},
		{"ratio_below_minimum", "k", 1024, 255},
		{"ratio_above_maximum", "k", 1024, 2049},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock, _ := calibrationTestClock()
			c := newGeminiInputCalibrator(clock)
			observeCalibrationSamples(c, tc.key, 32, tc.local, tc.measured)
			assertCalibrationEstimate(t, c, tc.key, 1024, 1024, false)
			if len(c.entries) != 0 {
				t.Fatal("invalid samples must not consume the bounded key budget")
			}
		})
	}
}

func TestGeminiInputCalibrationAcceptsInclusiveRatioBounds(t *testing.T) {
	for _, measured := range []int{256, 2048} {
		clock, _ := calibrationTestClock()
		c := newGeminiInputCalibrator(clock)
		observeCalibrationSamples(c, "k", 16, 1024, measured)
		assertCalibrationEstimate(t, c, "k", 1024, measured, true)
	}
}

func TestGeminiInputCalibrationUsesRecentHeldOutSamples(t *testing.T) {
	for _, tc := range []struct {
		name      string
		training  []int
		heldOut   []int
		want      int
		calibrate bool
	}{
		{"stable_improvement", []int{900, 900, 900, 900, 900, 900, 900, 900, 900, 900, 900, 900}, []int{940, 940, 940, 940}, 900, true},
		{"holdout_never_trains_the_factor", []int{500, 500, 500, 500, 500, 500, 600, 600, 600, 600, 600, 600}, []int{600, 600, 600, 600}, 550, true},
		{"twenty_percent_improvement_boundary", []int{980, 980, 980, 980, 980, 980, 980, 980, 980, 980, 980, 980}, []int{900, 900, 900, 900}, 980, true},
		{"fifteen_percent_maximum_boundary", []int{510, 510, 510, 510, 510, 510, 510, 510, 510, 510, 510, 510}, []int{600, 600, 600, 600}, 510, true},
		{"over_fifteen_percent_maximum", []int{509, 509, 509, 509, 509, 509, 509, 509, 509, 509, 509, 509}, []int{600, 600, 600, 600}, 1000, false},
		{"less_than_twenty_percent_improvement", []int{900, 900, 900, 900, 900, 900, 900, 900, 900, 900, 900, 900}, []int{949, 949, 949, 949}, 1000, false},
		{"recent_distribution_changed", []int{500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 500}, []int{900, 900, 900, 900}, 1000, false},
		{"one_bad_holdout_exceeds_maximum", []int{500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 500}, []int{500, 500, 500, 800}, 1000, false},
		{"training_spread_unstable", []int{500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 1000, 1000}, []int{500, 500, 500, 500}, 1000, false},
		{"raw_already_accurate", []int{1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}, []int{1000, 1000, 1000, 1000}, 1000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock, _ := calibrationTestClock()
			c := newGeminiInputCalibrator(clock)
			for _, measured := range append(tc.training, tc.heldOut...) {
				c.observe("k", 1000, measured)
			}
			assertCalibrationEstimate(t, c, "k", 1000, tc.want, tc.calibrate)
		})
	}
}

func TestGeminiInputCalibrationUsesMedianRatherThanMean(t *testing.T) {
	clock, _ := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	observeCalibrationSamples(c, "k", 11, 1000, 500)
	c.observe("k", 1000, 2000) // Within the allowed range, but not representative.
	observeCalibrationSamples(c, "k", 4, 1000, 500)
	assertCalibrationEstimate(t, c, "k", 1000, 500, true)
}

func TestGeminiInputCalibrationExpiresIndividualSamplesWithoutQueryRefresh(t *testing.T) {
	clock, now := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	observeCalibrationSamples(c, "k", 16, 1024, 512)
	*now = now.Add(59 * time.Minute)
	assertCalibrationEstimate(t, c, "k", 1024, 512, true)
	observeCalibrationSamples(c, "k", 8, 1024, 512)
	*now = now.Add(time.Minute)
	assertCalibrationEstimate(t, c, "k", 1024, 1024, false)
	observeCalibrationSamples(c, "k", 8, 1024, 512)
	assertCalibrationEstimate(t, c, "k", 1024, 512, true)
	*now = now.Add(time.Hour)
	assertCalibrationEstimate(t, c, "k", 1024, 1024, false)
	if len(c.entries) != 0 {
		t.Fatal("an expired key must be discarded")
	}
}

func TestGeminiInputCalibrationRingDiscardsOldestSamples(t *testing.T) {
	clock, _ := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	observeCalibrationSamples(c, "k", 32, 1024, 2048)
	observeCalibrationSamples(c, "k", 32, 1024, 512)
	assertCalibrationEstimate(t, c, "k", 1024, 512, true)
	if c.entries["k"].count != 32 {
		t.Fatal("the sample ring must be bounded at 32")
	}
}

func TestGeminiInputCalibrationBoundsKeysAndEvictsLeastRecentlyObserved(t *testing.T) {
	clock, now := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	for i := 0; i < 512; i++ {
		observeCalibrationSamples(c, fmt.Sprintf("key-%03d", i), 16, 1024, 512)
		*now = now.Add(time.Millisecond)
	}
	c.observe("key-000", 1024, 512)
	*now = now.Add(time.Millisecond)
	assertCalibrationEstimate(t, c, "key-001", 1024, 512, true) // Reading does not refresh recency.
	observeCalibrationSamples(c, "new-key", 16, 1024, 512)
	if len(c.entries) != 512 {
		t.Fatalf("key count = %d, want 512", len(c.entries))
	}
	assertCalibrationEstimate(t, c, "key-000", 1024, 512, true)
	assertCalibrationEstimate(t, c, "key-001", 1024, 1024, false)
	assertCalibrationEstimate(t, c, "new-key", 1024, 512, true)
}

func TestGeminiInputCalibrationRejectsOverflow(t *testing.T) {
	clock, _ := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	observeCalibrationSamples(c, "k", 16, 1024, 2048)
	maxInt := int(^uint(0) >> 1)
	assertCalibrationEstimate(t, c, "k", maxInt, maxInt, false)
}

func TestGeminiInputCalibrationConcurrentObserveAndEstimate(t *testing.T) {
	clock, _ := calibrationTestClock()
	c := newGeminiInputCalibrator(clock)
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 64; j++ {
				c.observe("k", 1024, 512)
				c.estimate("k", 1024)
			}
		}()
	}
	workers.Wait()
	assertCalibrationEstimate(t, c, "k", 1024, 512, true)
}
