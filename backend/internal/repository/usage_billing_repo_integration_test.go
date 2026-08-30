//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUsageBillingRepositoryApply_AllocatesGiftBalance(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	tests := []struct {
		name          string
		balance       float64
		giftBalance   float64
		cost          float64
		wantBalance   float64
		wantGift      float64
		wantExempt    float64
		wantOverdraft bool
	}{
		{name: "full gift", balance: 20, giftBalance: 20, cost: 12, wantBalance: 8, wantGift: 8, wantExempt: 12},
		{name: "mixed gift and ordinary", balance: 20, giftBalance: 10, cost: 12, wantBalance: 8, wantGift: 0, wantExempt: 10},
		{name: "no gift", balance: 20, giftBalance: 0, cost: 12, wantBalance: 8, wantGift: 0, wantExempt: 0},
		{name: "overdraft", balance: 5, giftBalance: 3, cost: 12, wantBalance: -7, wantGift: 0, wantExempt: 3, wantOverdraft: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := mustCreateUser(t, client, &service.User{
				Email:        fmt.Sprintf("usage-billing-gift-%s@example.com", uuid.NewString()),
				PasswordHash: "hash",
				Balance:      tt.balance,
			})
			_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = $1 WHERE id = $2", tt.giftBalance, user.ID)
			require.NoError(t, err)
			apiKey := mustCreateApiKey(t, client, &service.APIKey{
				UserID: user.ID,
				Key:    "sk-usage-billing-gift-" + uuid.NewString(),
				Name:   "billing-gift",
			})

			result, err := repo.Apply(ctx, &service.UsageBillingCommand{
				RequestID:   uuid.NewString(),
				APIKeyID:    apiKey.ID,
				UserID:      user.ID,
				BalanceCost: tt.cost,
			})
			require.NoError(t, err)
			require.True(t, result.Applied)
			require.Equal(t, tt.wantOverdraft, result.BalanceOverdrafted)
			require.NotNil(t, result.NewBalance)
			require.InDelta(t, tt.wantBalance, *result.NewBalance, 0.00000001)
			require.InDelta(t, tt.wantExempt, result.ThresholdExemptCost, 0.00000001)

			var balance, giftBalance float64
			require.NoError(t, integrationDB.QueryRowContext(ctx,
				"SELECT balance, gift_balance FROM users WHERE id = $1", user.ID,
			).Scan(&balance, &giftBalance))
			require.InDelta(t, tt.wantBalance, balance, 0.00000001)
			require.InDelta(t, tt.wantGift, giftBalance, 0.00000001)
			require.GreaterOrEqual(t, giftBalance, 0.0)
			if !tt.wantOverdraft {
				require.LessOrEqual(t, giftBalance, max(balance, 0.0))
			}
		})
	}
}

func TestUsageBillingRepositoryApply_GiftAllocationDeduplicates(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("usage-billing-gift-dedup-%s@example.com", uuid.NewString()), PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-gift-dedup-" + uuid.NewString(), Name: "gift-dedup"})
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), APIKeyID: apiKey.ID, UserID: user.ID, BalanceCost: 12}

	first, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	require.InDelta(t, 10, first.ThresholdExemptCost, 0.00000001)
	duplicate, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
	require.Zero(t, duplicate.ThresholdExemptCost)

	var balance, giftBalance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, gift_balance FROM users WHERE id = $1", user.ID).Scan(&balance, &giftBalance))
	require.InDelta(t, 8, balance, 0.00000001)
	require.Zero(t, giftBalance)
}

func TestUsageBillingRepositoryApply_SubscriptionDoesNotConsumeGift(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("usage-billing-gift-sub-%s@example.com", uuid.NewString()), PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	group := mustCreateGroup(t, client, &service.Group{Name: "usage-billing-gift-sub-" + uuid.NewString(), Platform: service.PlatformAnthropic, SubscriptionType: service.SubscriptionTypeSubscription})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-gift-sub-" + uuid.NewString(), Name: "gift-sub"})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{UserID: user.ID, GroupID: group.ID})

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID: uuid.NewString(), APIKeyID: apiKey.ID, UserID: user.ID,
		SubscriptionID: &subscription.ID, SubscriptionCost: 12,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.ThresholdExemptCost)

	var balance, giftBalance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, gift_balance FROM users WHERE id = $1", user.ID).Scan(&balance, &giftBalance))
	require.InDelta(t, 20, balance, 0.00000001)
	require.InDelta(t, 10, giftBalance, 0.00000001)
}

func TestUsageBillingRepositoryApply_MissingUserDoesNotAllocateGift(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	owner := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("usage-billing-missing-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: owner.ID, Key: "sk-gift-missing-" + uuid.NewString(), Name: "gift-missing"})
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID: uuid.NewString(), APIKeyID: apiKey.ID, UserID: 9_000_000_000_000, BalanceCost: 1,
	})
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

