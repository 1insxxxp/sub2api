//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func workbenchUser(t *testing.T, role string) *service.User {
	t.Helper()
	u := supportUser(t, role, 0)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM sub_admin_workbench_access WHERE user_id=$1 OR updated_by=$1", u.ID)
	})
	return u
}

func TestAdminWorkbenchAccessRolesDefaultsAndRevocation(t *testing.T) {
	ctx := context.Background()
	admin := workbenchUser(t, service.RoleAdmin)
	staff := workbenchUser(t, service.RoleSubAdmin)
	ordinary := workbenchUser(t, service.RoleUser)
	r := NewAdminWorkbenchAccessRepository(integrationDB)
	all := service.AdminWorkbenchAccess{Commission: true, AffiliateLeaderboard: true, UserSupport: true, BalanceTransfer: true}
	got, err := r.GetAccess(ctx, admin.ID)
	require.NoError(t, err)
	require.Equal(t, all, *got)
	got, err = r.GetUserAccess(ctx, admin.ID, admin.ID)
	require.NoError(t, err)
	require.Equal(t, all, *got)
	got, err = r.GetAccess(ctx, staff.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{BalanceTransfer: true}, *got)
	_, err = r.GetUserAccess(ctx, staff.ID, admin.ID)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	require.ErrorIs(t, r.SetUserAccess(ctx, admin.ID, admin.ID, false, false, false), service.ErrUserSupportForbidden)
	require.ErrorIs(t, r.SetUserAccess(ctx, staff.ID, staff.ID, true, true, true), service.ErrUserSupportForbidden)

	require.NoError(t, r.SetUserAccess(ctx, admin.ID, staff.ID, true, false, true))
	got, err = r.GetUserAccess(ctx, admin.ID, staff.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{Commission: true, UserSupport: true, BalanceTransfer: true}, *got)
	var actor int64
	require.NoError(t, integrationDB.QueryRow("SELECT updated_by FROM sub_admin_workbench_access WHERE user_id=$1", staff.ID).Scan(&actor))
	require.Equal(t, admin.ID, actor)
	require.NoError(t, integrationDB.QueryRow("SELECT updated_by FROM user_support_access WHERE user_id=$1", staff.ID).Scan(&actor))
	require.Equal(t, admin.ID, actor)

	// Normal users cannot inherit capabilities from stale or erroneous rows.
	_, err = integrationDB.Exec("INSERT INTO sub_admin_workbench_access(user_id,commission_enabled,affiliate_leaderboard_enabled,updated_by) VALUES($1,TRUE,TRUE,$2)", ordinary.ID, admin.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec("INSERT INTO user_support_access(user_id,enabled,updated_by) VALUES($1,TRUE,$2)", ordinary.ID, admin.ID)
	require.NoError(t, err)
	_, err = r.GetAccess(ctx, ordinary.ID)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	_, err = r.GetUserAccess(ctx, admin.ID, ordinary.ID)
	require.ErrorIs(t, err, service.ErrUserSupportUserNotFound)
	require.ErrorIs(t, r.SetUserAccess(ctx, admin.ID, ordinary.ID, true, true, true), service.ErrUserSupportUserNotFound)

	// Capability decisions are fresh, including role changes after earlier success.
	_, err = integrationDB.Exec("UPDATE users SET role='user' WHERE id=$1", staff.ID)
	require.NoError(t, err)
	_, err = r.GetAccess(ctx, staff.ID)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	_, err = integrationDB.Exec("UPDATE users SET status='inactive' WHERE id=$1", admin.ID)
	require.NoError(t, err)
	_, err = r.GetUserAccess(ctx, admin.ID, ordinary.ID)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
}

func TestAdminWorkbenchAccessThreeFlagsSaveAtomically(t *testing.T) {
	ctx := context.Background()
	admin := workbenchUser(t, service.RoleAdmin)
	secondAdmin := workbenchUser(t, service.RoleAdmin)
	staff := workbenchUser(t, service.RoleSubAdmin)
	r := NewAdminWorkbenchAccessRepository(integrationDB)
	require.NoError(t, r.SetUserAccess(ctx, admin.ID, staff.ID, true, false, true))
	name := "workbench_access_fail_" + strconv.FormatInt(staff.ID, 10)
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger AS $$ BEGIN IF NEW.user_id=%d THEN RAISE EXCEPTION 'private test diagnostic'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql; CREATE TRIGGER %s BEFORE INSERT OR UPDATE ON user_support_access FOR EACH ROW EXECUTE FUNCTION %s()`, name, staff.ID, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON user_support_access; DROP FUNCTION IF EXISTS %s()", name, name))
	})
	require.Error(t, r.SetUserAccess(ctx, secondAdmin.ID, staff.ID, false, true, false))
	got, err := r.GetAccess(ctx, staff.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{Commission: true, UserSupport: true, BalanceTransfer: true}, *got)
	var actor int64
	require.NoError(t, integrationDB.QueryRow("SELECT updated_by FROM sub_admin_workbench_access WHERE user_id=$1", staff.ID).Scan(&actor))
	require.Equal(t, admin.ID, actor)
	require.NoError(t, integrationDB.QueryRow("SELECT updated_by FROM user_support_access WHERE user_id=$1", staff.ID).Scan(&actor))
	require.Equal(t, admin.ID, actor)
}

func TestAdminWorkbenchAccessRechecksAdminAfterTargetLock(t *testing.T) {
	ctx := context.Background()
	admin := workbenchUser(t, service.RoleAdmin)
	staff := workbenchUser(t, service.RoleSubAdmin)
	r := NewAdminWorkbenchAccessRepository(integrationDB)
	lock, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback() }()
	_, err = lock.ExecContext(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", staff.ID)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() { result <- r.SetUserAccess(ctx, admin.ID, staff.ID, true, true, true) }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT role,status FROM users WHERE id=%')").Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	_, err = integrationDB.Exec("UPDATE users SET role='user' WHERE id=$1", admin.ID)
	require.NoError(t, err)
	require.NoError(t, lock.Commit())
	select {
	case err = <-result:
		require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	case <-time.After(3 * time.Second):
		t.Fatal("permission update remained blocked")
	}
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM sub_admin_workbench_access WHERE user_id=$1", staff.ID).Scan(&count))
	require.Zero(t, count)
}

func TestAdminWorkbenchAccessMigrationBackfillsExistingSubAdminsOnly(t *testing.T) {
	ctx := context.Background()
	admin := workbenchUser(t, service.RoleAdmin)
	existing := workbenchUser(t, service.RoleSubAdmin)
	ordinary := workbenchUser(t, service.RoleUser)
	data, err := migrations.FS.ReadFile("256_admin_workbench_access.sql")
	require.NoError(t, err)
	apply := func() {
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(ctx, string(data))
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	apply()
	r := NewAdminWorkbenchAccessRepository(integrationDB)
	got, err := r.GetAccess(ctx, existing.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{Commission: true, AffiliateLeaderboard: true, BalanceTransfer: true}, *got)
	var snapshot bool
	require.NoError(t, integrationDB.QueryRow("SELECT updated_by IS NULL FROM sub_admin_workbench_access WHERE user_id=$1", existing.ID).Scan(&snapshot))
	require.True(t, snapshot)
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM sub_admin_workbench_access WHERE user_id IN ($1,$2)", admin.ID, ordinary.ID).Scan(&count))
	require.Zero(t, count)
	newStaff := workbenchUser(t, service.RoleSubAdmin)
	got, err = r.GetAccess(ctx, newStaff.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{BalanceTransfer: true}, *got)
	require.NoError(t, r.SetUserAccess(ctx, admin.ID, existing.ID, false, true, true))
	apply()
	got, err = r.GetAccess(ctx, existing.ID)
	require.NoError(t, err)
	require.Equal(t, service.AdminWorkbenchAccess{AffiliateLeaderboard: true, UserSupport: true, BalanceTransfer: true}, *got)
}
