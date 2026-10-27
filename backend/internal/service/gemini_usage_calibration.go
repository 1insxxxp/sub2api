package service

import (
	"math"
	"sort"
	"sync"
	"time"
)

const (
	geminiInputCalibrationMaxKeys    = 512
	geminiInputCalibrationMaxSamples = 32
	geminiInputCalibrationMinSamples = 16
	geminiInputCalibrationMinLocal   = 256
	geminiInputCalibrationTTL        = time.Hour
)

type geminiInputCalibrationSample struct {
	local, measured int
	observedAt      time.Time
}

type geminiInputCalibrationEntry struct {
	samples      [geminiInputCalibrationMaxSamples]geminiInputCalibrationSample
	start, count int
	lastObserved time.Time
}

type geminiInputCalibrator struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*geminiInputCalibrationEntry
}

func newGeminiInputCalibrator(now func() time.Time) *geminiInputCalibrator {
	if now == nil {
		now = time.Now
	}
	return &geminiInputCalibrator{now: now, entries: make(map[string]*geminiInputCalibrationEntry)}
}

var defaultGeminiInputCalibrator = newGeminiInputCalibrator(time.Now)

// The caller supplies an opaque account/origin/model/request-shape key and
// complete provider prompt measurements. Never train on an estimate or store
// request contents here. These process-local observations are not billing proof.
func geminiObserveInputCalibration(key string, local, measured int) {
	defaultGeminiInputCalibrator.observe(key, local, measured)
}

func geminiCalibratedInputEstimate(key string, local int) (int, bool) {
	return defaultGeminiInputCalibrator.estimate(key, local)
}

func (c *geminiInputCalibrator) observe(key string, local, measured int) {
	if c == nil || key == "" || local < geminiInputCalibrationMinLocal || measured <= 0 {
		return
	}
	ratio := float64(measured) / float64(local)
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0.25 || ratio > 2 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for existingKey, entry := range c.entries {
		if !now.Before(entry.lastObserved.Add(geminiInputCalibrationTTL)) {
			delete(c.entries, existingKey)
		}
	}
	entry := c.entries[key]
	if entry == nil {
		if len(c.entries) >= geminiInputCalibrationMaxKeys {
			var oldestKey string
			var oldest *geminiInputCalibrationEntry
			for candidateKey, candidate := range c.entries {
				if oldest == nil || candidate.lastObserved.Before(oldest.lastObserved) ||
					(candidate.lastObserved.Equal(oldest.lastObserved) && candidateKey < oldestKey) {
					oldestKey, oldest = candidateKey, candidate
				}
			}
			delete(c.entries, oldestKey)
		}
		entry = &geminiInputCalibrationEntry{}
		c.entries[key] = entry
	}
	entry.expire(now)
	index := (entry.start + entry.count) % geminiInputCalibrationMaxSamples
	entry.samples[index] = geminiInputCalibrationSample{local: local, measured: measured, observedAt: now}
	if entry.count == geminiInputCalibrationMaxSamples {
		entry.start = (entry.start + 1) % geminiInputCalibrationMaxSamples
	} else {
		entry.count++
	}
	entry.lastObserved = now
}

func (c *geminiInputCalibrator) estimate(key string, local int) (int, bool) {
	if c == nil || key == "" || local < geminiInputCalibrationMinLocal {
		return local, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		return local, false
	}
	entry.expire(c.now())
	if entry.count == 0 {
		delete(c.entries, key)
	}
	if entry.count < geminiInputCalibrationMinSamples {
		return local, false
	}

	// Fit only the older 75%; the newest samples must validate the factor
	// independently. Passing historical checks does not guarantee future accuracy.
	trainCount := entry.count * 3 / 4
	ratios := make([]float64, trainCount)
	for i := range ratios {
		sample := entry.samples[(entry.start+i)%geminiInputCalibrationMaxSamples]
		ratios[i] = float64(sample.measured) / float64(sample.local)
	}
	factor := geminiCalibrationMedian(ratios)
	spread := make([]float64, trainCount)
	for i, ratio := range ratios {
		spread[i] = math.Abs(ratio-factor) / factor
	}
	sort.Float64s(spread)
	if spread[int(math.Ceil(float64(trainCount)*0.9))-1] > 0.15 {
		return local, false
	}
	rawErrors := make([]float64, 0, entry.count-trainCount)
	calibratedErrors := make([]float64, 0, entry.count-trainCount)
	for i := trainCount; i < entry.count; i++ {
		sample := entry.samples[(entry.start+i)%geminiInputCalibrationMaxSamples]
		measured := float64(sample.measured)
		calibratedError := math.Abs(float64(sample.local)*factor-measured) / measured
		if calibratedError > 0.15 {
			return local, false
		}
		rawErrors = append(rawErrors, math.Abs(float64(sample.local)-measured)/measured)
		calibratedErrors = append(calibratedErrors, calibratedError)
	}
	rawMedian := geminiCalibrationMedian(rawErrors)
	if rawMedian <= 0 || geminiCalibrationMedian(calibratedErrors) > rawMedian*0.8 {
		return local, false
	}
	calibrated := math.Round(float64(local) * factor)
	if math.IsNaN(calibrated) || math.IsInf(calibrated, 0) || calibrated <= 0 || calibrated >= float64(int(^uint(0)>>1)) {
		return local, false
	}
	return int(calibrated), true
}

func (e *geminiInputCalibrationEntry) expire(now time.Time) {
	for e.count > 0 && !now.Before(e.samples[e.start].observedAt.Add(geminiInputCalibrationTTL)) {
		e.samples[e.start] = geminiInputCalibrationSample{}
		e.start = (e.start + 1) % geminiInputCalibrationMaxSamples
		e.count--
	}
}

func geminiCalibrationMedian(values []float64) float64 {
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}
