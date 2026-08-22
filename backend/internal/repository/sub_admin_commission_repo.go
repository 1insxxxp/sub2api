package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type subAdminCommissionRepository struct {
	client *dbent.Client
	db     *sql.DB
	sql    sqlExecutor
}

// NewSubAdminCommissionRepository creates the SQL-backed commission grant and report repository.
func NewSubAdminCommissionRepository(client *dbent.Client, sqlDB *sql.DB) service.SubAdminCommissionRepository {
	return &subAdminCommissionRepository{client: client, db: sqlDB, sql: sqlDB}
}

func (r *subAdminCommissionRepository) ListAllGrants(ctx context.Context) ([]service.SubAdminCommissionGrant, error) {
	return r.listGrants(ctx, `
		SELECT cg.id, cg.sub_admin_user_id, COALESCE(u.email, ''), cg.group_id, COALESCE(g.name, ''),
			cg.granted_date::text, cg.enabled, cg.created_at, cg.updated_at
		FROM sub_admin_commission_grants cg
		JOIN users u ON u.id = cg.sub_admin_user_id AND u.deleted_at IS NULL
		JOIN groups g ON g.id = cg.group_id
		WHERE cg.enabled = TRUE
		ORDER BY u.email ASC, g.name ASC, cg.id ASC
	`, nil)
}

func (r *subAdminCommissionRepository) ListGrantsForSubAdmin(ctx context.Context, subAdminID int64) ([]service.SubAdminCommissionGrant, error) {
	return r.listGrants(ctx, `
		SELECT cg.id, cg.sub_admin_user_id, COALESCE(u.email, ''), cg.group_id, COALESCE(g.name, ''),
			cg.granted_date::text, cg.enabled, cg.created_at, cg.updated_at
		FROM sub_admin_commission_grants cg
		JOIN users u ON u.id = cg.sub_admin_user_id AND u.deleted_at IS NULL
		JOIN groups g ON g.id = cg.group_id
		WHERE cg.sub_admin_user_id = $1 AND cg.enabled = TRUE
		ORDER BY g.name ASC, cg.id ASC
	`, []any{subAdminID})
}

func (r *subAdminCommissionRepository) ReplaceGrants(ctx context.Context, input service.ReplaceSubAdminCommissionGrantsInput, grantedDate string) ([]service.SubAdminCommissionGrant, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sub-admin commission repository requires *sql.DB")
	}
	grantedDate = strings.TrimSpace(grantedDate)
	if _, err := service.ParseGroupUsageDate(grantedDate); err != nil {
		return nil, fmt.Errorf("parse granted date: %w", err)
	}

	groupIDs := normalizePositiveInt64s(input.GroupIDs)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if len(groupIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE sub_admin_commission_grants
			SET enabled = FALSE, updated_at = NOW()
			WHERE sub_admin_user_id = $1 AND enabled = TRUE
		`, input.SubAdminID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE sub_admin_commission_grants
			SET enabled = FALSE, updated_at = NOW()
			WHERE sub_admin_user_id = $1 AND enabled = TRUE AND NOT (group_id = ANY($2))
		`, input.SubAdminID, pq.Array(groupIDs)); err != nil {
			return nil, err
		}

		createdBy := sql.NullInt64{Int64: input.OperatorID, Valid: input.OperatorID > 0}
		for _, groupID := range groupIDs {
			if err := upsertSubAdminCommissionGrant(ctx, tx, input.SubAdminID, groupID, grantedDate, createdBy); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return r.ListGrantsForSubAdmin(ctx, input.SubAdminID)
}

