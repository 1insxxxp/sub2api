package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type channelAcceptanceRepository struct{ db *sql.DB }

func NewChannelAcceptanceRepository(db *sql.DB) service.ChannelAcceptanceRepository {
	return &channelAcceptanceRepository{db: db}
}

// Every scheduling mutation takes this transaction-level lock BEFORE reading
// lanes/budget. READ COMMITTED ensures the statement after a waiter acquires the
// lock observes the preceding commit. Never hold it during an upstream request.
func (r *channelAcceptanceRepository) schedulingTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('channel-acceptance-scheduling-v1', 0))`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func acceptanceValidMoney(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 1e12
}

func validateAcceptancePreview(p *service.AcceptancePreview) error {
	if p == nil || p.ID == "" || p.ActorID <= 0 || p.RuleVersion == "" || p.CreatedAt.IsZero() || !p.ExpiresAt.After(p.CreatedAt) || !acceptanceValidMoney(p.EstimatedCostUSD) {
		return service.ErrAcceptanceInvalid
	}
	if p.Request.BudgetUSD != nil && !acceptanceValidMoney(*p.Request.BudgetUSD) {
		return service.ErrAcceptanceInvalid
	}
	ids, ordinals := map[string]bool{}, map[string]bool{}
	for _, s := range p.Samples {
		key := fmt.Sprintf("%d:%d", s.Snapshot.AccountID, s.Ordinal)
		if s.ID == "" || ids[s.ID] || ordinals[key] || s.Ordinal < 0 || s.Snapshot.AccountID <= 0 || s.TimeoutSeconds <= 0 || s.TimeoutSeconds > 360 || !acceptanceValidMoney(s.EstimatedCostUSD) || s.Status != "pending" || s.Result != nil || s.LeaseToken != "" || s.LeaseUntil != nil {
			return service.ErrAcceptanceInvalid
		}
		ids[s.ID], ordinals[key] = true, true
	}
	return nil
}

func (r *channelAcceptanceRepository) SavePreview(ctx context.Context, p *service.AcceptancePreview) error {
	if err := validateAcceptancePreview(p); err != nil {
		return err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	// Previews are immutable. Overwriting one would invalidate the confirmed
	// manifest or permit an idempotency key to acquire different semantics.
	_, err = r.db.ExecContext(ctx, `INSERT INTO channel_acceptance_previews(id,actor_id,body,created_at,expires_at) VALUES($1,$2,$3,$4,$5)`, p.ID, p.ActorID, body, p.CreatedAt, p.ExpiresAt)
	return err
}

type acceptanceQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func acceptancePreview(ctx context.Context, q acceptanceQuerier, id string, actor int64, lock bool) (*service.AcceptancePreview, error) {
	query := `SELECT body,actor_id,created_at,expires_at FROM channel_acceptance_previews WHERE id=$1 AND actor_id=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	var body []byte
	p := new(service.AcceptancePreview)
	if err := q.QueryRowContext(ctx, query, id, actor).Scan(&body, &p.ActorID, &p.CreatedAt, &p.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAcceptanceNotFound
		}
		return nil, err
	}
	// Columns are authoritative, even if a JSON snapshot contains older dates.
	created, expiry := p.CreatedAt, p.ExpiresAt
	if err := json.Unmarshal(body, p); err != nil {
		return nil, err
	}
	p.ActorID, p.CreatedAt, p.ExpiresAt = actor, created, expiry
	return p, nil
}

func (r *channelAcceptanceRepository) GetPreview(ctx context.Context, id string, actor int64) (*service.AcceptancePreview, error) {
	return acceptancePreview(ctx, r.db, id, actor, false)
}

