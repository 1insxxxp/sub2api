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

func TestCheckinBrowserRiskClaimsAndReview(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE checkin_browser_claims (browser_hash TEXT, checkin_date TEXT, user_id BIGINT, PRIMARY KEY(browser_hash, checkin_date));
		CREATE TABLE checkin_browser_reviews (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, status TEXT, PRIMARY KEY(user_id, browser_hash, checkin_date));
		CREATE TABLE checkin_browser_whitelist (user_id BIGINT PRIMARY KEY);`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	ctx := context.Background()
	blocked, err := claimCheckinBrowser(ctx, client, "browser", "2026-11-26", 1, true)
	require.NoError(t, err)
	require.False(t, blocked)
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-26", 2, true)
	require.NoError(t, err)
	require.True(t, blocked)
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM checkin_browser_reviews WHERE user_id=2`).Scan(&status))
	require.Equal(t, "pending", status)
	blocked, err = claimCheckinBrowser(ctx, client, "new-browser", "2026-11-26", 2, true)
	require.NoError(t, err)
	require.True(t, blocked, "changing browser must not release a pending account")
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-26", 4, false)
	require.NoError(t, err)
	require.False(t, blocked)
	require.NoError(t, db.QueryRow("SELECT status FROM checkin_browser_reviews WHERE user_id=4").Scan(&status))
	require.Equal(t, "observed", status)
	_, err = db.Exec(`UPDATE checkin_browser_reviews SET status='approved' WHERE user_id=2`)
	require.NoError(t, err)
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-26", 2, true)
	require.NoError(t, err)
	require.False(t, blocked)
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-27", 3, true)
	require.NoError(t, err)
	require.False(t, blocked)
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-27", 2, true)
	require.NoError(t, err)
	require.True(t, blocked, "previous day's approval cannot release a new day's review")
	_, err = db.Exec("INSERT INTO checkin_browser_whitelist (user_id) VALUES (2)")
	require.NoError(t, err)
	blocked, err = claimCheckinBrowser(ctx, client, "browser", "2026-11-27", 2, true)
	require.NoError(t, err)
	require.False(t, blocked)

}

func TestCheckinBrowserReviewDecisionIsAuditedAndCannotBeOverwritten(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE checkin_browser_reviews (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, status TEXT, reviewed_at TIMESTAMP, reviewed_by BIGINT, reason TEXT, PRIMARY KEY(user_id, browser_hash, checkin_date));
		CREATE TABLE checkin_browser_audit (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, action TEXT, actor_id BIGINT, reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO checkin_browser_reviews (user_id, browser_hash, checkin_date, status) VALUES (2, 'browser', '2026-11-26', 'pending');`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	svc := NewCheckinService(client, nil, nil)
	ctx := context.Background()
	err = svc.DecideCheckinBrowserReview(ctx, 2, "browser", "2026-11-26", "approved", 9, "verified")
	require.NoError(t, err)
	var status, reason string
	var actor int64
	require.NoError(t, db.QueryRow(`SELECT status, reviewed_by, reason FROM checkin_browser_reviews`).Scan(&status, &actor, &reason))
	require.Equal(t, "approved", status)
	require.EqualValues(t, 9, actor)
	require.Equal(t, "verified", reason)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM checkin_browser_audit WHERE action='approved' AND actor_id=9`).Scan(&count))
	require.Equal(t, 1, count)
	err = svc.DecideCheckinBrowserReview(ctx, 2, "browser", "2026-11-26", "rejected", 9, "changed")
	require.Error(t, err)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM checkin_browser_audit`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestRejectedBrowserReviewCanBeReconsidered(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE checkin_browser_reviews (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, status TEXT, reviewed_at TIMESTAMP, reviewed_by BIGINT, reason TEXT);
 CREATE TABLE checkin_browser_audit (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, action TEXT, actor_id BIGINT, reason TEXT);
 CREATE TABLE checkin_browser_whitelist (user_id BIGINT);
 INSERT INTO checkin_browser_reviews (user_id,browser_hash,checkin_date,status) VALUES (2,'b','2026-11-26','pending')`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	svc := NewCheckinService(client, nil, nil)
	svc.now = func() time.Time { return time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	require.NoError(t, svc.DecideCheckinBrowserReview(ctx, 2, "b", "2026-11-26", "rejected", 9, "reviewed"))
	_, err = claimCheckinBrowser(ctx, client, "changed", "2026-11-26", 2, true)
	require.ErrorIs(t, err, ErrCheckinReviewRejected)
	require.NoError(t, svc.DecideCheckinBrowserReview(ctx, 2, "b", "2026-11-26", "approved", 10, "appeal accepted"))
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM checkin_browser_audit").Scan(&count))
	require.Equal(t, 2, count)
}

func TestListCheckinBrowserReviewsShowsEmails(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE users (id BIGINT PRIMARY KEY, email TEXT);
		CREATE TABLE checkin_browser_claims (browser_hash TEXT, checkin_date TEXT, user_id BIGINT);
		CREATE TABLE checkin_browser_reviews (user_id BIGINT, browser_hash TEXT, checkin_date TEXT, status TEXT, created_at TIMESTAMP);
		INSERT INTO users VALUES (1, 'first@example.com'), (2, 'second@example.com');
		INSERT INTO checkin_browser_claims VALUES ('browser', '2026-11-26', 1);
		INSERT INTO checkin_browser_reviews VALUES (2, 'browser', '2026-11-26', 'pending', '2026-11-26 08:00:00');`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	svc := NewCheckinService(client, nil, nil)
	items, total, err := svc.ListCheckinBrowserReviews(context.Background(), 1, 20, "second@example.com", "pending", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "second@example.com", items[0].UserEmail)
	require.Equal(t, "first@example.com", items[0].OwnerEmail)
}

func TestCheckinBrowserWhitelistChangesAreAudited(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY,email TEXT, deleted_at TIMESTAMP);
 CREATE TABLE checkin_browser_whitelist (user_id INTEGER PRIMARY KEY, created_by INTEGER, reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE checkin_browser_audit (user_id BIGINT,browser_hash TEXT,checkin_date TEXT,action TEXT,actor_id BIGINT,reason TEXT);
 INSERT INTO users VALUES (2,'second@example.com',NULL);`)
	require.NoError(t, err)
	svc := NewCheckinService(dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))), nil, nil)
	ctx := context.Background()
	for _, enabled := range []bool{true, true, false, false} {
		require.NoError(t, svc.SetBrowserWhitelist(ctx, "second@example.com", enabled, 9, "family"))
	}
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM checkin_browser_whitelist").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.QueryRow("SELECT count(*) FROM checkin_browser_audit").Scan(&count))
	require.Equal(t, 2, count)
}
