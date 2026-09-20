//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelStatusVisibilityPreservesEmptyArrays(t *testing.T) {
	source := &ModelStatusReport{Groups: []ModelStatusGroup{{
		ID: 1, Platform: PlatformOpenAI,
		Models: []ModelStatusModel{{Name: "gpt-5.4", Recent: []ModelStatusRecent{}, Buckets: emptyModelStatusBuckets(time.Now())}},
	}}}
	report := filterModelStatusReport(source, []Group{{ID: 1, Platform: PlatformOpenAI, Status: StatusActive}})
	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"recent":[]`)
	require.Contains(t, string(encoded), `"requests":[]`)
}

type modelStatusGroupStub struct {
	GroupRepository
	groups       []Group
	publicGroups []Group
	err          error
	publicCalls  atomic.Int64
	onList       func(context.Context)
}

func (r *modelStatusGroupStub) ListActive(ctx context.Context) ([]Group, error) {
	if r.onList != nil {
		r.onList(ctx)
	}
	return r.groups, r.err
}

func (r *modelStatusGroupStub) ListActivePublic(ctx context.Context) ([]Group, error) {
	r.publicCalls.Add(1)
	if r.onList != nil {
		r.onList(ctx)
	}
	if r.publicGroups == nil {
		return r.groups, r.err
	}
	return r.publicGroups, nil
}

type modelStatusAccountStub struct {
	AccountRepository
	accounts map[int64][]Account
	err      error
}

func (r *modelStatusAccountStub) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, _ bool) ([]Account, error) {
	return r.accounts[*groupID], r.err
}

type modelStatusRepoStub struct {
	rows   []ModelStatusAggregate
	err    error
	calls  atomic.Int64
	end    time.Time
	scopes []ModelStatusScope
	block  <-chan struct{}
}

func (r *modelStatusRepoStub) Aggregate(_ context.Context, end time.Time, scopes []ModelStatusScope) ([]ModelStatusAggregate, error) {
	r.calls.Add(1)
	r.end, r.scopes = end, scopes
	if r.block != nil {
		<-r.block
	}
	return r.rows, r.err
}

func modelStatusFixtures() (*modelStatusGroupStub, *modelStatusAccountStub) {
	groups := &modelStatusGroupStub{groups: []Group{
		{ID: 1, Name: "Public A", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 2},
		{ID: 2, Name: "Public B", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: 3, Name: "Private", Platform: PlatformOpenAI, Status: StatusActive, IsExclusive: true},
		{ID: 4, Name: "Disabled", Platform: PlatformOpenAI, Status: "disabled"},
	}}
	future := time.Now().Add(time.Hour)
	account := Account{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true,
		RateLimitResetAt: &future, Credentials: map[string]any{"model_mapping": map[string]any{"shared": "upstream-secret"}}}
	return groups, &modelStatusAccountStub{accounts: map[int64][]Account{1: {account}, 2: {account}}}
}

func TestModelStatusHealthUsesKnownSuccessRate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		metrics ModelStatusMetrics
		status  ModelStatusHealth
		rate    *float64
	}{
		{"no traffic", ModelStatusMetrics{}, ModelStatusNoData, nil},
		{"one success", ModelStatusMetrics{Total: 1, Success: 1}, ModelStatusInsufficientData, modelStatusFloat(100)},
		{"empty lowers success", ModelStatusMetrics{Total: 100, Success: 1, Empty: 99}, ModelStatusUnavailable, modelStatusFloat(1)},
		{"unknown evidence does not override healthy rate", ModelStatusMetrics{Total: 6, Success: 5, Unknown: 1}, ModelStatusHealthy, modelStatusFloat(100)},
		{"unknown evidence does not override degraded rate", ModelStatusMetrics{Total: 7, Success: 4, Failure: 1, Unknown: 2}, ModelStatusDegraded, modelStatusFloat(80)},
		{"only unknown evidence is unknown", ModelStatusMetrics{Total: 6, Unknown: 6}, ModelStatusUnknown, nil},
		{"healthy", ModelStatusMetrics{Total: 100, Success: 99, Failure: 1}, ModelStatusHealthy, modelStatusFloat(99)},
		{"degraded", ModelStatusMetrics{Total: 10, Success: 9, Empty: 1}, ModelStatusDegraded, modelStatusFloat(90)},
		{"80 percent remains degraded", ModelStatusMetrics{Total: 10, Success: 8, Empty: 2}, ModelStatusDegraded, modelStatusFloat(80)},
		{"below 80 percent is unavailable", ModelStatusMetrics{Total: 100, Success: 79, Failure: 21}, ModelStatusUnavailable, modelStatusFloat(79)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics := finalizeModelStatusMetrics(tc.metrics)
			require.Equal(t, tc.rate, metrics.SuccessRate)
			require.Equal(t, tc.status, modelStatusHealth(metrics))
		})
	}
}

func TestModelStatusUsesDatabaseFilteredPublicGroups(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	groups.publicGroups = append([]Group(nil), groups.groups[:2]...)
	groups.err = errors.New("full active group query should not be used")
	repo := &modelStatusRepoStub{}

	report, err := NewModelStatusService(repo, groups, accounts, nil).Report(context.Background())

	require.NoError(t, err)
	require.Len(t, report.Groups, 2)
	require.Equal(t, int64(1), groups.publicCalls.Load())
}

func modelStatusFloat(v float64) *float64 { return &v }

func TestModelStatusReportScopesSummaryAndPreservesTemporarilyLimitedModels(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	repo := &modelStatusRepoStub{rows: []ModelStatusAggregate{
		{GroupID: 1, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 100, Success: 1, Empty: 99}},
		{GroupID: 2, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 5, Success: 5}},
		{GroupID: 3, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 999, Failure: 999}},
		{GroupID: 1, Platform: PlatformOpenAI, Model: "unlisted-secret", Metrics: ModelStatusMetrics{Total: 888, Failure: 888}},
	}}
	svc := NewModelStatusService(repo, groups, accounts, nil)
	report, err := svc.Report(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Groups, 2)
	require.Equal(t, int64(2), report.Groups[0].ID)
	require.Equal(t, int64(105), report.Summary.Total)
	require.InDelta(t, 100.0*6/105, *report.Summary.SuccessRate, 0.00001)
	require.Equal(t, ModelStatusHealthy, report.Groups[0].Models[0].Status)
	require.Equal(t, ModelStatusUnavailable, report.Groups[1].Models[0].Status)
	require.WithinDuration(t, time.Now(), repo.end, 2*time.Second)
	require.Equal(t, "partial", report.Coverage.Status)
	require.Contains(t, report.Coverage.Reasons, "terminal_errors_disabled")
	require.Contains(t, report.Coverage.Reasons, "best_effort_recording")
	data, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(data), `"recent_limit":30`)
	require.Contains(t, string(data), `"bucket_count":20`)
	require.Contains(t, string(data), `"bucket_interval_minutes":15`)
	for _, group := range report.Groups {
		for _, model := range group.Models {
			require.Len(t, model.Buckets, ModelStatusBucketCount)
			require.Equal(t, ModelStatusBucketInterval, model.Buckets[0].EndAt.Sub(model.Buckets[0].StartAt))
		}
	}
	require.NotContains(t, string(data), `"window_`)
	for _, secret := range []string{"Private", "Disabled", "upstream-secret", "unlisted-secret", "api_key", "request_id", "account_id"} {
		require.NotContains(t, string(data), secret)
	}
}