// FindRunByPreview lets Start return an already accepted run before checking
// current configuration or preview expiry. Keep the actor predicate here as well
// as the service's preview ownership check; no match is an ordinary nil result.
func (r *channelAcceptanceRepository) FindRunByPreview(ctx context.Context, previewID string, actorID int64) (*service.AcceptanceRun, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM channel_acceptance_runs WHERE preview_id=$1 AND actor_id=$2`, previewID, actorID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetRun(ctx, id)
}

func (r *channelAcceptanceRepository) CreateRun(ctx context.Context, input *service.AcceptancePreview) (*service.AcceptanceRun, error) {
	if input == nil {
		return nil, service.ErrAcceptanceInvalid
	}
	// Creation also shares the scheduler lock with retention: an expired preview
	// cannot disappear between confirmation and the insertion of its run.
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	p, err := acceptancePreview(ctx, tx, input.ID, input.ActorID, true)
	if err != nil {
		return nil, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM channel_acceptance_runs WHERE preview_id=$1`, p.ID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var unexpired bool
		if err = tx.QueryRowContext(ctx, `SELECT $1::timestamptz > clock_timestamp()`, p.ExpiresAt).Scan(&unexpired); err != nil {
			return nil, err
		}
		if !unexpired {
			return nil, service.ErrAcceptanceStale
		}
		if !p.CanStart || len(p.Samples) == 0 {
			return nil, service.ErrAcceptanceInvalid
		}
		if err = validateAcceptancePreview(p); err != nil {
			return nil, err
		}
		id = uuid.NewString()
		_, err = tx.ExecContext(ctx, `INSERT INTO channel_acceptance_runs(id,preview_id,actor_id,rule_version,status,budget_usd,estimated_cost_usd,total) VALUES($1,$2,$3,$4,'pending',$5,$6,$7)`, id, p.ID, p.ActorID, p.RuleVersion, p.Request.BudgetUSD, p.EstimatedCostUSD, len(p.Samples))
		if err != nil {
			return nil, err
		}
		for _, sample := range p.Samples {
			body, marshalErr := json.Marshal(sample)
			if marshalErr != nil {
				return nil, marshalErr
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO channel_acceptance_samples(id,run_id,account_id,ordinal,body,estimated_cost_usd,timeout_seconds,status) VALUES($1,$2,$3,$4,$5,$6,$7,'pending')`, sample.ID, id, sample.Snapshot.AccountID, sample.Ordinal, body, sample.EstimatedCostUSD, sample.TimeoutSeconds)
			if err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRun(ctx, id)
}

const acceptanceRunColumns = `r.id,r.preview_id,r.actor_id,r.rule_version,r.status,r.created_at,r.updated_at,r.budget_usd,r.estimated_cost_usd,r.reserved_cost_usd,r.spent_cost_usd,r.total,r.completed`

type acceptanceScanner interface{ Scan(...any) error }

func scanAcceptanceRun(row acceptanceScanner) (*service.AcceptanceRun, error) {
	run := new(service.AcceptanceRun)
	err := row.Scan(&run.ID, &run.PreviewID, &run.ActorID, &run.RuleVersion, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.BudgetUSD, &run.EstimatedCostUSD, &run.ReservedCostUSD, &run.SpentCostUSD, &run.Total, &run.Completed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAcceptanceNotFound
	}
	return run, err
}

const acceptanceSampleColumns = `s.body,s.id,s.run_id,s.status,s.estimated_cost_usd,s.lease_token,s.lease_until,s.result`

func scanAcceptanceSample(row acceptanceScanner) (*service.AcceptanceSample, error) {
	var body, result []byte
	s := new(service.AcceptanceSample)
	if err := row.Scan(&body, &s.ID, &s.RunID, &s.Status, &s.EstimatedCostUSD, &s.LeaseToken, &s.LeaseUntil, &result); err != nil {
		return nil, err
	}
	var snapshot service.AcceptanceSample
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, err
	}
	snapshot.ID, snapshot.RunID, snapshot.Status, snapshot.EstimatedCostUSD = s.ID, s.RunID, s.Status, s.EstimatedCostUSD
	snapshot.LeaseToken, snapshot.LeaseUntil, snapshot.Result = s.LeaseToken, s.LeaseUntil, nil
	if len(result) > 0 {
		if err := json.Unmarshal(result, &snapshot.Result); err != nil {
			return nil, err
		}
	}
	return &snapshot, nil
}

func (r *channelAcceptanceRepository) GetRun(ctx context.Context, id string) (*service.AcceptanceRun, error) {
	// A report must not combine counters before settlement with samples after it.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := scanAcceptanceRun(tx.QueryRowContext(ctx, `SELECT `+acceptanceRunColumns+` FROM channel_acceptance_runs r WHERE r.id=$1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+acceptanceSampleColumns+` FROM channel_acceptance_samples s WHERE run_id=$1 ORDER BY ordinal,account_id,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	run.Samples = make([]service.AcceptanceSample, 0, run.Total)
	for rows.Next() {
		sample, scanErr := scanAcceptanceSample(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		run.Samples = append(run.Samples, *sample)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return run, tx.Commit()
}

func (r *channelAcceptanceRepository) ListRuns(ctx context.Context, accounts []int64, limit int) ([]service.AcceptanceRun, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	query := `SELECT ` + acceptanceRunColumns + ` FROM channel_acceptance_runs r`
	args := []any{limit}
	if len(accounts) > 0 {
		query += ` WHERE EXISTS(SELECT 1 FROM channel_acceptance_samples s WHERE s.run_id=r.id AND s.account_id=ANY($2::bigint[]))`
		args = append(args, pq.Array(accounts))
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY r.created_at DESC,r.id DESC LIMIT $1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]service.AcceptanceRun, 0)
	for rows.Next() {
		run, scanErr := scanAcceptanceRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

// Only pending samples are cancelled. Running samples keep their lane and
// reservation until completion/expiry, including after the run becomes terminal.
func stopAcceptanceRun(ctx context.Context, tx *sql.Tx, id, status, reason string) error {
	body, err := json.Marshal(&service.AcceptanceResult{Status: "cancelled", Findings: []service.AcceptanceFinding{{Code: status, Level: "info", Message: reason}}})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE channel_acceptance_samples SET status='cancelled',result=$2 WHERE run_id=$1 AND status='pending'`, id, body)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE channel_acceptance_runs SET status=$2,updated_at=clock_timestamp(),completed=(SELECT COUNT(*) FROM channel_acceptance_samples WHERE run_id=$1 AND status NOT IN ('pending','running')) WHERE id=$1 AND (status IN ('pending','running') OR ($2='cancelled' AND EXISTS(SELECT 1 FROM channel_acceptance_samples WHERE run_id=$1 AND status='running')))`, id, status)
	return err
}

func (r *channelAcceptanceRepository) CancelRun(ctx context.Context, id string) error {
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var inflight bool
	if err = tx.QueryRowContext(ctx, `SELECT status,EXISTS(SELECT 1 FROM channel_acceptance_samples WHERE run_id=$1 AND status='running') FROM channel_acceptance_runs WHERE id=$1`, id).Scan(&status, &inflight); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrAcceptanceNotFound
		}
		return err
	}
	// A budget-exhausted run may still have live requests. Publish cancelled so
	// workers can stop those requests, without releasing their lanes or budget.
	if status != "cancelled" && (status == "pending" || status == "running" || inflight) {
		if err = stopAcceptanceRun(ctx, tx, id, "cancelled", "管理员已取消后续检测"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *channelAcceptanceRepository) ClaimNext(ctx context.Context, workerID string, now time.Time) (*service.AcceptanceSample, error) {
	if strings.TrimSpace(workerID) == "" || len(workerID) > 256 || now.IsZero() {
		return nil, service.ErrAcceptanceInvalid
	}
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_acceptance_samples WHERE status='running'`).Scan(&active); err != nil {
		return nil, err
	}
	if active >= 2 {
		return nil, tx.Commit()
	}
	for {
		// The earliest unfinished sample of each account/run goes first. Account
		// lanes are global, not scoped to a process or to a particular run.
		row := tx.QueryRowContext(ctx, `SELECT `+acceptanceSampleColumns+` FROM channel_acceptance_samples s
		JOIN channel_acceptance_runs r ON r.id=s.run_id
		WHERE s.status='pending' AND r.status IN ('pending','running')
		AND NOT EXISTS(SELECT 1 FROM channel_acceptance_samples busy WHERE busy.account_id=s.account_id AND busy.status='running')
		AND NOT EXISTS(SELECT 1 FROM channel_acceptance_samples earlier WHERE earlier.run_id=s.run_id AND earlier.account_id=s.account_id AND earlier.ordinal<s.ordinal AND earlier.status IN ('pending','running'))
		ORDER BY r.created_at,r.id,s.ordinal,s.account_id,s.id LIMIT 1 FOR UPDATE OF s,r`)
		s, scanErr := scanAcceptanceSample(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil, tx.Commit()
		}
		if scanErr != nil {
			return nil, scanErr
		}
		// All budget arithmetic stays NUMERIC inside PostgreSQL; a single guarded
		// UPDATE both approves and reserves the next request before it is returned.
		reserved, execErr := tx.ExecContext(ctx, `UPDATE channel_acceptance_runs r SET reserved_cost_usd=r.reserved_cost_usd+s.estimated_cost_usd,status='running',updated_at=clock_timestamp()
		FROM channel_acceptance_samples s WHERE r.id=$1 AND s.id=$2 AND (r.budget_usd IS NULL OR r.spent_cost_usd+r.reserved_cost_usd+s.estimated_cost_usd<=r.budget_usd)`, s.RunID, s.ID)
		if execErr != nil {
			return nil, execErr
		}
		count, execErr := reserved.RowsAffected()
		if execErr != nil {
			return nil, execErr
		}
		if count == 0 {
			if err = stopAcceptanceRun(ctx, tx, s.RunID, "budget_exhausted", "剩余预算不足，已停止后续检测"); err != nil {
				return nil, err
			}
			continue
		}
		s.LeaseToken = uuid.NewString()
		until := now.Add(time.Duration(s.TimeoutSeconds+60) * time.Second)
		s.LeaseUntil = &until
		s.Status = "running"
		_, err = tx.ExecContext(ctx, `UPDATE channel_acceptance_samples SET status='running',worker_id=$2,lease_token=$3,lease_until=$4 WHERE id=$1`, s.ID, workerID, s.LeaseToken, until)
		if err != nil {
			return nil, err
		}
		return s, tx.Commit()
	}
}

