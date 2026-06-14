package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type checkinSettingRepoStub struct {
	values map[string]string
}

func (s *checkinSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *checkinSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *checkinSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *checkinSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *checkinSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *checkinSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *checkinSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type checkinRepoStub struct {
	status     *CheckinStatus
	checkin    *CheckinResult
	statusErr  error
	checkinErr error
	input      *CheckinCreateInput
}

type checkinAuthCacheInvalidatorStub struct {
	invalidatedUserIDs []int64
	mu                 sync.Mutex
}

func (s *checkinAuthCacheInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string)    {}
func (s *checkinAuthCacheInvalidatorStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}
func (s *checkinAuthCacheInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
}

type checkinBillingCacheStub struct {
	invalidateCallCount atomic.Int64
	invalidatedUserIDs  []int64
	mu                  sync.Mutex
}

func (s *checkinBillingCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	return 0, nil
}
func (s *checkinBillingCacheStub) SetUserBalance(context.Context, int64, float64) error {
	return nil
}
func (s *checkinBillingCacheStub) DeductUserBalance(context.Context, int64, float64) error {
	return nil
}
func (s *checkinBillingCacheStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.invalidateCallCount.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
	return nil
}
func (s *checkinBillingCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return nil, nil
}
func (s *checkinBillingCacheStub) SetSubscriptionCache(context.Context, int64, int64, *SubscriptionCacheData) error {
	return nil
}
func (s *checkinBillingCacheStub) UpdateSubscriptionUsage(context.Context, int64, int64, float64) error {
	return nil
}
func (s *checkinBillingCacheStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	return nil
}
func (s *checkinBillingCacheStub) GetAPIKeyRateLimit(context.Context, int64) (*APIKeyRateLimitCacheData, error) {
	return nil, nil
}
func (s *checkinBillingCacheStub) SetAPIKeyRateLimit(context.Context, int64, *APIKeyRateLimitCacheData) error {
	return nil
}
func (s *checkinBillingCacheStub) UpdateAPIKeyRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
func (s *checkinBillingCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error {
	return nil
}
func (s *checkinBillingCacheStub) GetUserPlatformQuotaCache(context.Context, int64, string) (*UserPlatformQuotaCacheEntry, bool, error) {
	return nil, false, nil
}
func (s *checkinBillingCacheStub) SetUserPlatformQuotaCache(context.Context, int64, string, *UserPlatformQuotaCacheEntry, time.Duration) error {
	return nil
}
func (s *checkinBillingCacheStub) DeleteUserPlatformQuotaCache(context.Context, int64, string) error {
	return nil
}
func (s *checkinBillingCacheStub) IncrUserPlatformQuotaUsageCache(context.Context, int64, string, float64, time.Duration, bool) error {
	return nil
}
func (s *checkinBillingCacheStub) PopDirtyUserPlatformQuotaKeys(context.Context, int) ([]UserPlatformQuotaKey, error) {
	return nil, nil
}
func (s *checkinBillingCacheStub) ReaddDirtyUserPlatformQuotaKeys(context.Context, []UserPlatformQuotaKey) error {
	return nil
}
func (s *checkinBillingCacheStub) BatchGetUserPlatformQuotaCache(context.Context, []UserPlatformQuotaKey) ([]*UserPlatformQuotaCacheEntry, error) {
	return nil, nil
}

func (r *checkinRepoStub) GetStatus(ctx context.Context, userID int64, checkinDate string) (*CheckinStatus, error) {
	if r.statusErr != nil {
		return nil, r.statusErr
	}
	if r.status == nil {
		return &CheckinStatus{UserID: userID, CheckinDate: checkinDate}, nil
	}
	out := *r.status
	out.UserID = userID
	out.CheckinDate = checkinDate
	return &out, nil
}

func (r *checkinRepoStub) Checkin(ctx context.Context, input CheckinCreateInput) (*CheckinResult, error) {
	r.input = &input
	if r.checkinErr != nil {
		return nil, r.checkinErr
	}
	if r.checkin == nil {
		return &CheckinResult{CheckinStatus: CheckinStatus{
			UserID:              input.UserID,
			CheckinDate:         input.CheckinDate,
			CheckedInToday:      true,
			CurrentStreak:       1,
			LifetimeCheckinDays: 1,
			BaseRewardAmount:    input.BaseRewardAmount,
			TotalRewardAmount:   input.BaseRewardAmount,
			BalanceBefore:       10,
			BalanceAfter:        10 + input.BaseRewardAmount,
		}}, nil
	}
	out := *r.checkin
	return &out, nil
}

func TestCheckinServiceStatusDisabledWhenSettingMissing(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, &checkinSettingRepoStub{}, nil, nil)

	status, err := svc.GetStatus(context.Background(), 42)

	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.False(t, status.CheckedInToday)
	require.Nil(t, repo.input)
}

func TestCheckinServiceStatusTreatsLegacyConfigWithTiersAsEnabled(t *testing.T) {
	settings := &checkinSettingRepoStub{values: map[string]string{
		SettingKeyCheckinRewardConfig: `{"tiers":[{"amount":1,"probability":30,"sort_order":1}],"streak_enabled":true}`,
	}}
	repo := &checkinRepoStub{status: &CheckinStatus{
		CheckedInToday:      false,
		CurrentStreak:       6,
		LifetimeCheckinDays: 20,
	}}
	svc := NewCheckinService(repo, settings, nil, nil)

	status, err := svc.GetStatus(context.Background(), 42)

	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.False(t, status.CheckedInToday)
	require.Equal(t, 6, status.CurrentStreak)
	require.Equal(t, 20, status.LifetimeCheckinDays)
}

func TestCheckinServiceCheckinSelectsWeightedBaseReward(t *testing.T) {
	settings := &checkinSettingRepoStub{values: map[string]string{
		SettingKeyCheckinRewardConfig: `{"enabled":true,"tiers":[{"amount":1,"probability":30,"sort_order":1},{"amount":2,"probability":70,"sort_order":2}]}`,
	}}
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, settings, nil, nil)
	svc.SetRandomFloatForTest(func() float64 { return 0.95 })

	result, err := svc.Checkin(context.Background(), 42)

	require.NoError(t, err)
	require.True(t, result.CheckedInToday)
	require.NotNil(t, repo.input)
	require.Equal(t, 2.0, repo.input.BaseRewardAmount)
	require.Equal(t, 2.0, result.BaseRewardAmount)
	require.Equal(t, 12.0, result.BalanceAfter)
}