func TestUsageBillingRepositoryApply_ConcurrentGiftAllocationCannotOverconsume(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("usage-billing-gift-race-%s@example.com", uuid.NewString()), PasswordHash: "hash", Balance: 30,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-gift-race-" + uuid.NewString(), Name: "gift-race"})

	start := make(chan struct{})
	results := make(chan *service.UsageBillingApplyResult, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, applyErr := repo.Apply(ctx, &service.UsageBillingCommand{
				RequestID: uuid.NewString(), APIKeyID: apiKey.ID, UserID: user.ID, BalanceCost: 8,
			})
			results <- result
			errs <- applyErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for applyErr := range errs {
		require.NoError(t, applyErr)
	}
	var exemptTotal float64
	for result := range results {
		require.True(t, result.Applied)
		exemptTotal += result.ThresholdExemptCost
	}
	require.InDelta(t, 10, exemptTotal, 0.00000001)

	var balance, giftBalance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, gift_balance FROM users WHERE id = $1", user.ID).Scan(&balance, &giftBalance))
	require.InDelta(t, 14, balance, 0.00000001)
	require.Zero(t, giftBalance)
}

func TestUsageBillingRepositoryApply_DeduplicatesBalanceBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-" + uuid.NewString(),
		Name:   "billing",
		Quota:  1,
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		AccountID:           account.ID,
		AccountType:         service.AccountTypeAPIKey,
		BalanceCost:         1.25,
		APIKeyQuotaCost:     1.25,
		APIKeyRateLimitCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.True(t, result1.Applied)
	require.True(t, result1.APIKeyQuotaExhausted)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.InDelta(t, 1.25, quotaUsed, 0.000001)

	var usage5h float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT usage_5h FROM api_keys WHERE id = $1", apiKey.ID).Scan(&usage5h))
	require.InDelta(t, 1.25, usage5h, 0.000001)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM api_keys WHERE id = $1", apiKey.ID).Scan(&status))
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, status)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesSubscriptionBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-group-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-" + uuid.NewString(),
		Name:    "billing-sub",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        0,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 2.5,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.5, dailyUsage, 0.000001)
}

func TestUsageBillingRepositoryApply_SystemCustomSharedSubscriptionAccumulatesAcrossSourceAccounts(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-system-custom-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	billingGroup := mustCreateGroup(t, client, &service.Group{
		Name:                       "usage-billing-system-custom-" + uuid.NewString(),
		Platform:                   service.PlatformComposite,
		SubscriptionType:           service.SubscriptionTypeSubscription,
		SystemCustomRoutingEnabled: true,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:    user.ID,
		GroupID:   &billingGroup.ID,
		Key:       "sk-usage-billing-system-custom-" + uuid.NewString(),
		Name:      "system-custom-shared-subscription",
		Quota:     50,
		QuotaUsed: 3.25,
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: billingGroup.ID,
	})
	sourceA := mustCreateAccount(t, client, &service.Account{
		Name:     "usage-billing-source-a-" + uuid.NewString(),
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
	})
	sourceB := mustCreateAccount(t, client, &service.Account{
		Name:     "usage-billing-source-b-" + uuid.NewString(),
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	})

	commands := []*service.UsageBillingCommand{
		{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKey.ID,
			UserID:           user.ID,
			AccountID:        sourceA.ID,
			AccountType:      service.AccountTypeAPIKey,
			SubscriptionID:   &subscription.ID,
			SubscriptionCost: 0.14,
		},
		{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKey.ID,
			UserID:           user.ID,
			AccountID:        sourceB.ID,
			AccountType:      service.AccountTypeAPIKey,
			SubscriptionID:   &subscription.ID,
			SubscriptionCost: 0.57,
		},
	}

	// The production HTTP A/B route test proves both source requests emit these
	// commands; this repository test completes the contract with real PostgreSQL
	// persistence against their one shared billing subscription.
	for _, command := range commands {
		result, err := repo.Apply(ctx, command)
		require.NoError(t, err)
		require.True(t, result.Applied)
	}

	var dailyUsage, weeklyUsage, monthlyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd, weekly_usage_usd, monthly_usage_usd
		FROM user_subscriptions
		WHERE id = $1
	`, subscription.ID).Scan(&dailyUsage, &weeklyUsage, &monthlyUsage))
	require.InDelta(t, 0.71, dailyUsage, 0.00000001)
	require.InDelta(t, 0.71, weeklyUsage, 0.00000001)
	require.InDelta(t, 0.71, monthlyUsage, 0.00000001)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 100, balance, 0.00000001)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.InDelta(t, 3.25, quotaUsed, 0.00000001)

	replayed, err := repo.Apply(ctx, commands[1])
	require.NoError(t, err)
	require.False(t, replayed.Applied)

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd, weekly_usage_usd, monthly_usage_usd
		FROM user_subscriptions
		WHERE id = $1
	`, subscription.ID).Scan(&dailyUsage, &weeklyUsage, &monthlyUsage))
	require.InDelta(t, 0.71, dailyUsage, 0.00000001)
	require.InDelta(t, 0.71, weeklyUsage, 0.00000001)
	require.InDelta(t, 0.71, monthlyUsage, 0.00000001)
}

