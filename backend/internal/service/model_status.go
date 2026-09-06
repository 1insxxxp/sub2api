package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	modelStatusCacheTTL       = 15 * time.Second
	modelStatusQueryTimeout   = 10 * time.Second
	ModelStatusRecentLimit    = 30
	ModelStatusBucketCount    = 20
	ModelStatusBucketInterval = 15 * time.Minute
	ModelStatusWindow         = ModelStatusBucketCount * ModelStatusBucketInterval
)

type ModelStatusHealth string

const (
	ModelStatusHealthy          ModelStatusHealth = "healthy"
	ModelStatusDegraded         ModelStatusHealth = "degraded"
	ModelStatusUnavailable      ModelStatusHealth = "unavailable"
	ModelStatusInsufficientData ModelStatusHealth = "insufficient_data"
	ModelStatusNoData           ModelStatusHealth = "no_data"
)

type ModelStatusMetrics struct {
	Total           int64    `json:"total"`
	Success         int64    `json:"success"`
	Failure         int64    `json:"failure"`
	Empty           int64    `json:"empty"`
	Unknown         int64    `json:"unknown"`
	SuccessRate     *float64 `json:"success_rate"`
	AvgTTFTMs       *float64 `json:"avg_ttft_ms"`
	AvgDurationMs   *float64 `json:"avg_duration_ms"`
	TTFTSamples     int64    `json:"ttft_samples"`
	DurationSamples int64    `json:"duration_samples"`
}

type ModelStatusRecent struct {
	At      time.Time          `json:"at"`
	Outcome UsageOutcomeStatus `json:"outcome"`
}

// ModelStatusBucket is one fixed 15-minute interval in the rolling report window.
type ModelStatusBucket struct {
	StartAt  time.Time           `json:"start_at"`
	EndAt    time.Time           `json:"end_at"`
	Total    int64               `json:"total"`
	Success  int64               `json:"success"`
	Failure  int64               `json:"failure"`
	Empty    int64               `json:"empty"`
	Unknown  int64               `json:"unknown"`
	Requests []ModelStatusRecent `json:"requests"`
}

type ModelStatusModel struct {
	Name     string              `json:"name"`
	Platform string              `json:"platform"`
	Status   ModelStatusHealth   `json:"status"`
	Metrics  ModelStatusMetrics  `json:"metrics"`
	Recent   []ModelStatusRecent `json:"recent"`
	Buckets  []ModelStatusBucket `json:"buckets"`
}

type ModelStatusGroup struct {
	ID       int64              `json:"id"`
	Name     string             `json:"name"`
	Platform string             `json:"platform"`
	Metrics  ModelStatusMetrics `json:"metrics"`
	Models   []ModelStatusModel `json:"models"`
}

type ModelStatusCoverage struct {
	Status                string   `json:"status"`
	TerminalErrorsEnabled bool     `json:"terminal_errors_enabled"`
	Reasons               []string `json:"reasons"`
}

type ModelStatusReport struct {
	GeneratedAt            time.Time           `json:"generated_at"`
	SnapshotAt             *time.Time          `json:"snapshot_at,omitempty"`
	RecentLimit            int                 `json:"recent_limit"`
	BucketCount            int                 `json:"bucket_count"`
	BucketIntervalMinutes  int                 `json:"bucket_interval_minutes"`
	RefreshIntervalSeconds int                 `json:"refresh_interval_seconds"`
	Coverage               ModelStatusCoverage `json:"coverage"`
	Summary                ModelStatusMetrics  `json:"summary"`
	Groups                 []ModelStatusGroup  `json:"groups"`
	windowEnd              time.Time           `json:"-"`
}

type ModelStatusScope struct {
	GroupID  int64  `json:"group_id"`
	Platform string `json:"platform"`
	Model    string `json:"model"`
}

type ModelStatusAggregate struct {
	GroupID  int64
	Platform string
	Model    string
	Metrics  ModelStatusMetrics
	Recent   []ModelStatusRecent
	Buckets  []ModelStatusBucket
}

type ModelStatusRepository interface {
	Aggregate(context.Context, time.Time, []ModelStatusScope) ([]ModelStatusAggregate, error)
}

type ModelStatusService struct {
	repo     ModelStatusRepository
	groups   PublicGroupRepository
	accounts AccountRepository
	ops      *OpsService
	mu       sync.Mutex
	cached   *ModelStatusReport
	cachedAt time.Time
	flight   singleflight.Group
}

func NewModelStatusService(repo ModelStatusRepository, groups PublicGroupRepository, accounts AccountRepository, ops *OpsService) *ModelStatusService {
	return &ModelStatusService{repo: repo, groups: groups, accounts: accounts, ops: ops}
}

