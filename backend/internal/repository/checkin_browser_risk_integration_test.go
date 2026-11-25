//go:build integration

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestConcurrentCheckinBrowserClaimsAndApprovedRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := NewSettingRepository(integrationEntClient)
	svc := service.NewCheckinService(integrationEntClient, nil, nil)
	svc.SetSettingRepository(repo)
	values := map[string]string{"checkin_browser_risk_mode": "enforce", service.SettingKeyCheckinMinTotalUsageUSD: "0", service.SettingKeyCheckinMinTotalRechargeUSD: "0", service.SettingKeyCheckinEnabled: "true", service.SettingKeyCheckinMinDailyUsageCount: "0"}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	before, err := repo.GetMultiple(ctx, keys)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, k := range keys {
			if v, ok := before[k]; ok {
				_ = repo.Set(context.Background(), k, v)
			} else {
				_ = repo.Delete(context.Background(), k)
			}
		}
	})
	require.NoError(t, repo.SetMultiple(ctx, values))
	a := createPostgresCheckinIntegrationUser(t, ctx, "browser-a")
	b := createPostgresCheckinIntegrationUser(t, ctx, "browser-b")
	t.Cleanup(func() {
		for _, id := range []int64{a.ID, b.ID} {
			_, _ = integrationDB.Exec("DELETE FROM redeem_codes WHERE used_by=$1", id)
			_, _ = integrationDB.Exec("DELETE FROM users WHERE id=$1", id)
		}
	})
	browser := service.CheckinBrowser{Hash: strings.Repeat("8", 64), CookiePresent: true, IP: "192.0.2.3", UserAgent: "test browser"}
	start := make(chan struct{})
	type outcome struct {
		id  int64
		err error
	}
	results := make(chan outcome, 2)
	for _, id := range []int64{a.ID, b.ID} {
		go func() {
			<-start
			_, err := svc.CheckinWithBrowser(ctx, id, browser)
			results <- outcome{id, err}
		}()
	}
	close(start)
	var winner, held int64
	for range 2 {
		o := <-results
		if errors.Is(o.err, service.ErrCheckinReviewRequired) {
			held = o.id
		} else {
			require.NoError(t, o.err)
			winner = o.id
		}
	}
	require.NotZero(t, winner)
	require.NotZero(t, held)
	day := time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format(time.DateOnly)
	err = svc.DecideCheckinBrowserReview(ctx, held, browser.Hash, day, "approved", winner, "confirmed shared device")
	require.NoError(t, err)
	for range 2 {
		_, err = svc.CheckinWithBrowser(ctx, held, browser)
		require.NoError(t, err)
	}
	for _, id := range []int64{winner, held} {
		var records, ledger int
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_checkins WHERE user_id=$1", id).Scan(&records))
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM redeem_codes WHERE used_by=$1 AND type=$2", id, service.AdjustmentTypeCheckinReward).Scan(&ledger))
		require.Equal(t, 1, records)
		require.Equal(t, 1, ledger)
	}
}
