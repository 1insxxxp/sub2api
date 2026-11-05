//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type categoryDefaultFreshAccountRepo struct {
	AccountRepository
	account  *Account
	err      error
	calls    int
	deadline time.Time
}

func (r *categoryDefaultFreshAccountRepo) GetByID(ctx context.Context, _ int64) (*Account, error) {
	r.calls++
	r.deadline, _ = ctx.Deadline()
	return r.account, r.err
}

type categoryDefaultStaleAccountCache struct {
	SchedulerCache
	account *Account
	reads   int
}

func (c *categoryDefaultStaleAccountCache) GetAccount(context.Context, int64) (*Account, error) {
	c.reads++
	return c.account, nil
}

func categoryDefaultAccountReadContext() context.Context {
	entry, source := int64(100), int64(9)
	key := &APIKey{ID: 1, GroupID: &source, CategoryDefaultGroupID: &entry, Group: &Group{ID: source, Platform: PlatformGemini, Status: StatusActive, Hydrated: true}}
	ctx := context.WithValue(context.Background(), ctxkey.APIKey, key)
	return context.WithValue(ctx, ctxkey.Group, key.Group)
}
func categoryDefaultReadAccount(target string) *Account {
	return &Account{ID: 703, Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{9}, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"alias": target}}}
}

func TestCategoryDefaultAccountReadBypassesStaleCacheWithBoundedDBRead(t *testing.T) {
	cache := &categoryDefaultStaleAccountCache{account: categoryDefaultReadAccount("gemini-2.5-flash")}
	repo := &categoryDefaultFreshAccountRepo{account: categoryDefaultReadAccount("gemini-2.5-pro")}
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)
	got, err := svc.GetAccount(categoryDefaultAccountReadContext(), 703)
	require.NoError(t, err)
	require.Equal(t, "gemini-2.5-pro", got.GetMappedModel("alias"))
	require.Equal(t, 1, repo.calls)
	require.Zero(t, cache.reads)
	require.False(t, repo.deadline.IsZero())
	require.WithinDuration(t, time.Now(), repo.deadline, 4*time.Second)
	require.Equal(t, "gemini-2.5-flash", cache.account.GetMappedModel("alias"), "do not mutate a shared cached account")
}

func TestCategoryDefaultAccountReadFailsClosedWithoutCacheFallback(t *testing.T) {
	for _, state := range []string{"database-error", "removed-binding", "disabled", "not-schedulable", "missing", "wrong-id", "missing-source"} {
		t.Run(state, func(t *testing.T) {
			cache := &categoryDefaultStaleAccountCache{account: categoryDefaultReadAccount("gemini-2.5-flash")}
			repo := &categoryDefaultFreshAccountRepo{account: categoryDefaultReadAccount("gemini-2.5-pro")}
			ctx := categoryDefaultAccountReadContext()
			switch state {
			case "database-error":
				repo.err = errors.New("database unavailable")
			case "removed-binding":
				repo.account.GroupIDs = []int64{10}
			case "disabled":
				repo.account.Status = StatusDisabled
			case "not-schedulable":
				repo.account.Schedulable = false
			case "missing":
				repo.account = nil
			case "wrong-id":
				repo.account.ID++
			case "missing-source":
				ctx.Value(ctxkey.APIKey).(*APIKey).GroupID = nil
			}
			svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)
			got, err := svc.GetAccount(ctx, 703)
			require.ErrorIs(t, err, ErrCategoryDefaultSourceUnavailable)
			require.Nil(t, got)
			require.Zero(t, cache.reads)
		})
	}
}

func TestCategoryDefaultAccountReadPreservesOrdinaryCacheAndCancellation(t *testing.T) {
	for _, state := range []string{"ordinary", "canceled"} {
		t.Run(state, func(t *testing.T) {
			cache := &categoryDefaultStaleAccountCache{account: categoryDefaultReadAccount("gemini-2.5-flash")}
			repo := &categoryDefaultFreshAccountRepo{account: categoryDefaultReadAccount("gemini-2.5-pro")}
			ctx := categoryDefaultAccountReadContext()
			if state == "ordinary" {
				ctx.Value(ctxkey.APIKey).(*APIKey).CategoryDefaultGroupID = nil
			} else {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			got, err := NewSchedulerSnapshotService(cache, nil, repo, nil, nil).GetAccount(ctx, 703)
			require.Zero(t, repo.calls)
			if state == "ordinary" {
				require.NoError(t, err)
				require.Same(t, cache.account, got)
				require.Equal(t, 1, cache.reads)
			} else {
				require.ErrorIs(t, err, context.Canceled)
				require.Nil(t, got)
				require.Zero(t, cache.reads)
			}
		})
	}
}

func TestCategoryDefaultAccountHydrationForwardsFreshGeminiMappingAcrossProtocols(t *testing.T) {
	for _, protocol := range []string{"native", "chat-completions", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			ctx := categoryDefaultAccountReadContext()
			stale := categoryDefaultReadAccount("gemini-2.5-flash")
			cache := &categoryDefaultStaleAccountCache{account: stale}
			repo := &categoryDefaultFreshAccountRepo{account: categoryDefaultReadAccount("gemini-2.5-pro")}
			snapshot := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)
			// Both public handlers hydrate the actual selection via GatewayService.
			gateway := &GatewayService{schedulerSnapshot: snapshot}
			selected, err := gateway.hydrateSelectedAccount(ctx, stale)
			require.NoError(t, err)
			svc, upstream := newGeminiVertexAPIKeyTestService()
			c, recorder := newGeminiNativeTestContext(t)
			c.Request = c.Request.WithContext(ctx)
			c.Set("api_key", ctx.Value(ctxkey.APIKey))
			var result *ForwardResult
			switch protocol {
			case "native":
				result, err = svc.ForwardNative(ctx, c, selected, "alias", "generateContent", false, geminiSignalTestRequest())
			case "chat-completions":
				result, err = svc.ForwardAsChatCompletions(ctx, c, selected, []byte(`{"model":"alias","messages":[{"role":"user","content":"hi"}]}`))
			case "messages":
				result, err = svc.Forward(ctx, c, selected, []byte(`{"model":"alias","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
			}
			require.NoError(t, err)
			require.Equal(t, 200, recorder.Code)
			require.Equal(t, 1, upstream.calls)
			require.Contains(t, upstream.lastReq.URL.Path, "/models/gemini-2.5-pro:generateContent")
			require.Equal(t, "gemini-2.5-pro", result.UpstreamModel)
			require.Zero(t, cache.reads)
		})
	}
}