func (s *ModelStatusService) Report(ctx context.Context) (*ModelStatusReport, error) {
	ctx, cancel := context.WithTimeout(ctx, modelStatusQueryTimeout)
	defer cancel()
	// Recheck group visibility even while metrics are cached: making a group
	// private must immediately remove both its cards and its summary counts.
	groups, err := s.groups.ListActivePublic(ctx)
	if err != nil {
		return nil, fmt.Errorf("load model status groups: %w", err)
	}
	if report := s.loadCached(); report != nil {
		return filterModelStatusReport(report, groups), nil
	}
	result := s.flight.DoChan("report", func() (any, error) {
		if report := s.loadCached(); report != nil {
			return report, nil
		}
		queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), modelStatusQueryTimeout)
		defer cancel()
		report, err := s.buildReport(queryCtx, groups)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cached, s.cachedAt = report, time.Now()
		s.mu.Unlock()
		return report, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return nil, result.Err
		}
		return filterModelStatusReport(result.Val.(*ModelStatusReport), groups), nil
	}
}

func (s *ModelStatusService) loadCached() *ModelStatusReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(s.cachedAt) < modelStatusCacheTTL {
		return s.cached
	}
	return nil
}

func (s *ModelStatusService) buildReport(ctx context.Context, groups []Group) (*ModelStatusReport, error) {
	end := time.Now().UTC()
	var snapshotAt *time.Time
	if os.Getenv("SERVER_MODE") == "debug" {
		if value := os.Getenv("MODEL_STATUS_SNAPSHOT_AT"); value != "" {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, fmt.Errorf("MODEL_STATUS_SNAPSHOT_AT must be an RFC3339 timestamp")
			}
			end = parsed.UTC()
			snapshotAt = &end
		}
	}
	report := &ModelStatusReport{
		SnapshotAt:  snapshotAt,
		RecentLimit: ModelStatusRecentLimit, BucketCount: ModelStatusBucketCount, BucketIntervalMinutes: int(ModelStatusBucketInterval / time.Minute), RefreshIntervalSeconds: 30,
		windowEnd: end,
		Groups:    []ModelStatusGroup{},
		Coverage:  ModelStatusCoverage{Status: "partial", Reasons: []string{"best_effort_recording"}},
	}
	if s.ops != nil {
		report.Coverage.TerminalErrorsEnabled = s.ops.IsMonitoringEnabled(ctx)
	}
	if !report.Coverage.TerminalErrorsEnabled {
		report.Coverage.Reasons = append(report.Coverage.Reasons, "terminal_errors_disabled")
	}
	groups = append([]Group(nil), groups...)
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	scopes := []ModelStatusScope{}
	for _, group := range groups {
		if group.IsExclusive || group.Status != StatusActive {
			continue
		}
		platforms := []string{group.Platform}
		if group.Platform == PlatformComposite {
			platforms = []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok}
		}
		accounts, err := s.accounts.ListModelAvailabilityCandidates(ctx, &group.ID, platforms, false)
		if err != nil {
			return nil, fmt.Errorf("load model status catalog: %w", err)
		}
		models := catalogModelsForGroup(group, accounts)
		if len(models) == 0 {
			continue
		}
		item := ModelStatusGroup{ID: group.ID, Name: group.Name, Platform: group.Platform, Models: make([]ModelStatusModel, 0, len(models))}
		for _, model := range models {
			item.Models = append(item.Models, ModelStatusModel{Name: model.Name, Platform: model.Platform, Status: ModelStatusNoData, Recent: []ModelStatusRecent{}, Buckets: emptyModelStatusBuckets(end)})
			scopes = append(scopes, ModelStatusScope{GroupID: group.ID, Platform: model.Platform, Model: model.Name})
		}
		report.Groups = append(report.Groups, item)
	}
	if len(scopes) > 0 {
		rows, err := s.repo.Aggregate(ctx, end, scopes)
		if err != nil {
			return nil, fmt.Errorf("aggregate model status: %w", err)
		}
		applyModelStatusAggregates(report, rows)
	}
	report.GeneratedAt = time.Now().UTC()
	return report, nil
}

func applyModelStatusAggregates(report *ModelStatusReport, rows []ModelStatusAggregate) {
	byScope := make(map[ModelStatusScope]ModelStatusAggregate, len(rows))
	for _, row := range rows {
		byScope[ModelStatusScope{row.GroupID, row.Platform, row.Model}] = row
	}
	for i := range report.Groups {
		group := &report.Groups[i]
		for j := range group.Models {
			model := &group.Models[j]
			row, ok := byScope[ModelStatusScope{group.ID, model.Platform, model.Name}]
			if !ok {
				continue
			}
			model.Metrics = finalizeModelStatusMetrics(row.Metrics)
			model.Status = modelStatusHealth(model.Metrics)
			if row.Recent != nil {
				model.Recent = row.Recent
			}
			model.Buckets = normalizeModelStatusBuckets(row.Buckets, report.windowEnd)
			group.Metrics = mergeModelStatusMetrics(group.Metrics, model.Metrics)
		}
		report.Summary = mergeModelStatusMetrics(report.Summary, group.Metrics)
	}
}