func upsertSubAdminCommissionGrant(ctx context.Context, tx *sql.Tx, subAdminID, groupID int64, grantedDate string, createdBy sql.NullInt64) error {
	var existingID int64
	err := scanSingleRow(ctx, tx, `
		UPDATE sub_admin_commission_grants
		SET updated_at = NOW()
		WHERE sub_admin_user_id = $1 AND group_id = $2 AND enabled = TRUE
		RETURNING id
	`, []any{subAdminID, groupID}, &existingID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	err = scanSingleRow(ctx, tx, `
		UPDATE sub_admin_commission_grants
		SET enabled = TRUE, granted_date = $3::date, created_by = $4, updated_at = NOW()
		WHERE id = (
			SELECT id
			FROM sub_admin_commission_grants
			WHERE sub_admin_user_id = $1 AND group_id = $2 AND enabled = FALSE
			ORDER BY id DESC
			LIMIT 1
		)
		RETURNING id
	`, []any{subAdminID, groupID, grantedDate, createdBy}, &existingID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sub_admin_commission_grants (sub_admin_user_id, group_id, granted_date, enabled, created_by)
		VALUES ($1, $2, $3::date, TRUE, $4)
	`, subAdminID, groupID, grantedDate, createdBy)
	return err
}

func (r *subAdminCommissionRepository) ListCalendar(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) ([]service.SubAdminCommissionCalendarDay, error) {
	if now.IsZero() {
		now = time.Now()
	}
	month = strings.TrimSpace(month)
	if month == "" {
		month = service.GroupUsageDate(now)[:7]
	}
	monthStart, err := service.ParseGroupUsageDate(month + "-01")
	if err != nil {
		return nil, err
	}
	monthEnd := monthStart.AddDate(0, 1, 0)
	todayStart, err := service.ParseGroupUsageDate(service.GroupUsageDate(now))
	if err != nil {
		return nil, err
	}

	grants, err := r.listEnabledGrantDates(ctx, subAdminID)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return []service.SubAdminCommissionCalendarDay{}, nil
	}

	earliestGrant := grants[0].grantedStart
	for _, grant := range grants[1:] {
		if grant.grantedStart.Before(earliestGrant) {
			earliestGrant = grant.grantedStart
		}
	}

	costs := make(map[string]float64)
	historicalEnd := minCommissionTime(monthEnd, todayStart)
	if historicalEnd.After(monthStart) {
		if err := r.loadCalendarRollupCosts(ctx, subAdminID, monthStart, historicalEnd, costs); err != nil {
			return nil, err
		}
	}
	if !todayStart.Before(monthStart) && todayStart.Before(monthEnd) {
		if err := r.loadCalendarLiveCosts(ctx, subAdminID, todayStart, costs); err != nil {
			return nil, err
		}
	}

	start := maxCommissionTime(monthStart, earliestGrant)
	end := minCommissionTime(monthEnd.AddDate(0, 0, -1), todayStart)
	if start.After(end) {
		return []service.SubAdminCommissionCalendarDay{}, nil
	}

	days := make([]service.SubAdminCommissionCalendarDay, 0, int(end.Sub(start).Hours()/24)+1)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		actualCost := costs[date]
		days = append(days, service.SubAdminCommissionCalendarDay{
			Date:             date,
			Enabled:          true,
			ActualCost:       actualCost,
			CommissionAmount: actualCost * commissionRate,
		})
	}
	return days, nil
}

func (r *subAdminCommissionRepository) ListDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]service.SubAdminCommissionDayGroup, error) {
	date = strings.TrimSpace(date)
	dayStart, err := service.ParseGroupUsageDate(strings.TrimSpace(date))
	if err != nil {
		return nil, err
	}
	todayStart, err := service.ParseGroupUsageDate(service.GroupUsageDate(time.Now()))
	if err != nil {
		return nil, err
	}
	if dayStart.Before(todayStart) {
		return r.listHistoricalDayGroups(ctx, subAdminID, date, commissionRate)
	}
	return r.listLiveDayGroups(ctx, subAdminID, date, commissionRate, dayStart)
}

func (r *subAdminCommissionRepository) listLiveDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64, dayStart time.Time) ([]service.SubAdminCommissionDayGroup, error) {
	dayEnd := dayStart.AddDate(0, 0, 1)

	rows, err := r.sql.QueryContext(ctx, `
		SELECT g.id, g.name, COUNT(ul.id) AS requests,
			COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0) AS total_tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS actual_cost
		FROM sub_admin_commission_grants cg
		JOIN groups g ON g.id = cg.group_id
		LEFT JOIN usage_logs ul ON ul.group_id = cg.group_id AND ul.created_at >= $2 AND ul.created_at < $3
		WHERE cg.sub_admin_user_id = $1 AND cg.enabled = TRUE AND $4::date >= cg.granted_date
		GROUP BY g.id, g.name
		ORDER BY g.name ASC, g.id ASC
	`, subAdminID, dayStart.UTC(), dayEnd.UTC(), strings.TrimSpace(date))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanSubAdminCommissionDayGroups(rows, commissionRate)
}

func (r *subAdminCommissionRepository) listHistoricalDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]service.SubAdminCommissionDayGroup, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT g.id, g.name, 0::bigint AS requests, 0::bigint AS total_tokens, COALESCE(r.actual_cost, 0) AS actual_cost
		FROM sub_admin_commission_grants cg
		JOIN groups g ON g.id = cg.group_id
		LEFT JOIN usage_group_daily_rollups r ON r.group_id = cg.group_id AND r.bucket_date = $2::date
		WHERE cg.sub_admin_user_id = $1 AND cg.enabled = TRUE AND $3::date >= cg.granted_date
		ORDER BY g.name ASC, g.id ASC
	`, subAdminID, date, date)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanSubAdminCommissionDayGroups(rows, commissionRate)
}

