package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// The wallet recheck reads authoritative storage independently of the Redis cache.
type inflightTestUserRepo struct {
	service.UserRepository
	balance float64
}

func (r *inflightTestUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Status: service.StatusActive, Balance: r.balance}, nil
}

func newInflightTestEnv(t *testing.T, enabled bool, ttlSeconds int, balance float64) (*miniredis.Miniredis, *billingCache, *service.BillingCacheService) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{
		Enabled:          enabled,
		TTLSeconds:       ttlSeconds,
		DefaultMaxTokens: 8192,
	}
	svc := service.NewBillingCacheService(cache, &inflightTestUserRepo{balance: balance}, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	return mr, cache, svc
}

func inflightCount(t *testing.T, cache *billingCache, userID int64) int64 {
	t.Helper()
	zkey, _ := billingInflightKeys(userID)
	n, err := cache.rdb.ZCard(context.Background(), zkey).Result()
	require.NoError(t, err)
	return n
}

func TestInflightReservation_ConcurrentAdmitsOnlyWhatBalanceCovers(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	user := &service.User{ID: 42}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	const n = 20
	var admitted, rejected atomic.Int32
	releases := make(chan func(), n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.3)
			if err != nil {
				if !errors.Is(err, service.ErrBalanceInFlight) {
					t.Errorf("unexpected error: %v", err)
				}
				rejected.Add(1)
				return
			}
			admitted.Add(1)
			releases <- release
		}()
	}
	close(start)
	wg.Wait()
	close(releases)

	// Balance 1.0 and estimate 0.3 allow three simultaneous reservations.
	require.Equal(t, int32(3), admitted.Load())
	require.Equal(t, int32(n-3), rejected.Load())
	require.Equal(t, int64(3), inflightCount(t, cache, user.ID))

	for release := range releases {
		release()
		release() // idempotent
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))

	// After release, capacity is available again.
	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.3)
	require.NoError(t, err)
	release()
}

func TestInflightReservation_FirstRequestRequiresSufficientBalance(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 0.01)
	ctx := context.Background()
	user := &service.User{ID: 7}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0.01))

	_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 5)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.Zero(t, inflightCount(t, cache, user.ID))
	balance, err := svc.GetUserBalance(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 0.01, balance)
	release2, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.001)
	require.NoError(t, err)
	release2()
}

func TestInflightReservation_ExactRemainingBalanceIsUsable(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 0.3)
	ctx := context.Background()
	user := &service.User{ID: 7}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0.3))
	first, err := svc.ReserveInflight(ctx, user, nil, nil, 0.2)
	require.NoError(t, err)
	defer first.Release()
	second, err := svc.ReserveInflight(ctx, user, nil, nil, 0.1)
	require.NoError(t, err, "wallet precision must allow the exact remaining amount")
	defer second.Release()
	_, err = svc.ReserveInflight(ctx, user, nil, nil, 0.00000001)
	require.ErrorIs(t, err, service.ErrBalanceInFlight)
}

func TestInflightReservation_RejectsIncidentSizedFirstRequest(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 33.69240739)
	ctx := context.Background()
	user := &service.User{ID: 7}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 33.69240739))
	res, err := svc.ReserveInflight(ctx, user, nil, nil, 128.09169)
	require.Nil(t, res)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.Zero(t, inflightCount(t, cache, user.ID))
}

