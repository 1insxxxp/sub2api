//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type usagePersistenceFixture struct {
	cmd                      *service.UsageBillingCommand
	userID, keyID, accountID int64
	requestID                string
}

func newUsagePersistenceFixture(t *testing.T, client *dbent.Client) usagePersistenceFixture {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	user := mustCreateUser(t, client, &service.User{Email: "usage-persistence-" + suffix + "@example.invalid", PasswordHash: "hash", Balance: 10})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-durable-" + suffix, Name: "durable-fixture", Quota: 20})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-persistence-" + suffix, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance=0.5 WHERE id=$1", user.ID)
	require.NoError(t, err)
	requestID := uuid.NewString()
	log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: requestID, Model: "gemini-2.5-flash", RequestedModel: "durable-alias", InputTokens: 12, OutputTokens: 2, UpstreamOutputTokens: 10000,
		InputCost: 1, OutputCost: 1, TotalCost: 2, ActualCost: 2, RateMultiplier: 1, Stream: true, CreatedAt: time.Now().Add(-time.Minute).UTC(),
		Outcome: &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, OutputBytes: 14, StreamCompleted: true, CollectorVersion: 1}}
	fixture := usagePersistenceFixture{cmd: &service.UsageBillingCommand{RequestID: requestID, APIKeyID: key.ID, UserID: user.ID, AccountID: account.ID, BalanceCost: 2, APIKeyQuotaCost: 2, UsageLog: log}, userID: user.ID, keyID: key.ID, accountID: account.ID, requestID: requestID}
	t.Cleanup(func() {
		for _, table := range []string{"usage_persistence_outbox", "usage_response_outcomes", "usage_logs", "usage_billing_dedup", "usage_billing_dedup_archive"} {
			_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM "+table+" WHERE api_key_id=$1", key.ID)
			require.NoError(t, err)
		}
		for _, target := range []struct {
			table string
			id    int64
		}{{"api_keys", key.ID}, {"accounts", account.ID}, {"users", user.ID}} {
			_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM "+target.table+" WHERE id=$1", target.id)
			require.NoError(t, err)
		}
	})
	return fixture
}
func usagePersistenceCounts(t *testing.T, f usagePersistenceFixture) (balance, quota float64, usage, outcome, dedup, pending int) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", f.keyID).Scan(&quota))
	for _, target := range []struct {
		table string
		dest  *int
	}{{"usage_logs", &usage}, {"usage_response_outcomes", &outcome}, {"usage_billing_dedup", &dedup}, {"usage_persistence_outbox", &pending}} {
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM "+target.table+" WHERE api_key_id=$1", f.keyID).Scan(target.dest))
	}
	return
}
func usagePersistenceRejectInsert(t *testing.T, table, predicate string) func() {
	t.Helper()
	name := "reject_usage_persistence_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := integrationDB.ExecContext(context.Background(), fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN RAISE EXCEPTION 'injected durable persistence failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER %s BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, name, predicate, name, table, name))
	require.NoError(t, err)
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			_, err := integrationDB.ExecContext(context.Background(), fmt.Sprintf("DROP TRIGGER %s ON %s; DROP FUNCTION %s()", name, table, name))
			require.NoError(t, err)
		})
	}
	t.Cleanup(cleanup)
	return cleanup
}
func claimUsagePersistenceFixture(t *testing.T, repo service.UsagePersistenceOutboxRepository, worker string) service.UsagePersistenceEvent {
	t.Helper()
	events, err := repo.Claim(context.Background(), worker, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, events, 1)
	return events[0]
}

func TestUsagePersistenceOutbox_EnqueueFailureRollsBackActualWalletQuotaAndDedup(t *testing.T) {
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	usagePersistenceRejectInsert(t, "usage_persistence_outbox", fmt.Sprintf("NEW.api_key_id=%d", f.keyID))
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(context.Background(), f.cmd)
	require.Error(t, err)
	require.Nil(t, result)
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 10.0, balance)
	require.Zero(t, quota)
	require.Zero(t, usage)
	require.Zero(t, outcome)
	require.Zero(t, dedup)
	require.Zero(t, pending)
}

