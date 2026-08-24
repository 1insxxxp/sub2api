//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type settingPublicRepoStub struct {
	values map[string]string
	err    error
}

func (s *settingPublicRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingPublicRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingPublicRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingPublicRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingPublicRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingPublicRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingPublicRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetPublicSettings_ExposesRegistrationEmailSuffixWhitelist(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled:              "true",
			SettingKeyEmailVerifyEnabled:               "true",
			SettingKeyRegistrationEmailSuffixWhitelist: `["@EXAMPLE.com"," @foo.bar ","*.EDU.CN","@invalid_domain",""]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar", "*.edu.cn"}, settings.RegistrationEmailSuffixWhitelist)
}

func TestSettingService_GetPublicSettings_ExposesTablePreferences(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyTableDefaultPageSize: "50",
			SettingKeyTablePageSizeOptions: "[20,50,100]",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 50, settings.TableDefaultPageSize)
	require.Equal(t, []int{20, 50, 100}, settings.TablePageSizeOptions)
}

func TestSettingService_GetPublicSettings_ExposesLotteryEnabled(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyLotteryEnabled: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.LotteryEnabled)
}

func TestSettingService_GetPublicSettings_DefaultBranding(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Sub2API", settings.SiteName)
	require.Equal(t, "Subscription to API Conversion Platform", settings.SiteSubtitle)
}

func TestSettingService_GetPublicSettings_ExposesThemeLogos(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			"site_logo":       "/logo-default.svg",
			"site_logo_light": "/logo-light.svg",
			"site_logo_dark":  "/logo-dark.svg",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/logo-default.svg", settings.SiteLogo)
	require.Equal(t, "/logo-light.svg", settings.SiteLogoLight)
	require.Equal(t, "/logo-dark.svg", settings.SiteLogoDark)
}

func TestSettingService_GetPublicSettingsForInjection_DefersLargeDataURLLogos(t *testing.T) {
	largeLogo := "data:image/png;base64," + strings.Repeat("a", 20*1024)
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeySiteLogo:      largeLogo,
			SettingKeySiteLogoLight: "/logo-light.svg",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	payloadAny, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	payload, ok := payloadAny.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.Empty(t, payload.SiteLogo)
	require.Equal(t, "/logo-light.svg", payload.SiteLogoLight)
	require.True(t, payload.Partial)

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, largeLogo, settings.SiteLogo)
}

func TestSettingService_GetPublicSettings_ExposesCompactHomeEnabled(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyCompactHomeEnabled: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())

	require.NoError(t, err)
	require.True(t, settings.CompactHomeEnabled)

	missingSettings, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, missingSettings.CompactHomeEnabled)
}

func TestSettingService_ChannelMonitorHideThroughputDefaultsToPrivate(t *testing.T) {
	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, missing.HideThroughput)
	public, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ChannelMonitorHideThroughput)

	for _, value := range []string{"false", "0", "off", "disabled"} {
		runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorHideThroughput: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, runtime.HideThroughput, "value=%q", value)
	}
}

func TestSettingService_ChannelMonitorShowQuotaFailsClosed(t *testing.T) {
	// 缺省（迁移插入 'false' / 老库无行）一律不展示。
	missingRuntime := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.False(t, missingRuntime.ShowQuota)
	missingPublic, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, missingPublic.ChannelMonitorShowQuota)

	// 仅字面 "true" 视为开启；其余值（含异常值）fail-closed。
	runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorShowQuota: "true",
	}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, runtime.ShowQuota)

	for _, value := range []string{"false", "TRUE", "1", "yes", "on", "garbage"} {
		rt := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorShowQuota: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, rt.ShowQuota, "value=%q", value)
	}
}

func TestSettingService_GetPublicSettings_ExposesForceEmailOnThirdPartySignup(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyForceEmailOnThirdPartySignup: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ForceEmailOnThirdPartySignup)
}

func TestSettingService_GetPublicSettings_ExposesAllowUserViewErrorRequests(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAllowUserViewErrorRequests: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.AllowUserViewErrorRequests)
}

func TestSettingService_GetPublicSettings_ExposesAvailableChannelsPriceCNYMultiplier(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAvailableChannelsPriceCNYMultiplier:    "0.16",
			SettingKeyAvailableChannelsPriceCNYMultiplierMax: "0.20",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.InDelta(t, 0.16, settings.AvailableChannelsPriceCNYMultiplier, 1e-12)
	require.InDelta(t, 0.20, settings.AvailableChannelsPriceCNYMultiplierMax, 1e-12)

	payloadAny, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	payload, ok := payloadAny.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.InDelta(t, 0.16, payload.AvailableChannelsPriceCNYMultiplier, 1e-12)
	require.InDelta(t, 0.20, payload.AvailableChannelsPriceCNYMultiplierMax, 1e-12)
}

func TestSettingService_GetPublicSettings_NormalizesAvailableChannelsPriceCNYMultiplierRange(t *testing.T) {
	for _, raw := range []string{"NaN", "+Inf", "-Inf"} {
		t.Run("normalizes a non-finite minimum "+raw, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
				SettingKeyAvailableChannelsPriceCNYMultiplier: raw,
			}}, &config.Config{})

			settings, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Zero(t, settings.AvailableChannelsPriceCNYMultiplier)
			require.InDelta(t, 0.20, settings.AvailableChannelsPriceCNYMultiplierMax, 1e-12)
		})
	}

	t.Run("defaults a missing maximum to point two", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyAvailableChannelsPriceCNYMultiplier: "0.16",
		}}, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.InDelta(t, 0.20, settings.AvailableChannelsPriceCNYMultiplierMax, 1e-12)
	})

	t.Run("preserves an explicit zero when the minimum is zero", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyAvailableChannelsPriceCNYMultiplier:    "0",
			SettingKeyAvailableChannelsPriceCNYMultiplierMax: "0",
		}}, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.Zero(t, settings.AvailableChannelsPriceCNYMultiplierMax)
	})

	t.Run("raises a maximum below the minimum", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyAvailableChannelsPriceCNYMultiplier:    "0.20",
			SettingKeyAvailableChannelsPriceCNYMultiplierMax: "0.16",
		}}, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.InDelta(t, 0.20, settings.AvailableChannelsPriceCNYMultiplierMax, 1e-12)
	})

	for _, raw := range []string{"invalid", "NaN", "+Inf", "-1"} {
		t.Run("falls back to point two for invalid maximum "+raw, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
				SettingKeyAvailableChannelsPriceCNYMultiplier:    "0.16",
				SettingKeyAvailableChannelsPriceCNYMultiplierMax: raw,
			}}, &config.Config{})

			settings, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.InDelta(t, 0.20, settings.AvailableChannelsPriceCNYMultiplierMax, 1e-12)
		})
	}
}

func TestSettingService_GetPublicSettings_ExposesOfficialUSDToCNYRate(t *testing.T) {
	t.Run("defaults to seven when unset", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.InDelta(t, 7, settings.AvailableChannelsOfficialUSDToCNYRate, 1e-12)
	})

	t.Run("exposes configured rate to API and injection payloads", func(t *testing.T) {
		repo := &settingPublicRepoStub{
			values: map[string]string{
				SettingKeyAvailableChannelsOfficialUSDToCNYRate: "7.15",
			},
		}
		svc := NewSettingService(repo, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.InDelta(t, 7.15, settings.AvailableChannelsOfficialUSDToCNYRate, 1e-12)

		payloadAny, err := svc.GetPublicSettingsForInjection(context.Background())
		require.NoError(t, err)
		payload, ok := payloadAny.(*PublicSettingsInjectionPayload)
		require.True(t, ok)
		require.InDelta(t, 7.15, payload.AvailableChannelsOfficialUSDToCNYRate, 1e-12)
	})

	t.Run("keeps an explicit zero as the operator opt-out", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyAvailableChannelsOfficialUSDToCNYRate: "0",
		}}, &config.Config{})

		settings, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.Zero(t, settings.AvailableChannelsOfficialUSDToCNYRate)
	})

	for _, raw := range []string{"invalid", "NaN", "+Inf", "-1"} {
		t.Run("falls back to seven for invalid value "+raw, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
				SettingKeyAvailableChannelsOfficialUSDToCNYRate: raw,
			}}, &config.Config{})

			settings, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.InDelta(t, 7, settings.AvailableChannelsOfficialUSDToCNYRate, 1e-12)
		})
	}
}

func TestSettingService_GetPublicSettings_ExposesImageStudioEnabled(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyImageStudioConfig: `{"enabled":true,"allowed_models":["gpt-image-1"],"default_model":"gpt-image-1"}`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ImageStudioEnabled)
}

func TestSettingService_GetPublicSettings_ExposesWeChatOAuthModeCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectAppID:               "wx-mp-app",
			SettingKeyWeChatConnectAppSecret:           "wx-mp-secret",
			SettingKeyWeChatConnectMode:                "mp",
			SettingKeyWeChatConnectScopes:              "snsapi_base",
			SettingKeyWeChatConnectOpenEnabled:         "true",
			SettingKeyWeChatConnectMPEnabled:           "true",
			SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.True(t, settings.WeChatOAuthMPEnabled)
}

func TestSettingService_GetPublicSettings_DoesNotExposeMobileOnlyWeChatAsWebOAuthAvailable(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectMobileEnabled:       "true",
			SettingKeyWeChatConnectMode:                "mobile",
			SettingKeyWeChatConnectMobileAppID:         "wx-mobile-app",
			SettingKeyWeChatConnectMobileAppSecret:     "wx-mobile-secret",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.WeChatOAuthEnabled)
	require.False(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.True(t, settings.WeChatOAuthMobileEnabled)
}

func TestSettingService_GetPublicSettings_FallsBackToConfigForWeChatOAuthCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			OpenEnabled:         true,
			OpenAppID:           "wx-open-config",
			OpenAppSecret:       "wx-open-secret",
			FrontendRedirectURL: "/auth/wechat/config-callback",
		},
	})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.False(t, settings.WeChatOAuthMobileEnabled)
}
