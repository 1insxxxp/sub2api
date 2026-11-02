//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type groupCascadeFixture struct {
	owner, other, source, kept, mixed, emptied, unrelated, deletedParent, key, account int64
}

func newGroupCascadeFixture(t *testing.T) groupCascadeFixture {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	var f groupCascadeFixture
	insert := func(query string, args ...any) int64 {
		t.Helper()
		var id int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, query, args...).Scan(&id))
		return id
	}
	f.owner = insert("INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',7) RETURNING id", "cascade-owner-"+suffix+"@test.local")
	f.other = insert("INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',11) RETURNING id", "cascade-other-"+suffix+"@test.local")
	f.source = insert("INSERT INTO groups(name,platform,status) VALUES($1,'openai','active') RETURNING id", "cascade-source-"+suffix)
	f.kept = insert("INSERT INTO groups(name,platform,status) VALUES($1,'openai','active') RETURNING id", "cascade-kept-"+suffix)
	f.mixed = insert("INSERT INTO user_custom_groups(user_id,name) VALUES($1,$2) RETURNING id", f.owner, "mixed-"+suffix)
	f.emptied = insert("INSERT INTO user_custom_groups(user_id,name) VALUES($1,$2) RETURNING id", f.owner, "emptied-"+suffix)
	f.unrelated = insert("INSERT INTO user_custom_groups(user_id,name) VALUES($1,$2) RETURNING id", f.other, "unrelated-"+suffix)
	f.deletedParent = insert("INSERT INTO user_custom_groups(user_id,name,deleted_at) VALUES($1,$2,NOW()) RETURNING id", f.other, "deleted-"+suffix)
	for _, row := range []struct {
		parent, source int64
		name           string
	}{{f.mixed, f.source, "removed"}, {f.mixed, f.kept, "kept"}, {f.emptied, f.source, "removed"}, {f.unrelated, f.kept, "unrelated"}, {f.deletedParent, f.source, "deleted-parent"}} {
		_, err := integrationDB.ExecContext(ctx, "INSERT INTO user_custom_group_models(custom_group_id,public_model,source_group_id,source_model) VALUES($1,$2,$3,$2)", row.parent, row.name, row.source)
		require.NoError(t, err)
	}
	f.key = insert("INSERT INTO api_keys(user_id,key,name,custom_group_id) VALUES($1,$2,'cascade-fixture',$3) RETURNING id", f.owner, "sk-cascade-"+suffix, f.mixed)
	insert("INSERT INTO api_keys(user_id,key,name,custom_group_id) VALUES($1,$2,'empty-parent-fixture',$3) RETURNING id", f.owner, "sk-empty-cascade-"+suffix, f.emptied)
	f.account = insert("INSERT INTO accounts(name,platform,type) VALUES($1,'openai','apikey') RETURNING id", "cascade-account-"+suffix)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,group_id,source_group_id,custom_group_id,actual_cost)
		VALUES($1,$2,$3,$4,'removed',$5,$5,$6,0.25)`, f.owner, f.key, f.account, "cascade-history-"+suffix, f.source, f.mixed)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id IN($1,$2)", f.owner, f.other)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM user_subscriptions WHERE user_id IN($1,$2)", f.owner, f.other)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id IN($1,$2)", f.owner, f.other)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM user_custom_group_models WHERE custom_group_id IN(SELECT id FROM user_custom_groups WHERE user_id IN($1,$2))", f.owner, f.other)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM user_custom_groups WHERE user_id IN($1,$2)", f.owner, f.other)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN($1,$2)", f.source, f.kept)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", f.account)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id IN($1,$2)", f.owner, f.other)
	})
	return f
}

func TestGroupDeleteCascadeRemovesOnlySourceModelReferences(t *testing.T) {
	for _, guarded := range []bool{false, true} {
		t.Run(fmt.Sprintf("guarded=%t", guarded), func(t *testing.T) {
			f := newGroupCascadeFixture(t)
			ctx := context.Background()
			repo := newGroupRepositoryWithSQL(testEntClient(t), integrationDB)
			var affected []int64
			var err error
			if guarded {
				affected, err = repo.DeleteCascadeIfEmpty(ctx, f.source)
			} else {
				affected, err = repo.DeleteCascade(ctx, f.source)
			}
			require.NoError(t, err)
			require.Equal(t, []int64{f.owner}, affected, "one owner with two affected custom groups is invalidated once; deleted parents do not add owners")
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_group_models WHERE source_group_id=$1", f.source).Scan(&count))
			require.Zero(t, count, "also remove stale references under already-deleted custom groups")
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_group_models WHERE source_group_id=$1", f.kept).Scan(&count))
			require.Equal(t, 2, count)
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_groups WHERE user_id IN($1,$2) AND deleted_at IS NULL", f.owner, f.other).Scan(&count))
			require.Equal(t, 3, count, "preserve the mixed, empty and unrelated parent groups")
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM api_keys WHERE user_id=$1 AND custom_group_id IN($2,$3) AND deleted_at IS NULL", f.owner, f.mixed, f.emptied).Scan(&count))
			require.Equal(t, 2, count, "custom-bound keys must not fall back to unrestricted/default routing")
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM usage_logs WHERE user_id=$1 AND group_id=$2 AND actual_cost=0.25", f.owner, f.source).Scan(&count))
			require.Equal(t, 1, count, "historical accounting must remain unchanged")
			customRepo := &userCustomGroupRepository{client: testEntClient(t), db: integrationDB}
			_, err = customRepo.ResolveModel(ctx, f.mixed, f.owner, "removed")
			require.ErrorIs(t, err, service.ErrUserCustomGroupInvalidModel)
			_, err = customRepo.ResolveModel(ctx, f.emptied, f.owner, "removed")
			require.ErrorIs(t, err, service.ErrUserCustomGroupInvalidModel)
			kept, err := customRepo.ResolveModel(ctx, f.mixed, f.owner, "kept")
			require.NoError(t, err)
			require.Equal(t, f.kept, kept.SourceGroupID)
			_, err = repo.GetByIDLite(ctx, f.source)
			require.ErrorIs(t, err, service.ErrGroupNotFound)
		})
	}
}

func TestGroupDeleteCascadeIncludesSubscriptionAndCustomModelOwners(t *testing.T) {
	f := newGroupCascadeFixture(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "UPDATE groups SET subscription_type='subscription' WHERE id=$1", f.source)
	require.NoError(t, err)
	for _, owner := range []int64{f.owner, f.other} {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at)
			VALUES($1,$2,NOW(),NOW()+INTERVAL '1 day')`, owner, f.source)
		require.NoError(t, err)
	}
	repo := newGroupRepositoryWithSQL(testEntClient(t), integrationDB)
	affected, err := repo.DeleteCascade(ctx, f.source)
	require.NoError(t, err)
	require.Equal(t, []int64{f.owner, f.other}, affected, "shared model/subscription owners must be returned once")
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_subscriptions WHERE group_id=$1 AND deleted_at IS NOT NULL", f.source).Scan(&count))
	require.Equal(t, 2, count, "preserve the existing subscription soft-delete behavior")
}

