//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupAccountUserExclusionsRealPostgresAuthorizationAndOutbox(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("exclude-g-%d", suffix)})
	otherGroup := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("exclude-other-g-%d", suffix)})
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("exclude-%d@example.com", suffix), Username: "Customer", Notes: "Private administrator note"})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("exclude-a-%d", suffix)})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups (account_id,group_id) VALUES ($1,$2),($1,$3)`, account.ID, group.ID, otherGroup.ID)
	require.NoError(t, err)
	keys := NewAPIKeyRepository(integrationEntClient, integrationDB)
	keyValue := fmt.Sprintf("sk-exclude-%d", suffix)
	key := &service.APIKey{UserID: user.ID, GroupID: &otherGroup.ID, Key: keyValue, Name: "cross-group-auth", Status: service.StatusActive}
	require.NoError(t, keys.Create(ctx, key))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=$1`, key.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM group_account_user_exclusions WHERE user_id=$1`, user.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1`, account.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, group.ID, otherGroup.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, keyValue)
		require.NoError(t, err)
	})
	repo := NewGroupAccountUserExclusionRepository(integrationDB)
	item, err := repo.Add(ctx, group.ID, account.ID, user.ID)
	require.NoError(t, err)
	duplicate, err := repo.Add(ctx, group.ID, account.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, item.ID, duplicate.ID)
	require.Equal(t, item.CreatedAt, duplicate.CreatedAt)
	items, page, err := repo.List(ctx, group.ID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, items, 1)
	require.Equal(t, user.Notes, items[0].Notes)
	require.Equal(t, user.Username, items[0].Username)
	require.Equal(t, account.Name, items[0].AccountName)
	var outbox int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, keyValue).Scan(&outbox))
	require.GreaterOrEqual(t, outbox, 2, "all user keys must invalidate even when bound to another group")
	for _, lookup := range []func() (*service.APIKey, error){func() (*service.APIKey, error) { return keys.GetByID(ctx, key.ID) }, func() (*service.APIKey, error) { return keys.GetByKey(ctx, keyValue) }, func() (*service.APIKey, error) { return keys.GetByKeyForAuth(ctx, keyValue) }} {
		got, err := lookup()
		require.NoError(t, err)
		require.Equal(t, service.GroupAccountExclusions{group.ID: {account.ID}}, got.User.GroupAccountExclusions)
	}
	_, err = repo.Remove(ctx, otherGroup.ID, item.ID)
	require.ErrorIs(t, err, service.ErrGroupAccountUserExclusionNotFound)
	_, err = repo.Remove(ctx, group.ID, item.ID)
	require.NoError(t, err)
	got, err := keys.GetByKeyForAuth(ctx, keyValue)
	require.NoError(t, err)
	require.Empty(t, got.User.GroupAccountExclusions)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, user.ID)
	require.NoError(t, err)
	_, err = repo.Add(ctx, group.ID, account.ID, user.ID)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET status='active' WHERE id=$1`, user.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1 AND group_id=$2`, account.ID, group.ID)
	require.NoError(t, err)
	_, err = repo.Add(ctx, group.ID, account.ID, user.ID)
	require.ErrorIs(t, err, service.ErrAccountNotFound)
}