func TestUsagePersistenceOutbox_CommitThenRestartDeliveryIsAtomicAndDoesNotChargeAgain(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Zero(t, usage)
	require.Zero(t, outcome)
	require.Equal(t, 1, dedup)
	require.Equal(t, 1, pending)
	first := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, first, "worker-after-commit")
	require.NotZero(t, event.CreatedAt)
	require.Zero(t, event.Attempts)
	restarted := NewUsagePersistenceOutboxRepository(client, integrationDB)
	require.NoError(t, restarted.Deliver(ctx, event.ID, "worker-after-commit"))
	require.NoError(t, restarted.Deliver(ctx, event.ID, "worker-after-commit"), "acknowledged delivery is idempotent")
	balance, quota, usage, outcome, dedup, pending = usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)
	var upstream int
	var exempt float64
	var created time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT upstream_output_tokens,threshold_exempt_cost,created_at FROM usage_logs WHERE api_key_id=$1 AND request_id=$2", f.keyID, f.requestID).Scan(&upstream, &exempt, &created))
	require.Equal(t, 10000, upstream)
	require.Equal(t, 0.5, exempt)
	require.WithinDuration(t, f.cmd.UsageLog.CreatedAt, created, time.Microsecond)
}

func TestUsagePersistenceOutbox_OutcomeFailureRollsBackUsageAndKeepsTaskForRetry(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	_, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "worker-outcome")
	removeFailure := usagePersistenceRejectInsert(t, "usage_response_outcomes", fmt.Sprintf("NEW.api_key_id=%d", f.keyID))
	require.Error(t, repo.Deliver(ctx, event.ID, "worker-outcome"))
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Zero(t, usage)
	require.Zero(t, outcome)
	require.Equal(t, 1, dedup)
	require.Equal(t, 1, pending)
	require.NoError(t, repo.RetryClaimed(ctx, event.ID, "worker-outcome", time.Now().Add(-time.Second), "usage_persistence_delivery_failed"))
	removeFailure()
	retried := claimUsagePersistenceFixture(t, repo, "worker-retry")
	require.Equal(t, event.ID, retried.ID)
	require.Equal(t, 1, retried.Attempts)
	require.NoError(t, repo.Deliver(ctx, event.ID, "worker-retry"))
	_, _, usage, outcome, _, pending = usagePersistenceCounts(t, f)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Zero(t, pending)
}

func TestUsagePersistenceOutbox_LeaseOwnershipAndCrashRecovery(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	_, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "worker-one")
	other, err := repo.Claim(ctx, "worker-two", 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, other)
	require.Error(t, repo.Deliver(ctx, event.ID, "worker-two"))
	require.Error(t, repo.RetryClaimed(ctx, event.ID, "worker-two", time.Now(), "usage_persistence_delivery_failed"))
	_, err = integrationDB.ExecContext(ctx, "UPDATE usage_persistence_outbox SET claimed_at=NOW()-INTERVAL '2 minutes' WHERE id=$1", event.ID)
	require.NoError(t, err)
	recovered := claimUsagePersistenceFixture(t, repo, "worker-two")
	require.Equal(t, event.ID, recovered.ID)
	require.Error(t, repo.Deliver(ctx, event.ID, "worker-one"))
	require.NoError(t, repo.Deliver(ctx, event.ID, "worker-two"))
	balance, quota, usage, outcome, _, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Zero(t, pending)
}