func TestInflightReservation_UsesBalanceAtAtomicAdmission(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	const userID = int64(7)
	require.NoError(t, cache.SetUserBalance(ctx, userID, 1))
	allowed, _, err := cache.ReserveInflightBalance(ctx, userID, "first", 0.6, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	staleBalance, err := cache.GetUserBalance(ctx, userID)
	require.NoError(t, err)
	// The first request settles after another request has read the wallet.
	require.NoError(t, cache.DeductUserBalance(ctx, userID, 0.6))
	require.NoError(t, cache.ReleaseInflightBalance(ctx, userID, "first"))
	allowed, _, err = cache.ReserveInflightBalance(ctx, userID, "second", 0.6, staleBalance, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed, "a settled reservation must not expose an old wallet snapshot")
	require.Zero(t, inflightCount(t, cache, userID))
}

func TestInflightReservation_MissingWalletDoesNotReuseOldBalance(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	const userID = int64(7)
	require.NoError(t, cache.SetUserBalance(ctx, userID, 1))
	require.NoError(t, cache.InvalidateUserBalance(ctx, userID))
	allowed, _, err := cache.ReserveInflightBalance(ctx, userID, "stale-wallet", 0.6, 1, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Zero(t, inflightCount(t, cache, userID))
}

func TestInflightReservation_InvalidationRejectsOlderCacheFill(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	version, err := cache.GetUserBalanceVersion(ctx, 7)
	require.NoError(t, err)
	require.NoError(t, cache.InvalidateUserBalance(ctx, 7))
	stored, err := cache.SetUserBalanceIfVersion(ctx, 7, 1, version)
	require.NoError(t, err)
	require.False(t, stored)
	current, err := cache.GetUserBalanceVersion(ctx, 7)
	require.NoError(t, err)
	stored, err = cache.SetUserBalanceIfVersion(ctx, 7, 0.4, current)
	require.NoError(t, err)
	require.True(t, stored)
	balance, err := cache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, 0.4, balance)
}

func TestInflightReservation_ExpiredVersionCannotMatchNewSettlement(t *testing.T) {
	mr, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	require.NoError(t, cache.InvalidateUserBalance(ctx, 7))
	version, err := cache.GetUserBalanceVersion(ctx, 7)
	require.NoError(t, err)
	mr.FastForward(11 * time.Minute)
	require.NoError(t, cache.DeductUserBalance(ctx, 7, 0.6))
	stored, err := cache.SetUserBalanceIfVersion(ctx, 7, 1, version)
	require.NoError(t, err)
	require.False(t, stored, "an expired generation must not be reused by the next settlement")
}

func TestInflightReservation_ReplayedInvalidationCannotRestoreOldVersion(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	keys := []string{billingBalanceKey(7), billingBalanceVersionKey(7), "billing:inflight:{7}", "billing:balance:sequence"}
	require.NoError(t, invalidateBalanceScript.Run(ctx, cache.rdb, keys, 600, "replayed-operation").Err())
	version, err := cache.GetUserBalanceVersion(ctx, 7)
	require.NoError(t, err)
	require.NoError(t, cache.DeductUserBalance(ctx, 7, 0.6))
	require.NoError(t, invalidateBalanceScript.Run(ctx, cache.rdb, keys, 600, "replayed-operation").Err())
	stored, err := cache.SetUserBalanceIfVersion(ctx, 7, 1, version)
	require.NoError(t, err)
	require.False(t, stored)
}

func TestInflightReservation_ExpiredHoldRefreshesPossiblyStaleWallet(t *testing.T) {
	mr, cache, svc := newInflightTestEnv(t, true, 60, 0.4)
	ctx := context.Background()
	require.NoError(t, cache.SetUserBalance(ctx, 7, 1))
	allowed, _, err := cache.ReserveInflightBalance(ctx, 7, "unreconciled", 0.6, 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed)
	mr.FastForward(2 * time.Second)
	res, err := svc.ReserveInflight(ctx, &service.User{ID: 7}, nil, nil, 0.6)
	require.Nil(t, res)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
}

func TestInflightReservation_RetryOfRegisteredRequestIsIdempotent(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	require.NoError(t, cache.SetUserBalance(ctx, 7, 1))
	for attempt := 0; attempt < 2; attempt++ {
		allowed, _, err := cache.ReserveInflightBalance(ctx, 7, "same-request", 0.6, 1, time.Minute)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	require.Equal(t, int64(1), inflightCount(t, cache, 7))
}

type pausedBalancePublishCache struct {
	*billingCache
	paused    atomic.Bool
	published chan struct{}
	resume    chan struct{}
}

func (c *pausedBalancePublishCache) SetUserBalanceIfVersion(ctx context.Context, userID int64, balance float64, version string) (bool, error) {
	stored, err := c.billingCache.SetUserBalanceIfVersion(ctx, userID, balance, version)
	if stored && c.paused.CompareAndSwap(false, true) {
		close(c.published)
		<-c.resume
	}
	return stored, err
}

func TestInflightReservation_RecheckDoesNotJoinOlderPublishedLoad(t *testing.T) {
	for _, strategy := range []string{"recheck", "cache_miss"} {
		t.Run(strategy, func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			cache := &pausedBalancePublishCache{billingCache: &billingCache{rdb: rdb}, published: make(chan struct{}), resume: make(chan struct{})}
			var resumeOnce sync.Once
			resume := func() { resumeOnce.Do(func() { close(cache.resume) }) }
			defer resume()
			repo := &inflightSnapshotUserRepo{read: make(chan struct{}), resume: make(chan struct{})}
			close(repo.resume)
			cfg := &config.Config{}
			cfg.Billing.InflightReservation.Enabled = true
			svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(svc.Stop)
			oldLoad := make(chan struct{}, 1)
			go func() {
				_, _ = svc.GetUserBalance(context.Background(), 7)
				oldLoad <- struct{}{}
			}()
			select {
			case <-cache.published:
			case <-time.After(5 * time.Second):
				t.Fatal("first balance load did not publish")
			}
			repo.mu.Lock()
			repo.balance = 1
			repo.mu.Unlock()
			require.NoError(t, cache.InvalidateUserBalance(context.Background(), 7))
			refreshed := make(chan float64, 1)
			go func() {
				var balance float64
				if strategy == "recheck" {
					balance, _ = svc.RecheckUserBalance(context.Background(), 7)
				} else {
					balance, _ = svc.GetUserBalance(context.Background(), 7)
				}
				refreshed <- balance
			}()
			select {
			case balance := <-refreshed:
				require.Equal(t, 1.0, balance)
			case <-time.After(3 * time.Second):
				t.Fatal("fresh wallet recheck joined a load published before invalidation")
			}
			resume()
			<-oldLoad

		})
	}
}

type inflightSnapshotUserRepo struct {
	service.UserRepository
	mu       sync.Mutex
	balance  float64
	read     chan struct{}
	resume   chan struct{}
	readOnce sync.Once
}

func (r *inflightSnapshotUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	r.mu.Lock()
	balance := r.balance
	r.mu.Unlock()
	r.readOnce.Do(func() {
		close(r.read)
		<-r.resume
	})
	return &service.User{ID: id, Status: service.StatusActive, Balance: balance}, nil
}

func TestInflightReservation_ColdLoadDoesNotRestorePreSettlementBalance(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}
	repo := &inflightSnapshotUserRepo{balance: 1, read: make(chan struct{}), resume: make(chan struct{})}
	var resumeOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(repo.resume) }) }
	t.Cleanup(resume)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	type outcome struct {
		balance float64
		err     error
	}
	loaded := make(chan outcome, 1)
	go func() {
		balance, err := svc.GetUserBalance(context.Background(), 7)
		loaded <- outcome{balance, err}
	}()
	select {
	case <-repo.read:
	case <-time.After(5 * time.Second):
		t.Fatal("wallet database load did not begin")
	}
	repo.mu.Lock()
	repo.balance = 0.4
	repo.mu.Unlock()
	require.NoError(t, cache.DeductUserBalance(context.Background(), 7, 0.6))
	resume()
	result := <-loaded
	require.NoError(t, result.err)
	require.Equal(t, 0.4, result.balance, "a database snapshot taken before settlement must be retried")
	allowed, _, err := cache.ReserveInflightBalance(context.Background(), 7, "after-load", 0.6, result.balance, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestInflightReservation_TTLExpiryFreesLeakedReservation(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 1, 1)
	ctx := context.Background()
	user := &service.User{ID: 9}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8) // leaked: never released
	require.NoError(t, err)
	_, err = svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8)
	require.ErrorIs(t, err, service.ErrBalanceInFlight)

	time.Sleep(1100 * time.Millisecond)
	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8)
	require.NoError(t, err)
	require.Equal(t, int64(1), inflightCount(t, cache, user.ID))
	release()
}

