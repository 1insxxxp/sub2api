//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultHistoryExpiresSavePreviewWithoutBlockingLiveRestores(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	first := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("history-first-%d", suffix), Platform: service.PlatformGemini, RateMultiplier: 1})
	second := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("history-second-%d", suffix), Platform: service.PlatformGemini, RateMultiplier: 1})
	a := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("history-a-%d", suffix), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "history-secret-must-not-retain", "model_mapping": map[string]any{"foo": "gemini-2.5-flash", "bar": "gemini-2.5-pro"}}})
	b := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("history-b-%d", suffix), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"baz": "gemini-3-pro"}}})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2),($3,$4)`, a.ID, first.ID, b.ID, second.ID)
	require.NoError(t, err)
	var entryID int64
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM category_default_group_models WHERE group_id=$1`,
			`DELETE FROM category_default_group_sources WHERE group_id=$1`,
			`DELETE FROM category_default_groups WHERE group_id=$1`,
		} {
			_, cleanupErr := integrationDB.ExecContext(ctx, statement, entryID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id IN ($1,$2)`, a.ID, b.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, a.ID, b.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2,$3)`, entryID, first.ID, second.ID)
		require.NoError(t, cleanupErr)
	})
	repo := NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB)
	svc := service.NewCategoryDefaultGroupService(repo, nil)
	ids := []int64{first.ID, second.ID}
	publish := func() {
		t.Helper()
		preview, previewErr := svc.Preview(ctx, "gemini", ids)
		require.NoError(t, previewErr)
		config, applyErr := svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
		require.NoError(t, applyErr)
		entryID = config.GroupID
		require.Empty(t, config.InvalidatedModels)
		var obsoleteHistory int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM category_default_declaration_history h JOIN category_default_groups c ON c.group_id=h.entry_group_id WHERE h.entry_group_id=$1 AND h.publication_revision<c.publication_revision`, entryID).Scan(&obsoleteHistory))
		require.Zero(t, obsoleteHistory, "confirmed publication must not retain obsolete declaration snapshots indefinitely")
	}
	user := &service.User{ID: 10, Status: service.StatusActive}
	available := func(model string) {
		t.Helper()
		_, resolveErr := svc.ResolveForUser(ctx, entryID, user, model)
		require.NoError(t, resolveErr)
	}
	updateFoo := func(target string) {
		t.Helper()
		_, updateErr := integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,foo}',to_jsonb($1::text)) WHERE id=$2`, target, a.ID)
		require.NoError(t, updateErr)
	}
	publish()
	available("foo")
	// No runtime read occurs while B is configured. History retains B for save validation, while live routes follow restored A.
	updateFoo("gemini-3-pro")
	updateFoo("gemini-2.5-flash")
	available("foo")
	available("bar")
	available("baz")
	config, err := repo.GetByGroupID(ctx, entryID)
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, config.InvalidatedModels)
	preview, err := svc.Preview(ctx, "gemini", ids)
	require.NoError(t, err)
	require.Empty(t, preview.Modified, "restored declarations are already current")
	require.Len(t, preview.Unchanged, 3)
	// Another unseen B -> A invalidates an earlier preview even when the
	// blocked model set and current fingerprint have not changed.
	updateFoo("gemini-3.1-pro")
	updateFoo("gemini-2.5-flash")
	_, err = svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.ErrorIs(t, err, service.ErrCategoryDefaultStalePreview)
	_, err = svc.SetStatus(ctx, "gemini", false)
	require.NoError(t, err)
	_, err = svc.SetStatus(ctx, "gemini", true)
	require.NoError(t, err)
	available("foo")
	publish()
	available("foo")
	// Closing unlinks the source; re-enabling requires an explicit association.
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = svc.ResolveForUser(ctx, entryID, user, "foo")
	require.ErrorIs(t, err, service.ErrCategoryDefaultModelNotAllowed)
	_, err = svc.ResolveForUser(ctx, entryID, user, "bar")
	require.ErrorIs(t, err, service.ErrCategoryDefaultModelNotAllowed)
	available("baz")
	publish()
	available("foo")
	available("bar")
	// A duplicate that has been removed no longer quarantines the restored name.
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,foo}','"gemini-3-pro"'::jsonb) WHERE id=$1`, b.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=credentials #- '{model_mapping,foo}' WHERE id=$1`, b.ID)
	require.NoError(t, err)
	available("foo")
	available("bar")
	available("baz")
	publish()
	// An atomic transaction ending at its original declaration is not a
	// separately committed source state and must not cause a false quarantine.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,foo}','"intermediate-only"'::jsonb) WHERE id=$1`, a.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,foo}','"gemini-2.5-flash"'::jsonb) WHERE id=$1`, a.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	available("foo")
	var retained bool
	err = integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM category_default_declaration_history WHERE entry_group_id=$1 AND snapshot::text LIKE '%history-secret-must-not-retain%')`, entryID).Scan(&retained)
	require.NoError(t, err)
	require.False(t, retained)
	// A source edit takes its group row lock before the database declaration
	// trigger waits on the publication row. Publishing must not then request a
	// source group row lock in the opposite order and abort this administrator.
	raceCtx, cancelRace := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRace()
	preview, err = svc.Preview(raceCtx, "gemini", ids)
	require.NoError(t, err)
	gate, err := integrationDB.BeginTx(raceCtx, nil)
	require.NoError(t, err)
	defer func() { _ = gate.Rollback() }()
	var gatePID int
	require.NoError(t, gate.QueryRowContext(raceCtx, `SELECT pg_backend_pid()`).Scan(&gatePID))
	// Block the publication's manifest read after it acquired the config row.
	// This avoids depending on the implementation's advisory-lock ordering.
	_, err = gate.ExecContext(raceCtx, `LOCK TABLE category_default_group_models IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	published := make(chan error, 1)
	go func() {
		_, publishErr := svc.Apply(raceCtx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
		published <- publishErr
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		queryErr := integrationDB.QueryRowContext(raceCtx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, gatePID).Scan(&blocked)
		return queryErr == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "publisher must hold its config lock before the source edit starts")
	writer, err := integrationDB.BeginTx(raceCtx, nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	var writerPID int
	require.NoError(t, writer.QueryRowContext(raceCtx, `SELECT pg_backend_pid()`).Scan(&writerPID))
	written := make(chan error, 1)
	go func() {
		_, updateErr := writer.ExecContext(raceCtx, `UPDATE groups SET status='inactive' WHERE id=$1`, first.ID)
		if updateErr == nil {
			updateErr = writer.Commit()
		} else {
			_ = writer.Rollback()
		}
		written <- updateErr
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		queryErr := integrationDB.QueryRowContext(raceCtx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, writerPID).Scan(&blocked)
		return queryErr == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "source writer must hold its group row while waiting for publication")
	require.NoError(t, gate.Commit())
	publishErr := <-published
	if publishErr != nil {
		require.ErrorIs(t, publishErr, service.ErrCategoryDefaultStalePreview, "a concurrent publication may ask for a refreshed preview")
	}
	require.NoError(t, <-written, "a concurrent source edit must not be a deadlock victim")
}

func TestCategoryDefaultGrokSettingHistoryIsFrozenAndLiveRestoresRecover(t *testing.T) {
	ctx := context.Background()
	initialRuntime := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(initialRuntime) })
	prior := make(map[string]string)
	rows, err := integrationDB.QueryContext(ctx, `SELECT key,value FROM settings WHERE key IN ('grok_default_text_model','grok_cross_client_model_map_enabled')`)
	require.NoError(t, err)
	for rows.Next() {
		var key, value string
		require.NoError(t, rows.Scan(&key, &value))
		prior[key] = value
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	set := func(key, value string) {
		t.Helper()
		_, setErr := integrationDB.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, key, value)
		require.NoError(t, setErr)
	}
	t.Cleanup(func() {
		for _, key := range []string{"grok_default_text_model", "grok_cross_client_model_map_enabled"} {
			if value, ok := prior[key]; ok {
				set(key, value)
			} else {
				_, cleanupErr := integrationDB.ExecContext(ctx, `DELETE FROM settings WHERE key=$1`, key)
				require.NoError(t, cleanupErr)
			}
		}
	})
	set("grok_default_text_model", "grok-4.6")
	set("grok_cross_client_model_map_enabled", "true")
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{DefaultText: "grok-4.6", EnableCrossClientMap: true})
	suffix := time.Now().UnixNano()
	source := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("grok-history-%d", suffix), Platform: service.PlatformGrok,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"grok", "grok-4.7", "gpt-test"}}})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("grok-history-account-%d", suffix), Platform: service.PlatformGrok, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "must-not-retain-grok-secret"}})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, source.ID)
	require.NoError(t, err)
	var entryID int64
	t.Cleanup(func() {
		for _, statement := range []string{`DELETE FROM category_default_group_models WHERE group_id=$1`, `DELETE FROM category_default_group_sources WHERE group_id=$1`, `DELETE FROM category_default_groups WHERE group_id=$1`} {
			_, cleanupErr := integrationDB.ExecContext(ctx, statement, entryID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, source.ID, entryID)
		require.NoError(t, cleanupErr)
	})
	repo := NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB)
	svc := service.NewCategoryDefaultGroupService(repo, nil)
	user := &service.User{ID: 10, Status: service.StatusActive}
	publish := func() {
		t.Helper()
		preview, previewErr := svc.Preview(ctx, "grok", []int64{source.ID})
		require.NoError(t, previewErr)
		config, publishErr := svc.Apply(ctx, "grok", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{source.ID}, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
		require.NoError(t, publishErr)
		entryID = config.GroupID
	}
	publish()
	set("grok_default_text_model", "grok-4.3")
	set("grok_default_text_model", "grok-4.6")
	_, err = svc.ResolveForUser(ctx, entryID, user, "grok")
	require.NoError(t, err, "restored settings must not wait for publication")
	_, err = svc.ResolveForUser(ctx, entryID, user, "grok-4.7")
	require.NoError(t, err, "literal model mapping is unaffected by a default-target change")
	var frozenB bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM category_default_declaration_history WHERE entry_group_id=$1 AND snapshot->'grok_mapping_options'->>'DefaultText'='grok-4.3')`, entryID).Scan(&frozenB))
	require.True(t, frozenB)
	publish()
	set("grok_cross_client_model_map_enabled", "false")
	set("grok_cross_client_model_map_enabled", "true")
	_, err = svc.ResolveForUser(ctx, entryID, user, "gpt-test")
	require.NoError(t, err)
	_, err = svc.ResolveForUser(ctx, entryID, user, "grok")
	require.NoError(t, err)
	publish()
	// A node with stale process-wide settings cannot execute a newly confirmed
	// DB declaration through a different target until that node has refreshed.
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{DefaultText: "grok-4.3", EnableCrossClientMap: true})
	_, err = svc.ResolveForUser(ctx, entryID, user, "grok")
	require.ErrorIs(t, err, service.ErrCategoryDefaultPendingSync)
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{DefaultText: "grok-4.6", EnableCrossClientMap: true})
	_, err = svc.ResolveForUser(ctx, entryID, user, "grok")
	require.NoError(t, err)
}
