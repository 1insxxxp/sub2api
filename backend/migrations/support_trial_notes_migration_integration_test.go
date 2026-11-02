//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestSupportTrialNotesMigrationRepairsOnlyProvenGrants(t *testing.T) {
	raw, err := FS.ReadFile("259_support_trial_notes.sql")
	require.NoError(t, err)
	db := supportTrialNotesPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
SET LOCAL search_path TO pg_temp;
CREATE TEMP TABLE redeem_codes (
    id BIGINT PRIMARY KEY, code TEXT, type TEXT, value NUMERIC(20,8), status TEXT,
    used_by BIGINT, created_by BIGINT, source TEXT, notes TEXT,
    threshold_exempt BOOLEAN, used_at TIMESTAMPTZ, created_at TIMESTAMPTZ
);
CREATE TEMP TABLE support_trial_grants (
    user_id BIGINT PRIMARY KEY, amount NUMERIC(20,8), operator_id BIGINT,
    redeem_code_id BIGINT, legacy BOOLEAN, granted_at TIMESTAMPTZ
);
CREATE TEMP TABLE users (
    id BIGINT PRIMARY KEY, balance NUMERIC(20,8), gift_balance NUMERIC(20,8),
    total_recharged NUMERIC(20,8)
);
INSERT INTO users VALUES (1, 27, 15, 12);`)
	require.NoError(t, err)

	cases := []struct {
		name   string
		change string
		repair bool
	}{
		{name: "matching durable support grant", repair: true},
		{name: "non-exempt matching grant", change: "UPDATE redeem_codes SET threshold_exempt=FALSE WHERE id=$1", repair: true},
		{name: "manual balance ledger", change: "UPDATE redeem_codes SET type='admin_balance' WHERE id=$1"},
		{name: "another source", change: "UPDATE redeem_codes SET source='admin' WHERE id=$1"},
		{name: "unused code", change: "UPDATE redeem_codes SET status='unused' WHERE id=$1"},
		{name: "another ledger amount", change: "UPDATE redeem_codes SET value=14.99 WHERE id=$1"},
		{name: "custom note", change: "UPDATE redeem_codes SET notes='Special support grant' WHERE id=$1"},
		{name: "already correct", change: "UPDATE redeem_codes SET notes='测试额度-15刀' WHERE id=$1"},
		{name: "old note with suffix", change: "UPDATE redeem_codes SET notes='Support trial credit ' WHERE id=$1"},
		{name: "missing durable claim", change: "DELETE FROM support_trial_grants WHERE redeem_code_id=$1"},
		{name: "legacy claim", change: "UPDATE support_trial_grants SET legacy=TRUE WHERE redeem_code_id=$1"},
		{name: "another claim amount", change: "UPDATE support_trial_grants SET amount=14.99 WHERE redeem_code_id=$1"},
		{name: "another user", change: "UPDATE support_trial_grants SET user_id=user_id+10000 WHERE redeem_code_id=$1"},
		{name: "another operator", change: "UPDATE support_trial_grants SET operator_id=operator_id+1 WHERE redeem_code_id=$1"},
		{name: "another ledger", change: "UPDATE support_trial_grants SET redeem_code_id=redeem_code_id+10000 WHERE redeem_code_id=$1"},
		{name: "missing ledger operator", change: "UPDATE redeem_codes SET created_by=NULL WHERE id=$1"},
		{name: "missing ledger user", change: "UPDATE redeem_codes SET used_by=NULL WHERE id=$1"},
		{name: "missing note", change: "UPDATE redeem_codes SET notes=NULL WHERE id=$1"},
		{name: "unknown legacy status", change: "UPDATE support_trial_grants SET legacy=NULL WHERE redeem_code_id=$1"},
	}
	for i, tc := range cases {
		id := i + 1
		_, err = tx.ExecContext(ctx, `INSERT INTO redeem_codes VALUES ($1,'fixture-'||($1::bigint)::text,'balance',15,'used',$1,77,'support_trial','Support trial credit',TRUE,'2026-11-01 00:00:00+00','2026-11-01 00:00:00+00')`, id)
		require.NoError(t, err, tc.name)
		_, err = tx.ExecContext(ctx, `INSERT INTO support_trial_grants VALUES ($1,15,77,$1,FALSE,'2026-11-01 00:00:00+00')`, id)
		require.NoError(t, err, tc.name)
		if tc.change != "" {
			_, err = tx.ExecContext(ctx, tc.change, id)
			require.NoError(t, err, tc.name)
		}
	}
	before := supportTrialNoteSnapshot(t, ctx, tx, "redeem_codes", "id", " - 'notes'")
	claims := supportTrialNoteSnapshot(t, ctx, tx, "support_trial_grants", "user_id", "")
	wallets := supportTrialNoteSnapshot(t, ctx, tx, "users", "id", "")
	var originalNotes []sql.NullString
	for i := range cases {
		var note sql.NullString
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT notes FROM redeem_codes WHERE id=$1", i+1).Scan(&note))
		originalNotes = append(originalNotes, note)
	}
	result, err := tx.ExecContext(ctx, string(raw))
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 2, affected)
	for i, tc := range cases {
		var note sql.NullString
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT notes FROM redeem_codes WHERE id=$1", i+1).Scan(&note))
		want := originalNotes[i]
		if tc.repair {
			want = sql.NullString{String: "测试额度-15刀", Valid: true}
		}
		require.Equal(t, want, note, tc.name)
	}
	require.Equal(t, before, supportTrialNoteSnapshot(t, ctx, tx, "redeem_codes", "id", " - 'notes'"), "other ledger fields must remain unchanged")
	require.Equal(t, claims, supportTrialNoteSnapshot(t, ctx, tx, "support_trial_grants", "user_id", ""), "durable claims must remain unchanged")
	require.Equal(t, wallets, supportTrialNoteSnapshot(t, ctx, tx, "users", "id", ""), "wallets must remain unchanged")
	result, err = tx.ExecContext(ctx, string(raw))
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "replaying metadata repair must be a no-op")
}

func supportTrialNoteSnapshot(t *testing.T, ctx context.Context, tx *sql.Tx, table, order, suffix string) string {
	t.Helper()
	var value string
	// These identifiers are fixed test-fixture literals, never external input.
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(r)"+suffix+" ORDER BY "+order+"),'[]'::jsonb)::text FROM "+table+" r").Scan(&value))
	return value
}

func supportTrialNotesPostgres(t *testing.T) *sql.DB {
	t.Helper()
	if dsn := os.Getenv("SUPPORT_TRIAL_NOTES_TEST_DSN"); dsn != "" {
		db, err := sql.Open("postgres", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	return startMigration175Postgres(t)
}
