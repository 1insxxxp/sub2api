package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type authIPAutoBlockSettingRepoStub struct {
	values map[string]string
}

func (s *authIPAutoBlockSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *authIPAutoBlockSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *authIPAutoBlockSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *authIPAutoBlockSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *authIPAutoBlockSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *authIPAutoBlockSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *authIPAutoBlockSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func newAuthIPAutoBlockGuardForTest(t *testing.T, values map[string]string) (*AuthIPAbuseGuard, *authIPAutoBlockSettingRepoStub) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	repo := &authIPAutoBlockSettingRepoStub{values: values}
	return NewAuthIPAbuseGuard(rdb, NewSettingService(repo, nil)), repo
}

func TestAuthIPAbuseGuardAutoBlocksRegistrationIPAtThreshold(t *testing.T) {
	guard, repo := newAuthIPAutoBlockGuardForTest(t, map[string]string{
		SettingKeyAuthIPBlacklistEnabled:               "false",
		SettingKeyAuthIPBlacklistRules:                 `[]`,
		SettingKeyAuthIPAutoBlockEnabled:               "true",
		SettingKeyAuthIPAutoBlockWindowMinutes:         "10",
		SettingKeyAuthIPAutoBlockRegisterThreshold:     "2",
		SettingKeyAuthIPAutoBlockVerifyCodeThreshold:   "20",
		SettingKeyAuthIPAutoBlockLoginFailureThreshold: "30",
	})

	first, err := guard.Record(context.Background(), AuthIPAbuseEventRegisterSuccess, "45.207.193.151")
	require.NoError(t, err)
	require.False(t, first.Blocked)

	second, err := guard.Record(context.Background(), AuthIPAbuseEventRegisterSuccess, "45.207.193.151")
	require.NoError(t, err)
	require.True(t, second.Blocked)
	require.Equal(t, "register_success", second.Event)
	require.Equal(t, int64(2), second.Count)
	require.Equal(t, int64(2), second.Threshold)
	require.Equal(t, "true", repo.values[SettingKeyAuthIPBlacklistEnabled])
	require.JSONEq(t, `["45.207.193.151"]`, repo.values[SettingKeyAuthIPBlacklistRules])
}

func TestAuthIPAbuseGuardDoesNothingWhenAutoBlockDisabled(t *testing.T) {
	guard, repo := newAuthIPAutoBlockGuardForTest(t, map[string]string{
		SettingKeyAuthIPBlacklistEnabled:               "false",
		SettingKeyAuthIPBlacklistRules:                 `[]`,
		SettingKeyAuthIPAutoBlockEnabled:               "false",
		SettingKeyAuthIPAutoBlockWindowMinutes:         "10",
		SettingKeyAuthIPAutoBlockRegisterThreshold:     "1",
		SettingKeyAuthIPAutoBlockVerifyCodeThreshold:   "1",
		SettingKeyAuthIPAutoBlockLoginFailureThreshold: "1",
	})

	decision, err := guard.Record(context.Background(), AuthIPAbuseEventRegisterSuccess, "45.207.193.151")
	require.NoError(t, err)
	require.False(t, decision.Blocked)
	require.Equal(t, "false", repo.values[SettingKeyAuthIPBlacklistEnabled])
	require.JSONEq(t, `[]`, repo.values[SettingKeyAuthIPBlacklistRules])
}