func emptyModelStatusBuckets(end time.Time) []ModelStatusBucket {
	// Keep the current partial bucket visible while retaining exactly 20 buckets.
	start := end.Truncate(ModelStatusBucketInterval).Add(-ModelStatusWindow + ModelStatusBucketInterval)
	buckets := make([]ModelStatusBucket, ModelStatusBucketCount)
	for i := range buckets {
		at := start.Add(time.Duration(i) * ModelStatusBucketInterval)
		buckets[i] = ModelStatusBucket{StartAt: at, EndAt: at.Add(ModelStatusBucketInterval), Requests: []ModelStatusRecent{}}
	}
	return buckets
}

func normalizeModelStatusBuckets(source []ModelStatusBucket, end time.Time) []ModelStatusBucket {
	if end.IsZero() {
		end = time.Now().UTC()
	}
	buckets := emptyModelStatusBuckets(end)
	byStart := make(map[int64]ModelStatusBucket, len(source))
	for _, bucket := range source {
		byStart[bucket.StartAt.UTC().UnixNano()] = bucket
	}
	for i := range buckets {
		if bucket, ok := byStart[buckets[i].StartAt.UnixNano()]; ok {
			if bucket.Requests == nil {
				bucket.Requests = []ModelStatusRecent{}
			}
			buckets[i] = bucket
		}
	}
	return buckets
}

func filterModelStatusReport(source *ModelStatusReport, groups []Group) *ModelStatusReport {
	allowed := make(map[int64]Group, len(groups))
	for _, group := range groups {
		if group.Status == StatusActive && !group.IsExclusive {
			allowed[group.ID] = group
		}
	}
	report := *source
	report.Groups, report.Summary = []ModelStatusGroup{}, ModelStatusMetrics{}
	for _, group := range source.Groups {
		current, ok := allowed[group.ID]
		if !ok || current.Platform != group.Platform {
			continue
		}
		group.Name = current.Name
		report.Groups = append(report.Groups, group)
		report.Summary = mergeModelStatusMetrics(report.Summary, group.Metrics)
	}
	return &report
}

func finalizeModelStatusMetrics(metrics ModelStatusMetrics) ModelStatusMetrics {
	metrics.SuccessRate = nil
	if known := metrics.Success + metrics.Failure + metrics.Empty; known > 0 {
		rate := float64(metrics.Success) * 100 / float64(known)
		metrics.SuccessRate = &rate
	}
	return metrics
}

func modelStatusHealth(metrics ModelStatusMetrics) ModelStatusHealth {
	if metrics.Total == 0 {
		return ModelStatusNoData
	}
	if metrics.Success+metrics.Failure+metrics.Empty < 5 || metrics.Unknown > 0 {
		return ModelStatusInsufficientData
	}
	if metrics.SuccessRate == nil {
		return ModelStatusInsufficientData
	}
	if *metrics.SuccessRate >= 99 {
		return ModelStatusHealthy
	}
	if *metrics.SuccessRate >= 90 {
		return ModelStatusDegraded
	}
	return ModelStatusUnavailable
}

func mergeModelStatusMetrics(a, b ModelStatusMetrics) ModelStatusMetrics {
	a.AvgTTFTMs = weightedModelStatusAverage(a.AvgTTFTMs, a.TTFTSamples, b.AvgTTFTMs, b.TTFTSamples)
	a.AvgDurationMs = weightedModelStatusAverage(a.AvgDurationMs, a.DurationSamples, b.AvgDurationMs, b.DurationSamples)
	a.Total += b.Total
	a.Success += b.Success
	a.Failure += b.Failure
	a.Empty += b.Empty
	a.Unknown += b.Unknown
	a.TTFTSamples += b.TTFTSamples
	a.DurationSamples += b.DurationSamples
	return finalizeModelStatusMetrics(a)
}

func weightedModelStatusAverage(a *float64, an int64, b *float64, bn int64) *float64 {
	var sum float64
	if a != nil {
		sum += *a * float64(an)
	} else {
		an = 0
	}
	if b != nil {
		sum += *b * float64(bn)
	} else {
		bn = 0
	}
	if an+bn == 0 {
		return nil
	}
	value := sum / float64(an+bn)
	return &value
}