func TestInflightReservation_DisabledKeepsLegacyBehavior(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, false, 60, 0.01)
	ctx := context.Background()
	user := &service.User{ID: 11}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0.01))
	require.False(t, svc.InflightReservationEnabled())
	for i := 0; i < 5; i++ {
		_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 10)
		require.NoError(t, err)
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_SubscriptionUnaffected(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 0)
	ctx := context.Background()
	user := &service.User{ID: 12}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0))
	group := &service.Group{ID: 1, SubscriptionType: service.SubscriptionTypeSubscription}
	sub := &service.UserSubscription{ID: 1}
	for i := 0; i < 5; i++ {
		_, err := svc.ReserveInflightBalance(ctx, user, group, sub, 10)
		require.NoError(t, err)
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_ZeroEstimateSkips(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	user := &service.User{ID: 13}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1))
	_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0)
	require.NoError(t, err)
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

// downReserveCache 余额读取正常，但预留走一个已关闭的 Redis。
type downReserveCache struct {
	service.BillingCache
	down *billingCache
}

func (d *downReserveCache) ReserveInflightBalance(ctx context.Context, userID int64, requestID string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	return d.down.ReserveInflightBalance(ctx, userID, requestID, amount, balance, ttl)
}

func (d *downReserveCache) ReleaseInflightBalance(ctx context.Context, userID int64, requestID string) error {
	return d.down.ReleaseInflightBalance(ctx, userID, requestID)
}