func TestUsagePersistenceOutbox_TwoReplicasClaimDifferentEvents(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	first := newUsagePersistenceFixture(t, client)
	second := newUsagePersistenceFixture(t, client)
	billing := NewUsageBillingRepository(client, integrationDB)
	for _, f := range []usagePersistenceFixture{first, second} {
		_, err := billing.Apply(ctx, f.cmd)
		require.NoError(t, err)
	}
	type claimed struct {
		event  service.UsagePersistenceEvent
		worker string
		err    error
	}
	results := make(chan claimed, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, worker := range []string{"replica-a", "replica-b"} {
		workers.Add(1)
		go func(worker string) {
			defer workers.Done()
			<-start
			events, err := NewUsagePersistenceOutboxRepository(client, integrationDB).Claim(ctx, worker, 1, time.Minute)
			if err == nil && len(events) != 1 {
				err = fmt.Errorf("expected one claimed event, got %d", len(events))
			}
			var event service.UsagePersistenceEvent
			if len(events) == 1 {
				event = events[0]
			}
			results <- claimed{event: event, worker: worker, err: err}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(results)
	ids := map[int64]bool{}
	for result := range results {
		require.NoError(t, result.err)
		require.False(t, ids[result.event.ID])
		ids[result.event.ID] = true
		require.NoError(t, NewUsagePersistenceOutboxRepository(client, integrationDB).Deliver(ctx, result.event.ID, result.worker))
	}
	require.Len(t, ids, 2)
	for _, f := range []usagePersistenceFixture{first, second} {
		balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
		require.Equal(t, 8.0, balance)
		require.Equal(t, 2.0, quota)
		require.Equal(t, 1, usage)
		require.Equal(t, 1, outcome)
		require.Equal(t, 1, dedup)
		require.Zero(t, pending)
	}
}

func TestUsagePersistenceOutbox_DedupReplayNeverRebillsOrOverwritesHistory(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	billing := NewUsageBillingRepository(client, integrationDB)
	_, err := billing.Apply(ctx, f.cmd)
	require.NoError(t, err)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "initial-writer")
	require.NoError(t, repo.Deliver(ctx, event.ID, "initial-writer"))
	f.cmd.UsageLog.OutputTokens = 999
	f.cmd.UsageLog.ActualCost = 99
	f.cmd.UsageLog.Outcome.HTTPStatus = 500
	replay, err := billing.Apply(ctx, f.cmd)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	var output, status int
	var cost float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT ul.output_tokens,ul.actual_cost,uro.http_status FROM usage_logs ul JOIN usage_response_outcomes uro ON uro.usage_log_id=ul.id WHERE ul.api_key_id=$1 AND ul.request_id=$2", f.keyID, f.requestID).Scan(&output, &cost, &status))
	require.Equal(t, 2, output)
	require.Equal(t, 2.0, cost)
	require.Equal(t, 200, status)
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)
}

func TestUsagePersistenceOutbox_GroupRollupLockDoesNotBlockSettlementAndRetriesDelivery(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: "durable-rollup-lock-" + uuid.NewString(), Platform: service.PlatformGemini})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id=$1", group.ID)
		require.NoError(t, err)
	})
	f := newUsagePersistenceFixture(t, client)
	f.cmd.UsageLog.GroupID = &group.ID
	var definition string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT pg_get_functiondef('invalidate_group_usage_rollup_state_after_insert'::regproc)").Scan(&definition))
	require.Contains(t, definition, "FOR KEY SHARE", "exercise the real migration 223 INSERT trigger, not an injected substitute")
	lock, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback() }()
	var singleton int
	require.NoError(t, lock.QueryRowContext(ctx, "SELECT id FROM usage_group_rollup_state WHERE id=1 FOR UPDATE").Scan(&singleton))
	require.Equal(t, 1, singleton)

	// Settlement never INSERTs usage_logs, so the rollup trigger's singleton
	// lock cannot hold a wallet, quota, or billing-dedup transaction open.
	settlementCtx, cancelSettlement := context.WithTimeout(ctx, 2*time.Second)
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(settlementCtx, f.cmd)
	cancelSettlement()
	require.NoError(t, err)
	require.True(t, result.Applied)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "blocked-rollup-writer")
	deliveryCtx, cancelDelivery := context.WithTimeout(ctx, 150*time.Millisecond)
	err = repo.Deliver(deliveryCtx, event.ID, "blocked-rollup-writer")
	require.Error(t, err, "actual usage INSERT must wait for the singleton lock")
	require.ErrorIs(t, deliveryCtx.Err(), context.DeadlineExceeded)
	cancelDelivery()
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Zero(t, usage)
	require.Zero(t, outcome)
	require.Equal(t, 1, dedup)
	require.Equal(t, 1, pending)

	require.NoError(t, lock.Rollback())
	require.NoError(t, repo.RetryClaimed(ctx, event.ID, "blocked-rollup-writer", time.Now().Add(-time.Second), "usage_persistence_delivery_timeout"))
	recovered := claimUsagePersistenceFixture(t, repo, "recovered-rollup-writer")
	require.Equal(t, event.ID, recovered.ID)
	require.Equal(t, 1, recovered.Attempts)
	require.NoError(t, repo.Deliver(ctx, event.ID, "recovered-rollup-writer"))
	balance, quota, usage, outcome, dedup, pending = usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)
}