func scanSubAdminCommissionDayGroups(rows *sql.Rows, commissionRate float64) ([]service.SubAdminCommissionDayGroup, error) {
	groups := make([]service.SubAdminCommissionDayGroup, 0)
	for rows.Next() {
		var item service.SubAdminCommissionDayGroup
		if err := rows.Scan(&item.GroupID, &item.GroupName, &item.Requests, &item.TotalTokens, &item.ActualCost); err != nil {
			return nil, err
		}
		item.CommissionAmount = item.ActualCost * commissionRate
		groups = append(groups, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *subAdminCommissionRepository) ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]service.SubAdminCommissionUsageLog, pagination.PaginationResult, error) {
	date = strings.TrimSpace(date)
	dayStart, err := service.ParseGroupUsageDate(date)
	if err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	dayEnd := dayStart.AddDate(0, 0, 1)

	var allowed bool
	err = scanSingleRow(ctx, r.sql, `
		SELECT EXISTS (
			SELECT 1
			FROM sub_admin_commission_grants
			WHERE sub_admin_user_id = $1 AND group_id = $2 AND enabled = TRUE AND $3::date >= granted_date
		)
	`, []any{subAdminID, groupID, date}, &allowed)
	if err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	if !allowed {
		return nil, pagination.PaginationResult{}, service.ErrSubAdminCommissionForbidden
	}

	baseArgs := []any{subAdminID, groupID, dayStart.UTC(), dayEnd.UTC()}
	baseFrom := `
		FROM usage_logs ul
		JOIN sub_admin_commission_grants cg ON cg.group_id = ul.group_id AND cg.sub_admin_user_id = $1 AND cg.group_id = $2 AND cg.enabled = TRUE
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
		JOIN groups g ON g.id = ul.group_id
		WHERE ul.group_id = $2 AND ul.created_at >= $3 AND ul.created_at < $4
	`

	var total int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) "+baseFrom, baseArgs, &total); err != nil {
		return nil, pagination.PaginationResult{}, err
	}

	params = normalizePagination(params)
	listArgs := append(append([]any{}, baseArgs...), params.Limit(), params.Offset())
	rows, err := r.sql.QueryContext(ctx, `
		SELECT ul.id, ul.request_id, ul.created_at, ul.user_id, COALESCE(u.email, ''),
			ul.api_key_id, COALESCE(ak.name, ''), ul.group_id, g.name, ul.model, ul.requested_model,
			ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens, ul.actual_cost
		`+baseFrom+`
		ORDER BY ul.created_at DESC, ul.id DESC
		LIMIT $5 OFFSET $6
	`, listArgs...)
	if err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	defer func() { _ = rows.Close() }()

	logs := make([]service.SubAdminCommissionUsageLog, 0)
	for rows.Next() {
		var item service.SubAdminCommissionUsageLog
		var requestedModel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.RequestID, &item.CreatedAt, &item.UserID, &item.UserEmail,
			&item.APIKeyID, &item.APIKeyName, &item.GroupID, &item.GroupName, &item.Model, &requestedModel,
			&item.InputTokens, &item.OutputTokens, &item.CacheCreationTokens, &item.CacheReadTokens, &item.ActualCost,
		); err != nil {
			return nil, pagination.PaginationResult{}, err
		}
		if requestedModel.Valid {
			value := requestedModel.String
			item.RequestedModel = &value
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	return logs, *paginationResultFromTotal(total, params), nil
}

