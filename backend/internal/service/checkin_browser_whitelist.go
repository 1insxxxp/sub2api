package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// scanRiskRow closes the Ent client's database rows before returning to the transaction.
func scanRiskRow(rows *sql.Rows, dest ...any) error {
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return rows.Scan(dest...)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return sql.ErrNoRows
}

func riskAudit(ctx context.Context, client *dbent.Client, userID int64, browserHash, date, action string, actorID int64, reason string) error {
	_, err := client.ExecContext(ctx, `INSERT INTO checkin_browser_audit (user_id,browser_hash,checkin_date,action,actor_id,reason) VALUES ($1,$2,$3,$4,$5,$6)`, userID, browserHash, date, action, actorID, reason)
	return err
}

func (s *CheckinService) SetBrowserWhitelist(ctx context.Context, email string, enabled bool, actorID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if actorID <= 0 || reason == "" || len(reason) > 1000 {
		return infraerrors.BadRequest("CHECKIN_BROWSER_WHITELIST_INVALID", "administrator and reason are required")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	rows, err := client.QueryContext(ctx, `SELECT id FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL`, strings.TrimSpace(email))
	if err != nil {
		return err
	}
	var userID int64
	if err = scanRiskRow(rows, &userID); errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	} else if err != nil {
		return err
	}
	action := "whitelist_remove"
	var result sql.Result
	if enabled {
		action = "whitelist_add"
		result, err = client.ExecContext(ctx, `INSERT INTO checkin_browser_whitelist (user_id,created_by,reason) VALUES ($1,$2,$3) ON CONFLICT (user_id) DO NOTHING`, userID, actorID, reason)
	} else {
		result, err = client.ExecContext(ctx, `DELETE FROM checkin_browser_whitelist WHERE user_id=$1`, userID)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		if err = riskAudit(ctx, client, userID, "", "", action, actorID, reason); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type CheckinBrowserWhitelistEntry struct {
	Email     string    `json:"email"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *CheckinService) ListBrowserWhitelist(ctx context.Context, page, pageSize int, search string) ([]CheckinBrowserWhitelistEntry, int64, error) {
	search = "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
	const filter = ` FROM checkin_browser_whitelist w JOIN users u ON u.id=w.user_id WHERE lower(u.email) LIKE $1`
	rows, err := s.entClient.QueryContext(ctx, `SELECT count(*)`+filter, search)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err = scanRiskRow(rows, &total); err != nil {
		return nil, 0, err
	}
	rows, err = s.entClient.QueryContext(ctx, `SELECT u.email,w.reason,w.created_at`+filter+` ORDER BY w.created_at DESC,w.user_id LIMIT $2 OFFSET $3`, search, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []CheckinBrowserWhitelistEntry{}
	for rows.Next() {
		var item CheckinBrowserWhitelistEntry
		if err = rows.Scan(&item.Email, &item.Reason, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
