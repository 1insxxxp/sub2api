package service

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	geminiFirstOutputSampleWindow   = 100
	geminiFirstOutputTrackerMaxKeys = 4096
	geminiFirstOutputTrackerTTL     = 30 * time.Minute
)

type geminiFirstOutputSample struct {
	ttft    time.Duration
	timeout bool
	at      time.Time
}

type geminiFirstOutputStats struct {
	SampleCount   int
	TimeoutCount  int
	AverageTTFT   time.Duration
	P95TTFT       time.Duration
	EffectiveTTFT time.Duration
	Reliability   float64
	LastSamples   []geminiFirstOutputSample
}

type geminiFirstOutputTracker struct {
	mu   sync.Mutex
	data map[string][]geminiFirstOutputSample
}

func newGeminiFirstOutputTracker() *geminiFirstOutputTracker {
	return &geminiFirstOutputTracker{data: make(map[string][]geminiFirstOutputSample)}
}

var defaultGeminiFirstOutputTracker = newGeminiFirstOutputTracker()

func geminiFirstOutputKey(accountID int64, model string) string {
	return fmtInt64(accountID) + "\x00" + strings.ToLower(strings.TrimSpace(model))
}

func (t *geminiFirstOutputTracker) recordAt(accountID int64, model string, ttft time.Duration, timeout bool, at time.Time) {
	if t == nil || accountID <= 0 || strings.TrimSpace(model) == "" || at.IsZero() {
		return
	}
	key := geminiFirstOutputKey(accountID, model)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.data == nil {
		t.data = make(map[string][]geminiFirstOutputSample)
	}
	if _, ok := t.data[key]; !ok && len(t.data) >= geminiFirstOutputTrackerMaxKeys {
		return
	}
	samples := append(t.data[key], geminiFirstOutputSample{ttft: ttft, timeout: timeout, at: at})
	if len(samples) > geminiFirstOutputSampleWindow {
		samples = samples[len(samples)-geminiFirstOutputSampleWindow:]
	}
	t.data[key] = samples
}

func (t *geminiFirstOutputTracker) snapshotAt(accountID int64, model string, now time.Time) (geminiFirstOutputStats, bool) {
	if t == nil {
		return geminiFirstOutputStats{}, false
	}
	key := geminiFirstOutputKey(accountID, model)
	t.mu.Lock()
	defer t.mu.Unlock()
	samples := t.data[key]
	if len(samples) == 0 {
		return geminiFirstOutputStats{}, false
	}
	valid := samples[:0]
	for _, sample := range samples {
		if now.Sub(sample.at) <= geminiFirstOutputTrackerTTL {
			valid = append(valid, sample)
		}
	}
	if len(valid) == 0 {
		delete(t.data, key)
		return geminiFirstOutputStats{}, false
	}
	t.data[key] = valid
	stats := geminiFirstOutputStats{SampleCount: len(valid), LastSamples: append([]geminiFirstOutputSample(nil), valid...)}
	values := make([]time.Duration, 0, len(valid))
	for _, sample := range valid {
		if sample.timeout {
			stats.TimeoutCount++
			continue
		}
		if sample.ttft > 0 {
			values = append(values, sample.ttft)
		}
	}
	if len(values) > 0 {
		var total time.Duration
		for _, value := range values {
			total += value
		}
		stats.AverageTTFT = total / time.Duration(len(values))
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		index := int(float64(len(values))*0.95) - 1
		if index < 0 {
			index = 0
		}
		if index >= len(values) {
			index = len(values) - 1
		}
		stats.P95TTFT = values[index]
		stats.EffectiveTTFT = stats.P95TTFT
		if safe := 2 * stats.AverageTTFT; safe > stats.EffectiveTTFT {
			stats.EffectiveTTFT = safe
		}
	}
	stats.Reliability = float64(stats.SampleCount-stats.TimeoutCount) / float64(stats.SampleCount)
	return stats, true
}

func (t *geminiFirstOutputTracker) shouldEscape(accountID int64, model string, switchTimeout time.Duration) bool {
	stats, ok := t.snapshotAt(accountID, model, time.Now())
	if !ok || stats.SampleCount < 2 {
		return false
	}
	if stats.TimeoutCount >= 2 && len(stats.LastSamples) >= 2 {
		n := len(stats.LastSamples)
		if stats.LastSamples[n-1].timeout && stats.LastSamples[n-2].timeout {
			return true
		}
	}
	return switchTimeout > 0 && stats.SampleCount >= 3 && stats.Reliability < 0.5
}

func (t *geminiFirstOutputTracker) prefer(candidateID, currentID int64, model string, switchTimeout time.Duration) bool {
	candidate, candidateOK := t.snapshotAt(candidateID, model, time.Now())
	current, currentOK := t.snapshotAt(currentID, model, time.Now())
	if !candidateOK || candidate.SampleCount < 3 {
		return false
	}
	if !currentOK || current.SampleCount < 3 {
		return true
	}
	if candidate.Reliability != current.Reliability {
		return candidate.Reliability > current.Reliability
	}
	if switchTimeout > 0 && candidate.EffectiveTTFT > switchTimeout && current.EffectiveTTFT <= switchTimeout {
		return false
	}
	return candidate.EffectiveTTFT < current.EffectiveTTFT
}

//nolint:unused // exercised by unit tests built with the unit tag.
func (t *geminiFirstOutputTracker) keyCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.data)
}

func recordGeminiFirstOutput(accountID int64, model string, ttft time.Duration, timeout bool) {
	defaultGeminiFirstOutputTracker.recordAt(accountID, model, ttft, timeout, time.Now())
}

// fmtInt64 avoids importing formatting into the hot path.
func fmtInt64(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if i == len(buf) {
		i--
		buf[i] = '0'
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
