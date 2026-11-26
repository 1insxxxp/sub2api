package service

import (
	"context"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var errCheckinBrowserReviewConflict = infraerrors.Conflict("CHECKIN_BROWSER_REVIEW_CONFLICT", "review status has changed")

type CheckinBrowserReview struct {
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	OwnerEmail  string    `json:"owner_email"`
	BrowserHash string    `json:"browser_hash"`
	CheckinDate string    `json:"checkin_date"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *CheckinService) ListCheckinBrowserReviews(ctx context.Context, page, pageSize int, search, status, date string) ([]CheckinBrowserReview, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if status != "" && status != "observed" && status != "pending" && status != "approved" && status != "rejected" {
		return nil, 0, infraerrors.BadRequest("CHECKIN_BROWSER_REVIEW_INVALID_STATUS", "invalid review status")
	}
	search = "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
	filter := ` FROM checkin_browser_reviews r JOIN users u ON u.id=r.user_id JOIN checkin_browser_claims c ON c.browser_hash=r.browser_hash AND c.checkin_date=r.checkin_date JOIN users owner ON owner.id=c.user_id WHERE ($1='' OR r.status=$1) AND ($3='' OR r.checkin_date=$3) AND (LOWER(u.email) LIKE $2 OR LOWER(owner.email) LIKE $2)`
	rows, err := s.entClient.QueryContext(ctx, `SELECT COUNT(*)`+filter, status, search, date)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	err = scanRiskRow(rows, &total)
	if err != nil {
		return nil, 0, err
	}
	rows, err = s.entClient.QueryContext(ctx, `SELECT r.user_id, u.email, owner.email, r.browser_hash, r.checkin_date, r.status, r.created_at`+filter+` ORDER BY r.created_at DESC, r.user_id DESC, r.browser_hash LIMIT $4 OFFSET $5`, status, search, date, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]CheckinBrowserReview, 0)
	for rows.Next() {
		var item CheckinBrowserReview
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.OwnerEmail, &item.BrowserHash, &item.CheckinDate, &item.Status, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// DecideCheckinBrowserReview records an administrator decision atomically with its audit entry.
func (s *CheckinService) DecideCheckinBrowserReview(ctx context.Context, userID int64, browserHash, date, decision string, actorID int64, reason string) error {
	if userID <= 0 || actorID <= 0 || browserHash == "" || date == "" || (decision != "approved" && decision != "rejected") || strings.TrimSpace(reason) == "" || len(reason) > 1000 {
		return infraerrors.BadRequest("CHECKIN_BROWSER_REVIEW_INVALID", "invalid check-in review decision")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	today, _ := s.currentBeijingDay()
	result, err := client.ExecContext(ctx, `UPDATE checkin_browser_reviews SET status=$1, reviewed_at=CURRENT_TIMESTAMP, reviewed_by=$2, reason=$3 WHERE user_id=$4 AND browser_hash=$5 AND checkin_date=$6 AND (status='pending' OR (status='rejected' AND $1='approved' AND checkin_date=$7))`, decision, actorID, strings.TrimSpace(reason), userID, browserHash, date, today)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errCheckinBrowserReviewConflict
	}
	err = riskAudit(ctx, client, userID, browserHash, date, decision, actorID, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// claimCheckinBrowser runs inside the award transaction. The unique browser
// and date key serializes concurrent claims before any reward is credited.
func claimCheckinBrowser(ctx context.Context, client *dbent.Client, browserHash, date string, userID int64, enforce bool) (bool, error) {
	rows, err := client.QueryContext(ctx, `SELECT EXISTS(SELECT 1 FROM checkin_browser_whitelist WHERE user_id=$1),
   EXISTS(SELECT 1 FROM checkin_browser_reviews WHERE user_id=$1 AND checkin_date=$2 AND status='pending'),
   EXISTS(SELECT 1 FROM checkin_browser_reviews WHERE user_id=$1 AND checkin_date=$2 AND status='rejected'),
   EXISTS(SELECT 1 FROM checkin_browser_reviews WHERE user_id=$1 AND checkin_date=$2 AND status='approved')`, userID, date)
	if err != nil {
		return false, err
	}
	var whitelisted, held, rejected, approved bool
	if err = scanRiskRow(rows, &whitelisted, &held, &rejected, &approved); err != nil {
		return false, err
	}
	if enforce && rejected && !whitelisted && !approved {
		return false, ErrCheckinReviewRejected
	}
	if enforce && held && !whitelisted && !approved {
		return true, nil
	}
	// A whitelisted first claimant still establishes the browser's daily owner.
	result, err := client.ExecContext(ctx, `INSERT INTO checkin_browser_claims (browser_hash,checkin_date,user_id) VALUES ($1,$2,$3) ON CONFLICT (browser_hash,checkin_date) DO NOTHING`, browserHash, date, userID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 1 || whitelisted || approved {
		return false, err
	}
	rows, err = client.QueryContext(ctx, `SELECT user_id FROM checkin_browser_claims WHERE browser_hash=$1 AND checkin_date=$2`, browserHash, date)
	if err != nil {
		return false, err
	}
	var owner int64
	if err = scanRiskRow(rows, &owner); err != nil {
		return false, err
	}
	if owner == userID {
		return false, nil
	}
	status := "observed"
	if enforce {
		status = "pending"
	}
	_, err = client.ExecContext(ctx, `INSERT INTO checkin_browser_reviews (user_id,browser_hash,checkin_date,status) VALUES ($1,$2,$3,$4) ON CONFLICT(user_id,browser_hash,checkin_date) DO NOTHING`, userID, browserHash, date, status)
	return enforce, err
}
