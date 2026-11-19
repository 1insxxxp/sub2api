//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestModelStatusSnapshotCacheCoordinatesAcrossInstances(t *testing.T) {
	server := miniredis.RunT(t)
	first := redis.NewClient(&redis.Options{Addr: server.Addr()})
	second := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = first.Close(); _ = second.Close() })
	a, b := NewModelStatusSnapshotCache(first), NewModelStatusSnapshotCache(second)
	ctx := context.Background()
	lease := 45 * time.Second

	held, err := a.TryRefresh(ctx, "first", lease)
	require.NoError(t, err)
	require.True(t, held)
	held, err = b.TryRefresh(ctx, "second", lease)
	require.NoError(t, err)
	require.False(t, held)

	require.NoError(t, a.FinishRefresh(ctx, "first", nil, time.Minute))
	held, err = b.TryRefresh(ctx, "second", lease)
	require.NoError(t, err)
	require.False(t, held, "a failed refresh backs off on both instances")

	server.FastForward(time.Minute)
	held, err = b.TryRefresh(ctx, "second", lease)
	require.NoError(t, err)
	require.True(t, held)
	report := &service.ModelStatusReport{GeneratedAt: time.Now().UTC(), Groups: []service.ModelStatusGroup{{ID: 7}}}
	require.NoError(t, a.FinishRefresh(ctx, "first", report, time.Minute))
	got, err := a.Load(ctx)
	require.NoError(t, err)
	require.Nil(t, got, "an expired holder cannot write a snapshot")

	require.NoError(t, b.FinishRefresh(ctx, "second", report, 5*time.Minute))
	got, err = a.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, report, got)

	held, err = a.TryRefresh(ctx, "third", lease)
	require.NoError(t, err)
	require.False(t, held, "a freshly published snapshot suppresses refresh across replicas")
}
