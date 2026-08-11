//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func availableChannelFloat64Ptr(value float64) *float64 { return &value }

func TestResolveAvailableChannelsPriceRangeUpdate(t *testing.T) {
	previous := &service.SystemSettings{
		AvailableChannelsPriceCNYMultiplier:    0.16,
		AvailableChannelsPriceCNYMultiplierMax: 0.20,
	}

	t.Run("old admin omission preserves both endpoints", func(t *testing.T) {
		minimum, maximum := resolveAvailableChannelsPriceRangeUpdate(UpdateSettingsRequest{}, previous)
		require.InDelta(t, 0.16, minimum, 1e-12)
		require.InDelta(t, 0.20, maximum, 1e-12)
	})

	t.Run("explicit update changes both endpoints", func(t *testing.T) {
		minimum, maximum := resolveAvailableChannelsPriceRangeUpdate(UpdateSettingsRequest{
			AvailableChannelsPriceCNYMultiplier:    availableChannelFloat64Ptr(0.17),
			AvailableChannelsPriceCNYMultiplierMax: availableChannelFloat64Ptr(0.22),
		}, previous)
		require.InDelta(t, 0.17, minimum, 1e-12)
		require.InDelta(t, 0.22, maximum, 1e-12)
	})

	t.Run("maximum below minimum is normalized for response and persistence", func(t *testing.T) {
		minimum, maximum := resolveAvailableChannelsPriceRangeUpdate(UpdateSettingsRequest{
			AvailableChannelsPriceCNYMultiplier:    availableChannelFloat64Ptr(0.20),
			AvailableChannelsPriceCNYMultiplierMax: availableChannelFloat64Ptr(0.16),
		}, previous)
		require.InDelta(t, 0.20, minimum, 1e-12)
		require.InDelta(t, 0.20, maximum, 1e-12)
	})
}

func TestResolveAvailableChannelsOfficialUSDToCNYRateUpdate(t *testing.T) {
	previous := &service.SystemSettings{AvailableChannelsOfficialUSDToCNYRate: 7.1}

	require.InDelta(t, 7.1, resolveAvailableChannelsOfficialUSDToCNYRateUpdate(UpdateSettingsRequest{}, previous), 1e-12)
	require.InDelta(t, 0, resolveAvailableChannelsOfficialUSDToCNYRateUpdate(UpdateSettingsRequest{
		AvailableChannelsOfficialUSDToCNYRate: availableChannelFloat64Ptr(0),
	}, previous), 1e-12)
	require.InDelta(t, service.AvailableChannelsOfficialUSDToCNYRateDefault, resolveAvailableChannelsOfficialUSDToCNYRateUpdate(UpdateSettingsRequest{
		AvailableChannelsOfficialUSDToCNYRate: availableChannelFloat64Ptr(-1),
	}, previous), 1e-12)
}

func TestDiffSettings_AuditsAvailableChannelPriceSettings(t *testing.T) {
	before := &service.SystemSettings{
		AvailableChannelsPriceCNYMultiplier:    0.16,
		AvailableChannelsPriceCNYMultiplierMax: 0.20,
		AvailableChannelsOfficialUSDToCNYRate:  7,
	}
	after := &service.SystemSettings{
		AvailableChannelsPriceCNYMultiplier:    0.17,
		AvailableChannelsPriceCNYMultiplierMax: 0.22,
		AvailableChannelsOfficialUSDToCNYRate:  7.1,
	}

	changed := diffSettings(before, after, nil, nil, UpdateSettingsRequest{})

	require.ElementsMatch(t, []string{
		service.SettingKeyAvailableChannelsPriceCNYMultiplier,
		service.SettingKeyAvailableChannelsPriceCNYMultiplierMax,
		service.SettingKeyAvailableChannelsOfficialUSDToCNYRate,
	}, changed)
}