func TestFilterModelStatusReportUsesIndependentVisibilityAndRecomputesMetrics(t *testing.T) {
	image := ModelStatusModel{
		Name:     "gpt-image-1",
		Platform: PlatformOpenAI,
		Status:   ModelStatusHealthy,
		Metrics:  ModelStatusMetrics{Total: 10, Success: 9, Failure: 1},
		Recent:   []ModelStatusRecent{{Outcome: UsageOutcomeSuccess}},
		Buckets:  []ModelStatusBucket{{Total: 10, Success: 9, Failure: 1, Requests: []ModelStatusRecent{{Outcome: UsageOutcomeSuccess}}}},
	}
	text := ModelStatusModel{
		Name:     "gpt-5",
		Platform: PlatformOpenAI,
		Status:   ModelStatusUnavailable,
		Metrics:  ModelStatusMetrics{Total: 4, Failure: 4},
	}
	source := &ModelStatusReport{
		Summary: ModelStatusMetrics{Total: 999, Failure: 999},
		Groups: []ModelStatusGroup{{
			ID:       1,
			Name:     "GPT",
			Platform: PlatformOpenAI,
			Metrics:  ModelStatusMetrics{Total: 999, Failure: 999},
			Models:   []ModelStatusModel{image, text},
		}},
	}
	groups := []Group{{
		ID:                    1,
		Name:                  "GPT",
		Platform:              PlatformOpenAI,
		Status:                StatusActive,
		ModelAllowlist:        GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5"}},
		ModelStatusVisibility: GroupModelStatusVisibility{Enabled: true, Models: []string{"gpt-image-1"}},
	}}

	filtered := filterModelStatusReport(source, groups)

	require.Len(t, filtered.Groups, 1)
	require.Equal(t, []string{"gpt-image-1"}, []string{filtered.Groups[0].Models[0].Name})
	require.Equal(t, int64(10), filtered.Groups[0].Metrics.Total)
	require.Equal(t, int64(9), filtered.Groups[0].Metrics.Success)
	require.Equal(t, int64(10), filtered.Summary.Total)
	require.Equal(t, int64(9), filtered.Summary.Success)
	require.InDelta(t, 90, *filtered.Summary.SuccessRate, 0.0001)
	// Filtering must not mutate the cached full report or its nested slices.
	require.Len(t, source.Groups[0].Models, 2)
	require.Equal(t, int64(999), source.Summary.Total)
	require.Len(t, source.Groups[0].Models[0].Recent, 1)
	require.Len(t, source.Groups[0].Models[0].Buckets, 1)
	require.Len(t, source.Groups[0].Models[0].Buckets[0].Requests, 1)

	filtered.Groups[0].Models[0].Recent[0].Outcome = UsageOutcomeFailure
	filtered.Groups[0].Models[0].Buckets[0].Total = 1
	filtered.Groups[0].Models[0].Buckets[0].Requests[0].Outcome = UsageOutcomeFailure
	require.Equal(t, UsageOutcomeSuccess, source.Groups[0].Models[0].Recent[0].Outcome)
	require.Equal(t, int64(10), source.Groups[0].Models[0].Buckets[0].Total)
	require.Equal(t, UsageOutcomeSuccess, source.Groups[0].Models[0].Buckets[0].Requests[0].Outcome)
}

