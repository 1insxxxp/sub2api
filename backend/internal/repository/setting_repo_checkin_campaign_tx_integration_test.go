//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type blockingCheckinCampaignTxRepository struct {
	service.SettingRepository
	inner        service.CheckinCampaignConfigTransactionRepository
	writeEntered chan struct{}
	releaseWrite chan struct{}
	once         sync.Once
}

func (r *blockingCheckinCampaignTxRepository) WithCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(client *dbent.Client, repo service.SettingRepository) error,
) error {
	return r.inner.WithCheckinCampaignConfigTx(ctx, func(client *dbent.Client, repo service.SettingRepository) error {
		return fn(client, &blockingCheckinCampaignSettingRepository{
			SettingRepository: repo,
			parent:            r,
		})
	})
}

type blockingCheckinCampaignSettingRepository struct {
	service.SettingRepository
	parent *blockingCheckinCampaignTxRepository
}

func (r *blockingCheckinCampaignSettingRepository) SetMultiple(ctx context.Context, values map[string]string) error {
	if err := r.SettingRepository.SetMultiple(ctx, values); err != nil {
		return err
	}
	r.parent.once.Do(func() { close(r.parent.writeEntered) })
	select {
	case <-r.parent.releaseWrite:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCheckinCampaignConfigTransactionSerializesUpdateAndEnable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo := NewSettingRepository(integrationEntClient)
	txRepo, ok := repo.(service.CheckinCampaignConfigTransactionRepository)
	require.True(t, ok)
	checkinService := service.NewCheckinService(integrationEntClient, nil, nil)
	checkinService.SetSettingRepository(repo)

	settingKeys := []string{
		service.SettingKeyCheckinEnabled,
		service.SettingKeyCheckinMinTotalUsageUSD,
		service.SettingKeyCheckinMinTotalRechargeUSD,
		service.SettingKeyCheckinRewardConfig,
	}
	previous, err := repo.GetMultiple(ctx, settingKeys)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range settingKeys {
			if value, exists := previous[key]; exists {
				_ = repo.Set(cleanupCtx, key, value)
			} else {
				_ = repo.Delete(cleanupCtx, key)
			}
		}
	})

	highCap := service.CheckinConfig{
		Enabled:                true,
		Tiers:                  []service.CheckinRewardTier{{Amount: 1, Probability: 100}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         4,
		TotalRewardCap:         4,
	}
	_, err = checkinService.UpdateConfig(ctx, highCap)
	require.NoError(t, err)

	campaign, err := integrationEntClient.CheckinRewardCampaign.Create().
		SetName("concurrent-config-enable").
		SetStatus(domain.CheckinRewardCampaignStatusDraft).
		SetStartDate(time.Date(2199, 8, 10, 0, 0, 0, 0, time.UTC)).
		SetEndDate(time.Date(2199, 8, 12, 0, 0, 0, 0, time.UTC)).
		SetRewardTiers([]domain.CheckinRewardTier{{Amount: 3, Probability: 100}}).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = integrationEntClient.CheckinRewardCampaign.DeleteOneID(campaign.ID).Exec(cleanupCtx)
	})

	enableReady := make(chan struct{})
	allowEnableCommit := make(chan struct{})
	enableResult := make(chan error, 1)
	go func() {
		enableResult <- txRepo.WithCheckinCampaignConfigTx(ctx, func(client *dbent.Client, txSettings service.SettingRepository) error {
			values, loadErr := txSettings.GetMultiple(ctx, []string{service.SettingKeyCheckinRewardConfig})
			if loadErr != nil {
				return loadErr
			}
			var stored struct {
				TotalRewardCap float64 `json:"total_reward_cap"`
			}
			if decodeErr := json.Unmarshal([]byte(values[service.SettingKeyCheckinRewardConfig]), &stored); decodeErr != nil {
				return decodeErr
			}
			if stored.TotalRewardCap < 3 {
				return errors.New("campaign tiers exceed baseline cap")
			}
			if _, updateErr := client.CheckinRewardCampaign.UpdateOneID(campaign.ID).
				SetStatus(domain.CheckinRewardCampaignStatusEnabled).
				Save(ctx); updateErr != nil {
				return updateErr
			}
			close(enableReady)
			select {
			case <-allowEnableCommit:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	select {
	case <-enableReady:
	case err := <-enableResult:
		t.Fatalf("simulated enable returned before holding the transaction: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	lowCap := highCap
	lowCap.UsageRebateCap = 2
	lowCap.TotalRewardCap = 2
	updateResult := make(chan error, 1)
	go func() {
		_, updateErr := checkinService.UpdateConfig(ctx, lowCap)
		updateResult <- updateErr
	}()

	select {
	case earlyErr := <-updateResult:
		t.Fatalf("UpdateConfig escaped the shared transaction lock before enable committed: %v", earlyErr)
	case <-time.After(250 * time.Millisecond):
	}
	close(allowEnableCommit)
	require.NoError(t, <-enableResult)

	updateErr := <-updateResult
	require.Error(t, updateErr)
	require.Equal(t, "CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG", infraerrors.Reason(updateErr))

	storedCampaign, err := integrationEntClient.CheckinRewardCampaign.Get(ctx, campaign.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CheckinRewardCampaignStatusEnabled, storedCampaign.Status)
	storedConfig, err := checkinService.GetConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, 4.0, storedConfig.TotalRewardCap)
}

func TestCheckinCampaignUpdateThenEnableConcurrentReadsCommittedBaseline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	baseRepo := NewSettingRepository(integrationEntClient)
	txRepo, ok := baseRepo.(service.CheckinCampaignConfigTransactionRepository)
	require.True(t, ok)
	checkinService := service.NewCheckinService(integrationEntClient, nil, nil)

	settingKeys := []string{
		service.SettingKeyCheckinEnabled,
		service.SettingKeyCheckinMinTotalUsageUSD,
		service.SettingKeyCheckinMinTotalRechargeUSD,
		service.SettingKeyCheckinRewardConfig,
	}
	previous, err := baseRepo.GetMultiple(ctx, settingKeys)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range settingKeys {
			if value, exists := previous[key]; exists {
				_ = baseRepo.Set(cleanupCtx, key, value)
			} else {
				_ = baseRepo.Delete(cleanupCtx, key)
			}
		}
	})

	highCap := service.CheckinConfig{
		Enabled:                true,
		Tiers:                  []service.CheckinRewardTier{{Amount: 1, Probability: 100}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         4,
		TotalRewardCap:         4,
	}
	checkinService.SetSettingRepository(baseRepo)
	_, err = checkinService.UpdateConfig(ctx, highCap)
	require.NoError(t, err)

	campaign, err := integrationEntClient.CheckinRewardCampaign.Create().
		SetName("config-first-concurrent-enable").
		SetStatus(domain.CheckinRewardCampaignStatusDraft).
		SetStartDate(time.Date(2199, 9, 10, 0, 0, 0, 0, time.UTC)).
		SetEndDate(time.Date(2199, 9, 12, 0, 0, 0, 0, time.UTC)).
		SetRewardTiers([]domain.CheckinRewardTier{{Amount: 3, Probability: 100}}).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = integrationEntClient.CheckinRewardCampaign.DeleteOneID(campaign.ID).Exec(cleanupCtx)
	})

	blockingRepo := &blockingCheckinCampaignTxRepository{
		SettingRepository: baseRepo,
		inner:             txRepo,
		writeEntered:      make(chan struct{}),
		releaseWrite:      make(chan struct{}),
	}
	checkinService.SetSettingRepository(blockingRepo)
	lowCap := highCap
	lowCap.UsageRebateCap = 2
	lowCap.TotalRewardCap = 2
	updateResult := make(chan error, 1)
	go func() {
		_, updateErr := checkinService.UpdateConfig(ctx, lowCap)
		updateResult <- updateErr
	}()

	select {
	case <-blockingRepo.writeEntered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	enableResult := make(chan error, 1)
	go func() {
		_, enableErr := checkinService.EnableRewardCampaign(ctx, campaign.ID, 77)
		enableResult <- enableErr
	}()
	select {
	case earlyErr := <-enableResult:
		t.Fatalf("EnableRewardCampaign escaped the shared advisory lock: %v", earlyErr)
	case <-time.After(250 * time.Millisecond):
	}

	close(blockingRepo.releaseWrite)
	require.NoError(t, <-updateResult)
	enableErr := <-enableResult
	require.Equal(t, "CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG", infraerrors.Reason(enableErr))

	storedCampaign, err := integrationEntClient.CheckinRewardCampaign.Get(ctx, campaign.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CheckinRewardCampaignStatusDraft, storedCampaign.Status)
	storedConfig, err := checkinService.GetConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, 2.0, storedConfig.TotalRewardCap)
}
