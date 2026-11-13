//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

type categoryDefaultUnlinkFixture struct {
	svc      *service.CategoryDefaultGroupService
	entryID  int64
	sources  []int64
	accounts []int64
	userID   int64
	keyID    int64
	key      string
}

func newCategoryDefaultUnlinkFixture(t *testing.T) categoryDefaultUnlinkFixture {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	f := categoryDefaultUnlinkFixture{svc: service.NewCategoryDefaultGroupService(NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB), nil)}
	for i := range 2 {
		group := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("unlink-source-%d-%d", suffix, i), Platform: service.PlatformGemini})
		account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("unlink-account-%d-%d", suffix, i), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{fmt.Sprintf("model-%d", i): "gemini-2.5-flash"}}})
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, group.ID)
		require.NoError(t, err)
		f.sources = append(f.sources, group.ID)
		f.accounts = append(f.accounts, account.ID)
	}
	preview, err := f.svc.Preview(ctx, "gemini", f.sources)
	require.NoError(t, err)
	config, err := f.svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: f.sources, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	f.entryID = config.GroupID
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("unlink-%d@example.com", suffix), Balance: 100})
	f.userID = user.ID
	f.key = fmt.Sprintf("unlink-test-key-%d", suffix)
	key, err := integrationEntClient.APIKey.Create().SetUserID(user.ID).SetName("default unlink regression").SetKey(f.key).SetGroupID(f.entryID).Save(ctx)
	require.NoError(t, err)
	f.keyID = key.ID
	t.Cleanup(func() {
		for _, query := range []string{`DELETE FROM category_default_group_models WHERE group_id=$1`, `DELETE FROM category_default_group_sources WHERE group_id=$1`, `DELETE FROM category_default_groups WHERE group_id=$1`} {
			_, cleanupErr := integrationDB.ExecContext(ctx, query, f.entryID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, f.userID)
		require.NoError(t, cleanupErr)
		for _, id := range f.accounts {
			_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, id)
			require.NoError(t, cleanupErr)
		}
		for _, id := range append(f.sources, f.entryID) {
			_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, id)
			require.NoError(t, cleanupErr)
		}
	})
	return f
}

func (f categoryDefaultUnlinkFixture) config(t *testing.T) service.CategoryDefaultGroupConfig {
	t.Helper()
	configs, err := f.svc.List(context.Background())
	require.NoError(t, err)
	for _, config := range configs {
		if config.GroupID == f.entryID {
			return config
		}
	}
	t.Fatal("category entry disappeared")
	return service.CategoryDefaultGroupConfig{}
}

func TestCategoryDefaultClosedSourceUnlinksAtomicallyAndStaysUnlinked(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	ctx := context.Background()
	original := f.config(t)
	preview, err := f.svc.Preview(ctx, "gemini", f.sources)
	require.NoError(t, err)
	var beforeInvalidations int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, f.key).Scan(&beforeInvalidations))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, f.sources[0])
	require.NoError(t, err)
	var links int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM category_default_group_sources WHERE group_id=$1 AND source_group_id=$2`, f.entryID, f.sources[0]).Scan(&links))
	require.Zero(t, links, "closing a source must unlink it in the same transaction")
	require.NoError(t, tx.Commit())
	config := f.config(t)
	require.Equal(t, []int64{f.sources[1]}, config.SourceGroupIDs)
	require.Greater(t, config.Revision, original.Revision)
	require.Empty(t, config.UnavailableSources)
	require.Equal(t, 1, config.ModelsCount)
	_, err = f.svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: f.sources, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.ErrorIs(t, err, service.ErrCategoryDefaultStalePreview)
	var afterInvalidations int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, f.key).Scan(&afterInvalidations))
	require.Greater(t, afterInvalidations, beforeInvalidations)
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, f.sources[0])
	require.NoError(t, err)
	require.Equal(t, []int64{f.sources[1]}, f.config(t).SourceGroupIDs, "re-enabling must not silently re-associate the source")
	var boundGroup int64
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT group_id FROM api_keys WHERE id=$1`, f.keyID).Scan(&boundGroup))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.userID).Scan(&balance))
	require.Equal(t, f.entryID, boundGroup)
	require.Equal(t, float64(100), balance)
	preview, err = f.svc.Preview(ctx, "gemini", f.sources)
	require.NoError(t, err)
	restored, err := f.svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: f.sources, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	require.Equal(t, f.sources, restored.SourceGroupIDs, "an administrator may explicitly link the restored source")
}