func TestFilterModelStatusReportDisabledOrEmptyVisibilityKeepsAllModels(t *testing.T) {
	source := &ModelStatusReport{Groups: []ModelStatusGroup{{
		ID: 1, Name: "GPT", Platform: PlatformOpenAI,
		Models: []ModelStatusModel{
			{Name: "gpt-image-1", Platform: PlatformOpenAI, Metrics: ModelStatusMetrics{Total: 2, Success: 2}},
			{Name: "gpt-5", Platform: PlatformOpenAI, Metrics: ModelStatusMetrics{Total: 3, Success: 3}},
		},
	}}}
	for _, visibility := range []GroupModelStatusVisibility{{}, {Enabled: true}} {
		filtered := filterModelStatusReport(source, []Group{{
			ID: 1, Name: "GPT", Platform: PlatformOpenAI, Status: StatusActive,
			ModelStatusVisibility: visibility,
		}})
		require.Len(t, filtered.Groups[0].Models, 2)
		require.Equal(t, int64(5), filtered.Summary.Total)
	}
}

func TestModelStatusCachedReportAppliesCurrentVisibilityConfiguration(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	groups.groups[0].ModelStatusVisibility = GroupModelStatusVisibility{Enabled: true, Models: []string{"shared"}}
	svc := NewModelStatusService(&modelStatusRepoStub{}, groups, accounts, nil)
	svc.cached = &ModelStatusReport{Groups: []ModelStatusGroup{{
		ID: 1, Name: "Public A", Platform: PlatformOpenAI,
		Models: []ModelStatusModel{
			{Name: "shared", Platform: PlatformOpenAI, Metrics: ModelStatusMetrics{Total: 2, Success: 2}},
			{Name: "gpt-image-1", Platform: PlatformOpenAI, Metrics: ModelStatusMetrics{Total: 8, Success: 8}},
		},
	}}}
	svc.cachedAt = time.Now()

	report, err := svc.Report(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Groups[0].Models, 1)
	require.Equal(t, "shared", report.Groups[0].Models[0].Name)

	groups.groups[0].ModelStatusVisibility = GroupModelStatusVisibility{}
	report, err = svc.Report(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Groups[0].Models, 2)
}

func TestModelStatusCacheCoalescesAndRechecksPublicVisibility(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	block := make(chan struct{})
	repo := &modelStatusRepoStub{block: block, rows: []ModelStatusAggregate{{GroupID: 1, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 5, Success: 5}}}}
	svc := NewModelStatusService(repo, groups, accounts, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.Report(context.Background()); errs <- err }()
	}
	require.Eventually(t, func() bool { return repo.calls.Load() == 1 }, time.Second, time.Millisecond)
	close(block)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), repo.calls.Load())
	groups.groups[0].IsExclusive = true
	report, err := svc.Report(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	require.Zero(t, report.Summary.Total)
	require.Equal(t, int64(1), repo.calls.Load())
}

