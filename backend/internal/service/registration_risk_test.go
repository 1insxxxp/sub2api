package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newRegistrationRiskTestDB(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE account_risk_events(user_id BIGINT,event_type TEXT,browser_hash TEXT,ip_address TEXT,user_agent TEXT,created_at TIMESTAMP);
 CREATE TABLE account_registration_risk_counts(day TEXT, action TEXT, hits BIGINT, PRIMARY KEY(day,action));
 CREATE TABLE account_registration_cooldowns(ip_address TEXT PRIMARY KEY,expires_at TIMESTAMP);
 CREATE TABLE account_registration_captcha(ip_address TEXT,browser_hash TEXT,scope TEXT,expires_at TIMESTAMP, PRIMARY KEY(ip_address,browser_hash,scope));`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	return db, client
}

func TestRegistrationRiskChallengeCooldownAndExpiry(t *testing.T) {
	db, client := newRegistrationRiskTestDB(t)
	source := &RegistrationRiskSource{IP: "192.0.2.2", BrowserHash: "browser"}
	now := time.Now().UTC()
	cfg := AccountRiskConfig{RegistrationMode: "enforce", WindowMinutes: 5, ChallengeCount: 3, CooldownCount: 6, CooldownMinutes: 15}
	ctx := context.Background()
	signup := func(n int, at time.Time) error {
		tx, err := client.Tx(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		if err = source.CompleteRegistration(ctx, tx.Client(), cfg, int64(n), "user@example.com", at); err != nil {
			return err
		}
		return tx.Commit()
	}
	for n := 1; n <= 3; n++ {
		require.NoError(t, signup(n, now))
	}
	require.ErrorIs(t, signup(4, now), ErrRegistrationCaptchaRequired)
	for n := 4; n <= 6; n++ {
		require.NoError(t, source.saveCaptchaReceipt(ctx, client, "user@example.com", now))
		require.NoError(t, signup(n, now))
	}
	require.ErrorIs(t, signup(7, now.Add(6*time.Minute)), ErrRegistrationCoolingDown)
	require.NoError(t, signup(7, now.Add(16*time.Minute)))
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM account_risk_events").Scan(&count))
	require.Equal(t, 7, count)
}