func TestUsageBillingRepositoryApply_RequestFingerprintConflict(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-conflict-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-conflict-" + uuid.NewString(),
		Name:   "billing-conflict",
	})

	requestID := uuid.NewString()
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	})
	require.NoError(t, err)

	_, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 2.50,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestUsageBillingRepositoryApply_UpdatesAccountQuota(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-account-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-account-" + uuid.NewString(),
		Name:   "billing-account",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-quota-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_limit": 100.0,
		},
	})

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        account.ID,
		AccountType:      service.AccountTypeAPIKey,
		AccountQuotaCost: 3.5,
	})
	require.NoError(t, err)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COALESCE((extra->>'quota_used')::numeric, 0) FROM accounts WHERE id = $1", account.ID).Scan(&quotaUsed))
	require.InDelta(t, 3.5, quotaUsed, 0.000001)
}

func TestUsageBillingRepositoryApply_EnqueuesSchedulerOutboxOnQuotaCrossing(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	newFixture := func(t *testing.T, extra map[string]any) (int64, int64) {
		t.Helper()
		user := mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("usage-billing-outbox-user-%d-%s@example.com", time.Now().UnixNano(), uuid.NewString()),
			PasswordHash: "hash",
		})
		apiKey := mustCreateApiKey(t, client, &service.APIKey{
			UserID: user.ID,
			Key:    "sk-usage-billing-outbox-" + uuid.NewString(),
			Name:   "billing-outbox",
		})
		account := mustCreateAccount(t, client, &service.Account{
			Name:  "usage-billing-outbox-" + uuid.NewString(),
			Type:  service.AccountTypeAPIKey,
			Extra: extra,
		})
		return apiKey.ID, account.ID
	}

	outboxCountFor := func(t *testing.T, accountID int64) int {
		t.Helper()
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND account_id = $2",
			service.SchedulerOutboxEventAccountChanged, accountID,
		).Scan(&count))
		return count
	}

	t.Run("daily_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_daily_limit": 10.0,
		})
		// 第一次低于日限额：不应入队 outbox
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 4,
		})
		require.NoError(t, err)
		require.Equal(t, 0, outboxCountFor(t, accountID), "below limit should not enqueue")

		// 第二次跨越日限额：应入队一次 outbox
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 8,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "crossing daily limit should enqueue once")

		// 再次递增（已超）：不应重复入队
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 2,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "subsequent increments beyond limit should not re-enqueue")
	})

	t.Run("weekly_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_weekly_limit": 10.0,
		})
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 15, // 单次即跨越
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "single-shot crossing weekly limit should enqueue once")
	})
}

func TestDashboardAggregationRepositoryCleanupUsageBillingDedup_BatchDeletesOldRows(t *testing.T) {
	ctx := context.Background()
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	oldRequestID := "dedup-old-" + uuid.NewString()
	newRequestID := "dedup-new-" + uuid.NewString()
	oldCreatedAt := time.Now().UTC().AddDate(0, 0, -400)
	newCreatedAt := time.Now().UTC().Add(-time.Hour)

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, created_at)
		VALUES ($1, 1, $2, $3), ($4, 1, $5, $6)
	`,
		oldRequestID, strings.Repeat("a", 64), oldCreatedAt,
		newRequestID, strings.Repeat("b", 64), newCreatedAt,
	)
	require.NoError(t, err)

	require.NoError(t, repo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	var oldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", oldRequestID).Scan(&oldCount))
	require.Equal(t, 0, oldCount)

	var newCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", newRequestID).Scan(&newCount))
	require.Equal(t, 1, newCount)

	var archivedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup_archive WHERE request_id = $1", oldRequestID).Scan(&archivedCount))
	require.Equal(t, 1, archivedCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesAgainstArchivedKey(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	aggRepo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-archive-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-archive-" + uuid.NewString(),
		Name:   "billing-archive",
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE usage_billing_dedup
		SET created_at = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, time.Now().UTC().AddDate(0, 0, -400), requestID, apiKey.ID)
	require.NoError(t, err)
	require.NoError(t, aggRepo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)
}