func acceptanceTerminalResult(status string) bool {
	switch status {
	case "passed", "anomaly", "unsupported", "inconclusive", "error", "cancelled", "unknown":
		return true
	}
	return false
}

func refreshAcceptanceRun(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `UPDATE channel_acceptance_runs r SET completed=c.n,updated_at=clock_timestamp(),
	status=CASE WHEN r.status IN ('cancelled','budget_exhausted') THEN r.status WHEN c.n=r.total THEN 'completed' ELSE r.status END
	FROM (SELECT COUNT(*) AS n FROM channel_acceptance_samples WHERE run_id=$1 AND status NOT IN ('pending','running')) c WHERE r.id=$1`, id)
	return err
}

func (r *channelAcceptanceRepository) CompleteSample(ctx context.Context, id, token string, result *service.AcceptanceResult) error {
	if token == "" || result == nil || !acceptanceTerminalResult(result.Status) || (result.CostUSD != nil && !acceptanceValidMoney(*result.CostUSD)) {
		return service.ErrAcceptanceInvalid
	}
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var runID, storedToken, status string
	var live bool
	err = tx.QueryRowContext(ctx, `SELECT run_id,lease_token,status,COALESCE(lease_until>clock_timestamp(),FALSE) FROM channel_acceptance_samples WHERE id=$1`, id).Scan(&runID, &storedToken, &status, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAcceptanceNotFound
	}
	if err != nil {
		return err
	}
	if storedToken != token {
		return service.ErrAcceptanceStale
	}
	if status != "running" {
		return tx.Commit()
	} // already settled by this lease; never apply costs twice
	if !live {
		return service.ErrAcceptanceStale
	}
	if result.CostUSD != nil {
		_, err = tx.ExecContext(ctx, `UPDATE channel_acceptance_runs r SET reserved_cost_usd=r.reserved_cost_usd-s.estimated_cost_usd,spent_cost_usd=r.spent_cost_usd+$2::numeric FROM channel_acceptance_samples s WHERE r.id=s.run_id AND s.id=$1`, id, *result.CostUSD)
		if err != nil {
			return err
		}
	}
	// Unknown cost is not a zero invoice: keep the full existing reservation.
	_, err = tx.ExecContext(ctx, `UPDATE channel_acceptance_samples SET status=$2,result=$3,lease_until=NULL WHERE id=$1`, id, result.Status, body)
	if err != nil {
		return err
	}
	if err = refreshAcceptanceRun(ctx, tx, runID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *channelAcceptanceRepository) RecoverExpired(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		return service.ErrAcceptanceInvalid
	}
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	body, err := json.Marshal(&service.AcceptanceResult{Status: "unknown", Findings: []service.AcceptanceFinding{{Code: "lease_expired", Level: "warning", Message: "在途请求结果未知，保留预估费用且不自动重试"}}})
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `UPDATE channel_acceptance_samples SET status='unknown',result=$2,lease_token='',lease_until=NULL WHERE status='running' AND lease_until<=$1 RETURNING run_id`, now, body)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids[id] = true
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for id := range ids {
		if err = refreshAcceptanceRun(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Cleanup accepts the retention cutoff (the service passes now minus 30 days).
// Never remove outstanding requests, including cancelled but still-live work.
func (r *channelAcceptanceRepository) Cleanup(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return service.ErrAcceptanceInvalid
	}
	tx, err := r.schedulingTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `DELETE FROM channel_acceptance_runs r WHERE r.updated_at<$1 AND r.status IN ('completed','cancelled','budget_exhausted') AND NOT EXISTS(SELECT 1 FROM channel_acceptance_samples s WHERE s.run_id=r.id AND s.status IN ('pending','running'))`, cutoff)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM channel_acceptance_previews p WHERE p.expires_at<$1 AND NOT EXISTS(SELECT 1 FROM channel_acceptance_runs r WHERE r.preview_id=p.id)`, cutoff)
	if err != nil {
		return err
	}
	return tx.Commit()
}
