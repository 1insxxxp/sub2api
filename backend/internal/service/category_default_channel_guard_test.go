//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultAutoFollowChannelCacheRefreshesBeforeRouting(t *testing.T) {
	for _, state := range []string{"fresh", "changed-again", "read-error"} {
		t.Run(state, func(t *testing.T) {
			source := categoryDefaultChannelSource()
			svc, _ := categoryDefaultPublishSource(t, source, "openai")
			stale := source.Channel.Clone()
			stale.ModelMapping = map[string]map[string]string{PlatformOpenAI: {"public-model": "old-target"}}
			reads := 0
			repo := &mockChannelRepository{
				listAllFn: func(context.Context) ([]Channel, error) {
					reads++
					if state == "read-error" {
						return nil, errors.New("channel database unavailable")
					}
					fresh := source.Channel.Clone()
					if state == "changed-again" {
						fresh.ID++
					}
					return []Channel{*fresh}, nil
				},
				getGroupPlatformsFn: func(context.Context, []int64) (map[int64]string, error) {
					return map[int64]string{1: PlatformOpenAI}, nil
				},
			}
			channels := &ChannelService{repo: repo}
			channels.cache.Store(populateChannelCache([]Channel{*stale}, map[int64]string{1: PlatformOpenAI}))
			svc.SetChannelService(channels)
			_, err := svc.ResolveForUser(context.Background(), 100, &User{ID: 2, Status: StatusActive}, "public-model")
			if state == "fresh" {
				require.NoError(t, err)
				cached, loadErr := channels.GetChannelForGroup(context.Background(), 1)
				require.NoError(t, loadErr)
				require.Equal(t, "channel-target", cached.ModelMapping[PlatformOpenAI]["public-model"])
			} else if state == "changed-again" {
				require.ErrorIs(t, err, ErrCategoryDefaultPendingSync)
			} else {
				require.ErrorContains(t, err, "channel database unavailable")
			}
			require.Equal(t, 1, reads)
		})
	}
}