func TestModelStatusErrorsDoNotFabricateEmptyHealthyReport(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	repo := &modelStatusRepoStub{err: errors.New("database unavailable")}
	svc := NewModelStatusService(repo, groups, accounts, nil)
	report, err := svc.Report(context.Background())
	require.Error(t, err)
	require.Nil(t, report)
	groups.err = errors.New("catalog unavailable")
	_, err = svc.Report(context.Background())
	require.ErrorContains(t, err, "catalog unavailable")
	require.False(t, strings.Contains(err.Error(), "healthy"))
}

func TestModelStatusSummaryWeightsLatencyBySamples(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	repo := &modelStatusRepoStub{rows: []ModelStatusAggregate{
		{GroupID: 1, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 10, Success: 10, AvgTTFTMs: modelStatusFloat(100), TTFTSamples: 10, AvgDurationMs: modelStatusFloat(1000), DurationSamples: 2}},
		{GroupID: 2, Platform: PlatformOpenAI, Model: "shared", Metrics: ModelStatusMetrics{Total: 5, Failure: 5, AvgTTFTMs: modelStatusFloat(400), TTFTSamples: 5, AvgDurationMs: modelStatusFloat(2000), DurationSamples: 4}},
	}}
	report, err := NewModelStatusService(repo, groups, accounts, nil).Report(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(15), report.Summary.TTFTSamples)
	require.InDelta(t, 200, *report.Summary.AvgTTFTMs, 0.0001)
	require.InDelta(t, 10000.0/6, *report.Summary.AvgDurationMs, 0.0001)
}

func TestModelStatusInitialCatalogReadHasBoundedDeadline(t *testing.T) {
	groups, accounts := modelStatusFixtures()
	var bounded bool
	groups.onList = func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		bounded = ok && time.Until(deadline) <= 10*time.Second
	}
	_, err := NewModelStatusService(&modelStatusRepoStub{}, groups, accounts, nil).Report(context.Background())
	require.NoError(t, err)
	require.True(t, bounded, "initial public catalog lookup must have a deadline")
}

func TestModelStatusDevelopmentSnapshotPreservesOriginalUpperBound(t *testing.T) {
	t.Setenv("SERVER_MODE", "debug")
	t.Setenv("MODEL_STATUS_SNAPSHOT_AT", "2026-09-05T22:18:30+08:00")
	groups, accounts := modelStatusFixtures()
	repo := &modelStatusRepoStub{}
	before := time.Now().UTC()
	report, err := NewModelStatusService(repo, groups, accounts, nil).Report(context.Background())
	require.NoError(t, err)
	want := time.Date(2026, 9, 5, 14, 18, 30, 0, time.UTC)
	require.Equal(t, want, repo.end)
	require.False(t, report.GeneratedAt.Before(before))
	data, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(data), `"snapshot_at":"2026-09-05T14:18:30Z"`)
}

func TestModelStatusSnapshotIsIgnoredOutsideDebug(t *testing.T) {
	for _, mode := range []string{"", "release", "test"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SERVER_MODE", mode)
			for _, value := range []string{"2026-09-05T14:18:30Z", "invalid-local-snapshot"} {
				t.Setenv("MODEL_STATUS_SNAPSHOT_AT", value)
				groups, accounts := modelStatusFixtures()
				before := time.Now().UTC()
				repo := &modelStatusRepoStub{}
				report, err := NewModelStatusService(repo, groups, accounts, nil).Report(context.Background())
				require.NoError(t, err)
				require.False(t, repo.end.Before(before))
				data, err := json.Marshal(report)
				require.NoError(t, err)
				require.NotContains(t, string(data), "snapshot_at")
			}
		})
	}
}

func TestModelStatusDebugSnapshotRejectsInvalidTime(t *testing.T) {
	t.Setenv("SERVER_MODE", "debug")
	t.Setenv("MODEL_STATUS_SNAPSHOT_AT", "2026-09-05 22:18:30")
	groups, accounts := modelStatusFixtures()
	report, err := NewModelStatusService(&modelStatusRepoStub{}, groups, accounts, nil).Report(context.Background())
	require.ErrorContains(t, err, "MODEL_STATUS_SNAPSHOT_AT")
	require.Nil(t, report)
}