func TestCheckinServiceCheckinRejectsDisabledConfig(t *testing.T) {
	settings := &checkinSettingRepoStub{values: map[string]string{
		SettingKeyCheckinRewardConfig: `{"enabled":false,"tiers":[{"amount":1,"probability":100,"sort_order":1}]}`,
	}}
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, settings, nil, nil)

	_, err := svc.Checkin(context.Background(), 42)

	require.ErrorIs(t, err, ErrCheckinDisabled)
	require.Nil(t, repo.input)
}

func TestCheckinServiceCheckinInvalidatesBalanceAndAuthCaches(t *testing.T) {
	settings := &checkinSettingRepoStub{values: map[string]string{
		SettingKeyCheckinRewardConfig: `{"enabled":true,"tiers":[{"amount":1,"probability":100,"sort_order":1}]}`,
	}}
	repo := &checkinRepoStub{}
	authInvalidator := &checkinAuthCacheInvalidatorStub{}
	billingCache := &checkinBillingCacheStub{}
	svc := NewCheckinService(repo, settings, authInvalidator, billingCache)
	svc.SetRandomFloatForTest(func() float64 { return 0 })

	_, err := svc.Checkin(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, []int64{42}, authInvalidator.invalidatedUserIDs)
	require.Eventually(t, func() bool {
		return billingCache.invalidateCallCount.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, []int64{42}, billingCache.invalidatedUserIDs)
}
