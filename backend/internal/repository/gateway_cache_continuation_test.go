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

func TestGatewayCacheContinuationAliasIsolationAndExpiry(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { require.NoError(t, client.Close()) }()
	cache, ok := NewGatewayCache(client).(service.OpenAIContinuationAliasCache)
	require.True(t, ok)
	ctx := context.Background()
	binding := service.OpenAIContinuationAlias{ResponseID: "resp_final", AccountID: 13}
	require.NoError(t, cache.SetOpenAIContinuationAlias(ctx, 7, 11, "resp_public", binding, time.Minute))
	got, ok, err := cache.GetOpenAIContinuationAlias(ctx, 7, 11, "resp_public")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, binding, got)
	for _, scope := range [][2]int64{{8, 11}, {7, 12}} {
		_, ok, err = cache.GetOpenAIContinuationAlias(ctx, scope[0], scope[1], "resp_public")
		require.NoError(t, err)
		require.False(t, ok)
	}
	mr.FastForward(2 * time.Minute)
	_, ok, err = cache.GetOpenAIContinuationAlias(ctx, 7, 11, "resp_public")
	require.NoError(t, err)
	require.False(t, ok)
}
