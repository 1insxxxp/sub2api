//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

// Each test uses a private schema even when multiple packages share a local PostgreSQL instance.
func TestAccountRiskAssociationEvidence(t *testing.T) {
	var db *sql.DB
	var err error
	if dsn := os.Getenv("SUB2API_TEST_POSTGRES_DSN"); dsn != "" {
		db, err = sql.Open("postgres", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
	} else {
		db = startCheckinRewardCampaignServicePostgres(t)
	}
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("test_account_risk_%d", time.Now().UnixNano())
	_, err = db.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec("DROP SCHEMA " + schema + " CASCADE") })
	_, err = db.Exec("SET search_path TO " + schema)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (id BIGINT PRIMARY KEY,email TEXT,registration_ip TEXT, registration_user_agent TEXT, last_login_ip TEXT,last_login_user_agent TEXT,created_at TIMESTAMPTZ DEFAULT NOW(),last_login_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ);
        CREATE TABLE payment_orders (user_id BIGINT,client_ip TEXT,created_at TIMESTAMPTZ DEFAULT NOW());
        CREATE TABLE usage_logs (user_id BIGINT,ip_address TEXT,created_at TIMESTAMPTZ DEFAULT NOW());
        CREATE TABLE user_affiliates (user_id BIGINT, inviter_id BIGINT, created_at TIMESTAMPTZ DEFAULT NOW());
        CREATE TABLE user_checkins (user_id BIGINT,reward_amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
        INSERT INTO users(id,email,registration_ip) VALUES (1,'first@example.org','192.0.2.1'),(2,'second@example.org','192.0.2.1'),(3,'shared-network@example.org','192.0.2.1'),(4,'no-match@example.org','');
        INSERT INTO payment_orders VALUES (1,'192.0.2.2',now());
        INSERT INTO usage_logs VALUES (2,'192.0.2.2',now()),(3,'192.0.2.2',now() - interval '31 days');
        INSERT INTO user_affiliates VALUES (2,1,now());
        INSERT INTO user_checkins VALUES (1,5,now()),(2,3.5,now()),(1,10,now()-interval '91 days');`)
	require.NoError(t, err)
	for _, filename := range []string{"272_checkin_browser_risk.sql", "273_account_risk_events.sql", "274_registration_ip_risk.sql"} {
		data, err := os.ReadFile("../../migrations/" + filename)
		require.NoError(t, err)
		_, err = db.Exec(string(data))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO account_risk_events(user_id,event_type,browser_hash,ip_address,user_agent)
      VALUES (1,'login',repeat('a',64),'192.0.2.3','test-browser'), (2,'checkin',repeat('a',64),'192.0.2.3','another agent'), (4,'register',repeat('b',64),'','');
      INSERT INTO account_risk_events(user_id,event_type,browser_hash,ip_address,user_agent,created_at) VALUES (4,'login',repeat('a',64),'','',NOW()-INTERVAL '91 days');
      INSERT INTO checkin_browser_claims (user_id,browser_hash,checkin_date) VALUES (1,repeat('a',64),'2026-11-26');
      INSERT INTO checkin_browser_reviews (user_id,browser_hash,checkin_date,status) VALUES (2,repeat('a',64),'2026-11-26','pending');`)
	require.NoError(t, err)
	svc := NewCheckinService(dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db))), nil, nil)
	_, err = db.Exec(`INSERT INTO account_registration_cooldowns VALUES ('192.0.2.2', NOW()-INTERVAL '1 hour');
        INSERT INTO account_registration_captcha VALUES ('192.0.2.2','test','x',NOW()-INTERVAL '1 hour');
        INSERT INTO account_registration_risk_counts VALUES (CURRENT_DATE-91,'challenge',5);
        INSERT INTO checkin_browser_claims(user_id,browser_hash,checkin_date,created_at) VALUES (1,'expired','expired',NOW()-INTERVAL '91 days');
        INSERT INTO checkin_browser_reviews(user_id,browser_hash,checkin_date,created_at) VALUES (2,'expired','expired',NOW()-INTERVAL '91 days');
        INSERT INTO checkin_browser_audit(user_id,browser_hash,checkin_date,action,created_at) VALUES (2,'expired','expired','approved',NOW()-INTERVAL '91 days');
        INSERT INTO checkin_browser_whitelist(user_id,created_at) VALUES (3,NOW()-INTERVAL '91 days');`)
	require.NoError(t, err)
	require.NoError(t, svc.maintainAccountRiskEvidence(ctx))
	rows, total, err := svc.ListRiskAssociations(ctx, 1, 10, RiskAssociationFilters{Search: "SECOND@", Status: "pending", Reason: "browser"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	pair := rows[0]
	require.Equal(t, "first@example.org", pair.UserEmail)
	require.Equal(t, "second@example.org", pair.RelatedEmail)
	require.Equal(t, pair.UserEmail, pair.InviterEmail)
	require.True(t, pair.HighRisk)
	require.ElementsMatch(t, []string{"browser", "invitation", "registration_ip", "login_ip", "checkin_ip", "payment_ip", "usage_ip"}, pair.Reasons)
	require.EqualValues(t, 1, pair.CheckinCount)
	require.Equal(t, 5.0, pair.RewardAmount)
	require.Equal(t, 3.5, pair.RelatedRewardAmount)
	require.Equal(t, "test-browser", pair.UserAgent)
	require.Equal(t, "another agent", pair.RelatedUserAgent)
	rows, total, err = svc.ListRiskAssociations(ctx, 1, 1, RiskAssociationFilters{Search: "shared-network"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 1)
	require.False(t, rows[0].HighRisk)
	require.Equal(t, []string{"registration_ip"}, rows[0].Reasons)
	_, total, err = svc.ListRiskAssociations(ctx, 3, 1, RiskAssociationFilters{Search: "shared-network"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	var expired int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM account_risk_events WHERE created_at<now()-interval '90 days'").Scan(&expired))
	require.Zero(t, expired)
	for _, table := range []string{"account_registration_cooldowns", "account_registration_captcha", "account_registration_risk_counts", "checkin_browser_audit"} {
		require.NoError(t, db.QueryRow("SELECT count(*) FROM "+table).Scan(&expired))
		require.Zerof(t, expired, "expired rows in %s", table)
	}
	for _, table := range []string{"checkin_browser_claims", "checkin_browser_reviews"} {
		require.NoError(t, db.QueryRow("SELECT count(*) FROM "+table+" WHERE browser_hash='expired'").Scan(&expired))
		require.Zerof(t, expired, "expired rows in %s", table)
	}
	require.NoError(t, db.QueryRow("SELECT count(*) FROM checkin_browser_whitelist WHERE user_id=3").Scan(&expired))
	require.Equal(t, 1, expired)
	_, _, err = svc.ListRiskAssociations(ctx, 1, 20, RiskAssociationFilters{StartDate: "2025-10-99"})
	require.Error(t, err)
	_, total, err = svc.ListRiskAssociations(ctx, 1, 20, RiskAssociationFilters{StartDate: "2000-01-01", EndDate: "2000-01-02"})
	require.NoError(t, err)
	require.Zero(t, total)
}
