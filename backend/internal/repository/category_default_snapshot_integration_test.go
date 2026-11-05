//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultSnapshotKeepsAssociationsAndDeclarationsInOneReadVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	suffix := time.Now().UnixNano()
	first := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("snapshot-first-%d", suffix), Platform: service.PlatformGemini})
	second := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("snapshot-second-%d", suffix), Platform: service.PlatformGemini})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("snapshot-account-%d", suffix), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"before": "gemini-2.5-flash"}}})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, first.ID)
	require.NoError(t, err)
	var entryID int64
	t.Cleanup(func() {
		for _, statement := range []string{`DELETE FROM category_default_group_models WHERE group_id=$1`, `DELETE FROM category_default_group_sources WHERE group_id=$1`, `DELETE FROM category_default_groups WHERE group_id=$1`} {
			_, cleanupErr := integrationDB.ExecContext(context.Background(), statement, entryID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id IN ($1,$2,$3)`, first.ID, second.ID, entryID)
		require.NoError(t, cleanupErr)
	})
	repo := NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB)
	svc := service.NewCategoryDefaultGroupService(repo, nil)
	preview, err := svc.Preview(ctx, "gemini", []int64{first.ID})
	require.NoError(t, err)
	config, err := svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{first.ID}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	entryID = config.GroupID
	driver := &categoryDefaultSnapshotDriver{Driver: integrationEntClient.Driver()}
	writes := 0
	driver.afterConfigRead = func() {
		writes++
		writer, beginErr := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, beginErr)
		defer func() { _ = writer.Rollback() }()
		_, writeErr := writer.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}','{"after":"gemini-2.5-pro"}'::jsonb) WHERE id=$1`, account.ID)
		require.NoError(t, writeErr)
		_, writeErr = writer.ExecContext(ctx, `DELETE FROM category_default_group_sources WHERE group_id=$1`, entryID)
		require.NoError(t, writeErr)
		_, writeErr = writer.ExecContext(ctx, `INSERT INTO category_default_group_sources(group_id,source_group_id) VALUES($1,$2)`, entryID, second.ID)
		require.NoError(t, writeErr)
		_, writeErr = writer.ExecContext(ctx, `UPDATE category_default_groups SET revision=revision+1 WHERE group_id=$1`, entryID)
		require.NoError(t, writeErr)
		require.NoError(t, writer.Commit())
	}
	consistentRepo := NewCategoryDefaultGroupRepository(dbent.NewClient(dbent.Driver(driver)), integrationDB)
	snapshot, err := consistentRepo.Snapshot(ctx, service.CategoryDefaultSnapshotRequest{GroupID: entryID})
	require.NoError(t, err)
	require.Equal(t, 1, writes)
	require.Equal(t, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, driver.options)
	require.Equal(t, config.Revision, snapshot.Config.Revision)
	require.Equal(t, []int64{first.ID}, snapshot.Config.SourceGroupIDs)
	require.Len(t, snapshot.Sources, 1)
	require.Equal(t, first.ID, snapshot.Sources[0].Group.ID)
	models, err := service.CategoryDefaultSourceDeclarations(snapshot.Sources[0])
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "before", models[0].PublicModel)
	next, err := repo.Snapshot(ctx, service.CategoryDefaultSnapshotRequest{GroupID: entryID})
	require.NoError(t, err)
	require.Equal(t, config.Revision+1, next.Config.Revision)
	require.Equal(t, []int64{second.ID}, next.Config.SourceGroupIDs)
}
