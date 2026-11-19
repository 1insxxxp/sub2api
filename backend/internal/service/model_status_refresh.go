package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const (
	modelStatusFailureCooldown = time.Minute
	modelStatusPeerPollDelay   = 2 * time.Second
	modelStatusCacheTimeout    = time.Second
	modelStatusRefreshLease    = modelStatusQueryTimeout + 15*time.Second
)

var errModelStatusRefreshPending = errors.New("model status snapshot is being refreshed or waiting to retry")

// ModelStatusSnapshotCache shares finished reports and refresh ownership across
// gateway instances. FinishRefresh is conditional on the caller's lease token.
type ModelStatusSnapshotCache interface {
	Load(ctx context.Context) (*ModelStatusReport, error)
	TryRefresh(ctx context.Context, owner string, lease time.Duration) (bool, error)
	FinishRefresh(ctx context.Context, owner string, report *ModelStatusReport, cooldown time.Duration) error
}

func (s *ModelStatusService) modelStatusRefreshBackoff() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.retryAfter) {
		return s.refreshErr
	}
	return nil
}

func (s *ModelStatusService) delayModelStatusRefresh(err error, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshErr = err
	s.retryAfter = time.Now().Add(delay)
}

func (s *ModelStatusService) rememberReport(report *ModelStatusReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached == nil || !report.GeneratedAt.Before(s.cachedAt) {
		s.cached = report
		s.cachedAt = report.GeneratedAt
		s.refreshErr = nil
		s.retryAfter = time.Time{}
	}
}

func (s *ModelStatusService) refreshReport(ctx context.Context, groups []Group) (*ModelStatusReport, error) {
	if report := s.loadCached(); report != nil {
		return report, nil
	}
	if err := s.modelStatusRefreshBackoff(); err != nil {
		return nil, err
	}

	cache := s.snapshotCache
	owner := ""
	if cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, modelStatusCacheTimeout)
		shared, err := cache.Load(cacheCtx)
		cancel()
		if err != nil {
			err = fmt.Errorf("load shared model status snapshot: %w", err)
			s.delayModelStatusRefresh(err, modelStatusFailureCooldown)
			return nil, err
		}
		if shared != nil {
			s.rememberReport(shared)
			if report := s.loadCached(); report != nil {
				return report, nil
			}
		}

		owner = uuid.NewString()
		cacheCtx, cancel = context.WithTimeout(ctx, modelStatusCacheTimeout)
		held, err := cache.TryRefresh(cacheCtx, owner, modelStatusRefreshLease)
		cancel()
		if err != nil {
			err = fmt.Errorf("acquire model status refresh: %w", err)
			s.delayModelStatusRefresh(err, modelStatusFailureCooldown)
			return nil, err
		}
		if !held {
			s.delayModelStatusRefresh(errModelStatusRefreshPending, modelStatusPeerPollDelay)
			return nil, errModelStatusRefreshPending
		}
	}

	report, err := s.buildReport(ctx, groups)
	cooldown := modelStatusCacheTTL
	if err != nil {
		report = nil
		cooldown = modelStatusFailureCooldown
		s.delayModelStatusRefresh(err, cooldown)
	} else {
		s.rememberReport(report)
	}

	if cache != nil {
		finishCtx, cancel := context.WithTimeout(context.Background(), modelStatusCacheTimeout)
		defer cancel()
		if finishErr := cache.FinishRefresh(finishCtx, owner, report, cooldown); finishErr != nil {
			slog.Warn("publish model status refresh result", "error", finishErr)
		}
	}
	return report, err
}