func TestInflightReservation_RedisDownFailsClosed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	up := &billingCache{rdb: rdb}
	require.NoError(t, up.SetUserBalance(context.Background(), 5, 0.01))

	downMR := miniredis.RunT(t)
	downRDB := redis.NewClient(&redis.Options{Addr: downMR.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = downRDB.Close() })
	downMR.Close()

	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	svc := service.NewBillingCacheService(&downReserveCache{BillingCache: up, down: &billingCache{rdb: downRDB}}, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	user := &service.User{ID: 5}
	for i := 0; i < 3; i++ {
		_, err := svc.ReserveInflightBalance(context.Background(), user, nil, nil, 100)
		require.ErrorIs(t, err, service.ErrBillingServiceUnavailable)
	}
}

func TestInflightReservation_ScriptErrorSurfaces(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	cache := &billingCache{rdb: rdb}
	mr.Close()
	_, _, err := cache.ReserveInflightBalance(context.Background(), 1, "r", 1, 1, time.Second)
	require.Error(t, err)
	require.False(t, errors.Is(err, context.Canceled))
	_ = rdb.Close()
}

// 回归：原实现在 handler 返回时即释放预留，而计费（RecordUsage → 余额缓存扣减）是异步的，
// 窗口内的顺序请求看到「在途=0 且余额未扣」全部放行（余额 $1、5×$0.9 → -4.40）。
// 现在预留由计费任务在余额缓存扣减之后才释放。
func TestInflightReservation_SequentialInBillingWindowAdmitsOnlyWhatBalanceCovers(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	user := &service.User{ID: 77}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	const cost = 0.9
	var pendingBilling []func()
	admitted := 0
	for i := 0; i < 5; i++ {
		res, err := svc.ReserveInflight(ctx, user, nil, nil, cost)
		if err != nil {
			require.ErrorIs(t, err, service.ErrBalanceInFlight)
			continue
		}
		admitted++
		taskDone := res.Acquire() // handler submits the async billing task
		res.HandlerDone()         // handler returns before billing lands
		pendingBilling = append(pendingBilling, func() {
			require.NoError(t, cache.DeductUserBalance(ctx, user.ID, cost))
			taskDone()
		})
	}
	require.Equal(t, 1, admitted, "requests arriving before billing lands must not be admitted")

	for _, bill := range pendingBilling {
		bill()
	}
	bal, err := cache.GetUserBalance(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 0.1, bal, 1e-9)
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_RenewalKeepsStreamingReservationAlive(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 1, 1)
	ctx := context.Background()
	user := &service.User{ID: 78}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	res, err := svc.ReserveInflight(ctx, user, nil, nil, 0.8)
	require.NoError(t, err)
	time.Sleep(2500 * time.Millisecond) // streaming well past the 1s TTL
	_, err = svc.ReserveInflight(ctx, user, nil, nil, 0.8)
	require.ErrorIs(t, err, service.ErrBalanceInFlight, "renewed reservation must still count")
	require.Equal(t, int64(1), inflightCount(t, cache, user.ID))

	res.HandlerDone()
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))

	ok, err := cache.RenewInflightBalance(ctx, user.ID, "missing", time.Second)
	require.NoError(t, err)
	require.False(t, ok, "renew must not resurrect a released reservation")
}