func TestCategoryDefaultClosingAllSourcesKeepsTheEntryAndKey(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE groups SET status='inactive' WHERE id IN ($1,$2)`, f.sources[0], f.sources[1])
	require.NoError(t, err)
	config := f.config(t)
	require.Equal(t, f.entryID, config.GroupID)
	require.True(t, config.Enabled)
	require.Empty(t, config.SourceGroupIDs)
	require.Zero(t, config.ModelsCount)
	var boundGroup int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `SELECT group_id FROM api_keys WHERE id=$1`, f.keyID).Scan(&boundGroup))
	require.Equal(t, f.entryID, boundGroup)
}

func TestCategoryDefaultUnlinkRollsBackAndKeepsTransientlyLimitedAccounts(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, f.sources[0])
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	require.Equal(t, f.sources, f.config(t).SourceGroupIDs)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET schedulable=FALSE WHERE id=$1`, f.accounts[0])
	require.NoError(t, err)
	require.Equal(t, f.sources, f.config(t).SourceGroupIDs, "account scheduling state must not remove a group association")
}

func TestCategoryDefaultUnlinkMigrationRepairsLegacyClosedLinksOnce(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	ctx := context.Background()
	original := f.config(t)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	// Reproduce a pre-migration database without changing triggers in other sessions.
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role=replica`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, f.sources[0])
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET deleted_at=NOW() WHERE id=$1`, f.sources[1])
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role=origin`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("263_category_default_unlink_closed_sources.sql")
	require.NoError(t, err)
	var revision int64
	for attempt := range 2 {
		_, err = tx.ExecContext(ctx, string(raw))
		require.NoError(t, err)
		var links, models int
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM category_default_group_sources WHERE group_id=$1`, f.entryID).Scan(&links))
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM category_default_group_models WHERE group_id=$1`, f.entryID).Scan(&models))
		require.Zero(t, links)
		require.Zero(t, models)
		var current int64
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT revision FROM category_default_groups WHERE group_id=$1`, f.entryID).Scan(&current))
		if attempt == 0 {
			require.Equal(t, original.Revision+1, current)
			revision = current
		} else {
			require.Equal(t, revision, current, "replaying the backfill must not touch an already cleaned entry")
		}
	}
}

func TestCategoryDefaultConcurrentSourceClosuresDoNotDeadlock(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range f.sources {
		go func(id int64) {
			<-start
			_, err := integrationDB.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, id)
			results <- err
		}(id)
	}
	close(start)
	for range 2 {
		require.NoError(t, <-results)
	}
	require.Empty(t, f.config(t).SourceGroupIDs)
}

func TestCategoryDefaultDeletedSourcesUnlink(t *testing.T) {
	for _, kind := range []string{"soft", "hard"} {
		t.Run(kind, func(t *testing.T) {
			f := newCategoryDefaultUnlinkFixture(t)
			query := `UPDATE groups SET deleted_at=NOW() WHERE id=$1`
			if kind == "hard" {
				query = `DELETE FROM groups WHERE id=$1`
			}
			_, err := integrationDB.ExecContext(context.Background(), query, f.sources[0])
			require.NoError(t, err)
			config := f.config(t)
			require.Equal(t, []int64{f.sources[1]}, config.SourceGroupIDs)
			require.Equal(t, 1, config.ModelsCount)
		})
	}
}

func TestCategoryDefaultCascadeDeleteAfterDeclarationWriteDoesNotDeadlock(t *testing.T) {
	f := newCategoryDefaultUnlinkFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	first, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = first.Rollback() }()
	_, err = first.Client().ExecContext(ctx, `DELETE FROM account_groups WHERE group_id=$1`, f.sources[0])
	require.NoError(t, err)
	second, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = second.Rollback() }()
	var secondPID int
	require.NoError(t, scanSingleRow(ctx, second.Client(), `SELECT pg_backend_pid()`, nil, &secondPID))
	result := make(chan error, 1)
	go func() {
		_, deleteErr := NewGroupRepository(second.Client(), integrationDB).DeleteCascade(ctx, f.sources[1])
		if deleteErr == nil {
			deleteErr = second.Commit()
		} else {
			_ = second.Rollback()
		}
		result <- deleteErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		queryErr := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, secondPID).Scan(&waiting)
		return queryErr == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	_, err = NewGroupRepository(first.Client(), integrationDB).DeleteCascade(ctx, f.sources[0])
	require.NoError(t, err, "a declaration write followed by a cascade delete must not upgrade competing shared locks")
	require.NoError(t, first.Commit())
	require.NoError(t, <-result)
	require.Empty(t, f.config(t).SourceGroupIDs)
}