type subAdminCommissionGrantDate struct {
	groupID      int64
	grantedDate  string
	grantedStart time.Time
}

func (r *subAdminCommissionRepository) listEnabledGrantDates(ctx context.Context, subAdminID int64) ([]subAdminCommissionGrantDate, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT group_id, granted_date::text
		FROM sub_admin_commission_grants
		WHERE sub_admin_user_id = $1 AND enabled = TRUE
		ORDER BY granted_date ASC, group_id ASC
	`, subAdminID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	grants := make([]subAdminCommissionGrantDate, 0)
	for rows.Next() {
		var grant subAdminCommissionGrantDate
		if err := rows.Scan(&grant.groupID, &grant.grantedDate); err != nil {
			return nil, err
		}
		grantedStart, err := service.ParseGroupUsageDate(grant.grantedDate)
		if err != nil {
			return nil, err
		}
		grant.grantedStart = grantedStart
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grants, nil
}

func (r *subAdminCommissionRepository) loadCalendarRollupCosts(ctx context.Context, subAdminID int64, start, end time.Time, costs map[string]float64) error {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT r.bucket_date::text AS date, COALESCE(SUM(r.actual_cost), 0) AS actual_cost
		FROM usage_group_daily_rollups r
		JOIN sub_admin_commission_grants cg ON cg.group_id = r.group_id
		WHERE cg.sub_admin_user_id = $1 AND cg.enabled = TRUE
			AND r.bucket_date >= cg.granted_date
			AND r.bucket_date >= $2::date
			AND r.bucket_date < $3::date
		GROUP BY r.bucket_date
		ORDER BY r.bucket_date ASC
	`, subAdminID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	return scanCommissionCostRows(rows, costs)
}

func (r *subAdminCommissionRepository) loadCalendarLiveCosts(ctx context.Context, subAdminID int64, todayStart time.Time, costs map[string]float64) error {
	timezone := service.GroupUsageTimezoneName()
	rows, err := r.sql.QueryContext(ctx, `
		SELECT (ul.created_at AT TIME ZONE $4)::date::text AS date, COALESCE(SUM(ul.actual_cost), 0) AS actual_cost
		FROM usage_logs ul
		JOIN sub_admin_commission_grants cg ON cg.group_id = ul.group_id
		WHERE cg.sub_admin_user_id = $1 AND cg.enabled = TRUE
			AND (ul.created_at AT TIME ZONE $4)::date >= cg.granted_date
			AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY 1
	`, subAdminID, todayStart.UTC(), todayStart.AddDate(0, 0, 1).UTC(), timezone)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	return scanCommissionCostRows(rows, costs)
}

func scanCommissionCostRows(rows *sql.Rows, costs map[string]float64) error {
	for rows.Next() {
		var date string
		var actualCost float64
		if err := rows.Scan(&date, &actualCost); err != nil {
			return err
		}
		costs[date] += actualCost
	}
	return rows.Err()
}

func (r *subAdminCommissionRepository) listGrants(ctx context.Context, query string, args []any) ([]service.SubAdminCommissionGrant, error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	grants := make([]service.SubAdminCommissionGrant, 0)
	for rows.Next() {
		var grant service.SubAdminCommissionGrant
		if err := rows.Scan(
			&grant.ID, &grant.SubAdminID, &grant.SubAdminEmail, &grant.GroupID, &grant.GroupName,
			&grant.GrantedDate, &grant.Enabled, &grant.CreatedAt, &grant.UpdatedAt,
		); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grants, nil
}

func normalizePositiveInt64s(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func normalizePagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page < 1 {
		params.Page = 1
	}
	return params
}

func minCommissionTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxCommissionTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