func TestInflightReservation_RenewDoesNotResurrectExpiredMember(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()
	userID := int64(79)
	zkey, hkey := billingInflightKeys(userID)
	// 已过期但尚未被惰性清理的成员（score 在过去）。
	require.NoError(t, cache.rdb.ZAdd(ctx, zkey, redis.Z{Score: float64(time.Now().UnixMilli() - 1000), Member: "stale"}).Err())
	require.NoError(t, cache.rdb.HSet(ctx, hkey, "stale", "0.5").Err())

	ok, err := cache.RenewInflightBalance(ctx, userID, "stale", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "renew must not resurrect an expired member")
	require.Equal(t, int64(0), inflightCount(t, cache, userID))
	exists, err := cache.rdb.HExists(ctx, hkey, "stale").Result()
	require.NoError(t, err)
	require.False(t, exists)
}

func TestInflightScopeReservation_ReturnsOthersAndReleases(t *testing.T) {
	mr, cache, _ := newInflightTestEnv(t, true, 60, 1)
	ctx := context.Background()

	others, err := cache.ReserveInflightScope(ctx, "apikey:7", "r1", 3, time.Minute)
	require.NoError(t, err)
	require.Zero(t, others)
	others, err = cache.ReserveInflightScope(ctx, "apikey:7", "r2", 2, time.Minute)
	require.NoError(t, err)
	require.InDelta(t, 3, others, 1e-9, "返回登记前其他在途预留的合计")
	others, err = cache.ReserveInflightScope(ctx, "sub:7", "r3", 5, time.Minute)
	require.NoError(t, err)
	require.Zero(t, others, "不同维度互不影响")

	require.NoError(t, cache.ReleaseInflightScope(ctx, "apikey:7", "r1"))
	require.NoError(t, cache.ReleaseInflightScope(ctx, "apikey:7", "r1"))
	others, err = cache.ReserveInflightScope(ctx, "apikey:7", "r4", 1, time.Minute)
	require.NoError(t, err)
	require.InDelta(t, 2, others, 1e-9)

	mr.FastForward(2 * time.Minute)
	others, err = cache.ReserveInflightScope(ctx, "apikey:7", "r5", 1, time.Minute)
	require.NoError(t, err)
	require.Zero(t, others, "过期预留不再计入")
}

func TestReserveFixedCharge_ConcurrentChargesAdmitOnlyWhatAPIKeyQuotaCovers(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60, 100)
	ctx := context.Background()
	user := &service.User{ID: 42}
	apiKey := &service.APIKey{ID: 9, UserID: user.ID, Quota: 1.0}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 100))

	const n = 20
	var admitted atomic.Int32
	reservations := make(chan *service.InflightReservation, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := svc.ReserveFixedCharge(ctx, user, apiKey, nil, nil, 0.3)
			if err != nil {
				if !errors.Is(err, service.ErrAPIKeyQuotaExhausted) {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			admitted.Add(1)
			reservations <- res
		}()
	}
	close(start)
	wg.Wait()
	close(reservations)

	require.Equal(t, int32(3), admitted.Load(), "Key 额度 1.0、每笔 0.3：同时在途的不超过 3 笔")
	for res := range reservations {
		res.Release()
	}
	zkey, _ := billingInflightScopeKeys("apikey:9")
	remaining, err := cache.rdb.ZCard(ctx, zkey).Result()
	require.NoError(t, err)
	require.Zero(t, remaining)
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}
