//go:build unit

package ent_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestCheckinRewardCampaignEntCreatedSchemaEnforcesChecks(t *testing.T) {
	db, err := sql.Open("sqlite", "file:checkin_reward_campaign_constraints?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	driver := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(driver)))
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	startDate := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

	_, err = client.CheckinRewardCampaign.Create().
		SetName("builder-invalid-status").
		SetStatus("invalid").
		SetStartDate(startDate).
		SetEndDate(endDate).
		Save(ctx)
	require.Error(t, err)

	now := time.Now().UTC()
	insertRaw := func(name, status string, start, end time.Time) error {
		_, insertErr := client.ExecContext(ctx, `
INSERT INTO checkin_reward_campaigns
    (name, status, start_date, end_date, reward_tiers, created_at, updated_at)
VALUES (?, ?, ?, ?, '[]', ?, ?)`, name, status, start, end, now, now)
		return insertErr
	}

	require.Error(t, insertRaw("database-invalid-status", "invalid", startDate, endDate))
	require.Error(t, insertRaw("database-reversed-dates", "draft", endDate, startDate))
	require.NoError(t, insertRaw("valid", "draft", startDate, endDate))

	var createTableSQL string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT sql FROM sqlite_master
WHERE type = 'table' AND name = 'checkin_reward_campaigns'`).Scan(&createTableSQL))
	require.Contains(t, createTableSQL, "checkin_reward_campaigns_status_check")
	require.Contains(t, createTableSQL, "checkin_reward_campaigns_date_order_check")
}