func TestGroupDeleteCascadeRollsBackRemovedModelReferences(t *testing.T) {
	f := newGroupCascadeFixture(t)
	ctx := context.Background()
	name := fmt.Sprintf("fail_group_cascade_%d", f.source)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger AS $$ BEGIN
		IF NEW.id=%d AND NEW.deleted_at IS NOT NULL THEN RAISE EXCEPTION 'forced group cascade failure'; END IF;
		RETURN NEW; END $$ LANGUAGE plpgsql;
		CREATE TRIGGER %s BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION %s()`, name, f.source, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+name+" ON groups; DROP FUNCTION IF EXISTS "+name+"()")
	})
	repo := newGroupRepositoryWithSQL(testEntClient(t), integrationDB)
	affected, err := repo.DeleteCascade(ctx, f.source)
	require.ErrorContains(t, err, "forced group cascade failure")
	require.Empty(t, affected)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_group_models WHERE source_group_id=$1", f.source).Scan(&count))
	require.Equal(t, 3, count)
	_, err = repo.GetByIDLite(ctx, f.source)
	require.NoError(t, err)
}

func TestCustomModelWriterWaitingBehindSourceDeleteCannotReinsert(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%t", update), func(t *testing.T) {
			f := newGroupCascadeFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			lock, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = lock.Rollback() }()
			var id int64
			var blockerPID int
			require.NoError(t, lock.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
			require.NoError(t, lock.QueryRowContext(ctx, "SELECT id FROM groups WHERE id=$1 FOR UPDATE", f.source).Scan(&id))
			repo := &userCustomGroupRepository{client: testEntClient(t), db: integrationDB}
			models := []service.UserCustomGroupModelInput{{PublicModel: "new-model", SourceGroupID: f.source, SourceModel: "new-model"}}
			finished := make(chan error, 1)
			go func() {
				if update {
					finished <- repo.Update(ctx, &service.UserCustomGroup{ID: f.mixed, UserID: f.owner, Name: "updated", Status: service.StatusActive}, &models)
				} else {
					finished <- repo.Create(ctx, &service.UserCustomGroup{UserID: f.owner, Name: "new-parent", Status: service.StatusActive}, models)
				}
			}()
			require.Eventually(t, func() bool {
				var blocked bool
				err := integrationDB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", blockerPID).Scan(&blocked)
				return err == nil && blocked
			}, 3*time.Second, 10*time.Millisecond)
			// Simulate the committed deletion while this writer waits at the
			// source row; a stale service-level validation must not suffice.
			_, err = lock.ExecContext(ctx, "DELETE FROM user_custom_group_models WHERE source_group_id=$1", f.source)
			require.NoError(t, err)
			_, err = lock.ExecContext(ctx, "UPDATE groups SET deleted_at=NOW() WHERE id=$1", f.source)
			require.NoError(t, err)
			require.NoError(t, lock.Commit())
			select {
			case err := <-finished:
				require.ErrorIs(t, err, service.ErrGroupNotFound)
			case <-ctx.Done():
				t.Fatal("writer did not finish after source deletion")
			}
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_group_models WHERE source_group_id=$1", f.source).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_groups WHERE user_id=$1 AND name IN('new-parent','updated')", f.owner).Scan(&count))
			require.Zero(t, count, "failed source validation must not create or rename the custom parent")
		})
	}
}

func TestGroupDeleteWaitingBehindCustomModelWriterRemovesCommittedReference(t *testing.T) {
	f := newGroupCascadeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	writer, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	var id int64
	var blockerPID int
	require.NoError(t, writer.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
	require.NoError(t, writer.QueryRowContext(ctx, "SELECT id FROM groups WHERE id=$1 FOR SHARE", f.source).Scan(&id))
	finished := make(chan error, 1)
	repo := newGroupRepositoryWithSQL(testEntClient(t), integrationDB)
	go func() { _, err := repo.DeleteCascade(ctx, f.source); finished <- err }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := integrationDB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", blockerPID).Scan(&blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
	_, err = writer.ExecContext(ctx, "INSERT INTO user_custom_group_models(custom_group_id,public_model,source_group_id,source_model) VALUES($1,'just-committed',$2,'model')", f.unrelated, f.source)
	require.NoError(t, err)
	require.NoError(t, writer.Commit())
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("deleter did not finish after custom-model writer committed")
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_custom_group_models WHERE source_group_id=$1", f.source).Scan(&count))
	require.Zero(t, count)
}
