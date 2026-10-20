package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const commissionMonthlyGlobalGrants = `
 WITH global_grants AS (
  SELECT cg.group_id, MIN(cg.granted_date) AS granted_date
  FROM sub_admin_commission_grants cg
  JOIN users u ON u.id = cg.sub_admin_user_id AND u.deleted_at IS NULL AND u.role = 'sub_admin'
  WHERE cg.enabled = TRUE
  GROUP BY cg.group_id
 )
`

type commissionMonthlyGrant struct {
	groupID      int64
	groupName    string
	grantedStart time.Time
	active       bool
}

// ListMonthlyReport loads the month in at most three bulk queries. Historical
// days use rollups, while yesterday/today use bounded live logs so delayed
// rollup jobs and exclusion intervals have the same reporting semantics.
func (r *subAdminCommissionRepository) ListMonthlyReport(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) (service.SubAdminCommissionMonthlyReport, error) {
	report := service.SubAdminCommissionMonthlyReport{Days: []service.SubAdminCommissionCalendarDay{}, Groups: []service.SubAdminCommissionMonthlyGroup{}}
	if now.IsZero() {
		now = time.Now()
	}
	month = strings.TrimSpace(month)
	if month == "" {
		month = service.GroupUsageDate(now)[:7]
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return report, err
	}
	monthStart, err := service.ParseGroupUsageDate(month + "-01")
	if err != nil {
		return report, err
	}
	today, err := service.ParseGroupUsageDate(service.GroupUsageDate(now))
	if err != nil {
		return report, err
	}
	end := minCommissionTime(monthStart.AddDate(0, 1, 0), today.AddDate(0, 0, 1))
	if !monthStart.Before(end) {
		return report, nil
	}
	grants, err := r.listMonthlyCommissionGrants(ctx, end)
	if err != nil {
		return report, err
	}
	if len(grants) == 0 {
		return report, nil
	}
	start := end
	for _, grant := range grants {
		start = minCommissionTime(start, maxCommissionTime(monthStart, grant.grantedStart))
	}
	if !start.Before(end) {
		return report, nil
	}
	costs := make(map[int64]map[string]float64, len(grants))
	liveStart := today.AddDate(0, 0, -1)
	historicalEnd := minCommissionTime(end, liveStart)
	if start.Before(historicalEnd) {
		if err := r.loadMonthlyCommissionRollupCosts(ctx, start, historicalEnd, costs); err != nil {
			return report, err
		}
	}
	liveRangeStart := maxCommissionTime(start, liveStart)
	if liveRangeStart.Before(end) {
		if err := r.loadMonthlyCommissionLiveCosts(ctx, liveRangeStart, end, costs); err != nil {
			return report, err
		}
	}
	firstDay := end
	totals := make(map[string]service.SubAdminCommissionCalendarDay)
	for _, grant := range grants {
		groupCosts := costs[grant.groupID]
		if !grant.active && len(groupCosts) == 0 {
			continue
		}
		groupStart := maxCommissionTime(monthStart, grant.grantedStart)
		if !groupStart.Before(end) {
			continue
		}
		firstDay = minCommissionTime(firstDay, groupStart)
		group := service.SubAdminCommissionMonthlyGroup{GroupID: grant.groupID, GroupName: grant.groupName, Days: []service.SubAdminCommissionCalendarDay{}}
		for day := groupStart; day.Before(end); day = day.AddDate(0, 0, 1) {
			date := day.Format("2006-01-02")
			cost := groupCosts[date]
			entry := service.SubAdminCommissionCalendarDay{Date: date, Enabled: true, ActualCost: cost, CommissionAmount: cost * commissionRate}
			group.Days = append(group.Days, entry)
			total := totals[date]
			total.Date = date
			total.Enabled = true
			total.ActualCost += entry.ActualCost
			total.CommissionAmount += entry.CommissionAmount
			totals[date] = total
		}
		report.Groups = append(report.Groups, group)
	}
	for day := firstDay; day.Before(end); day = day.AddDate(0, 0, 1) {
		report.Days = append(report.Days, totals[day.Format("2006-01-02")])
	}
	return report, nil
}

func (r *subAdminCommissionRepository) listMonthlyCommissionGrants(ctx context.Context, end time.Time) ([]commissionMonthlyGrant, error) {
	rows, err := r.sql.QueryContext(ctx, commissionMonthlyGlobalGrants+`
  SELECT g.id, g.name, gg.granted_date::text,
   (g.status = 'active' AND g.deleted_at IS NULL) AS active
  FROM global_grants gg
  JOIN groups g ON g.id = gg.group_id
  WHERE gg.granted_date < $1::date
  ORDER BY g.name ASC, g.id ASC
 `, end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	grants := make([]commissionMonthlyGrant, 0)
	for rows.Next() {
		var grant commissionMonthlyGrant
		var date string
		if err := rows.Scan(&grant.groupID, &grant.groupName, &date, &grant.active); err != nil {
			return nil, err
		}
		grant.grantedStart, err = service.ParseGroupUsageDate(date)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (r *subAdminCommissionRepository) loadMonthlyCommissionRollupCosts(ctx context.Context, start, end time.Time, costs map[int64]map[string]float64) error {
	rows, err := r.sql.QueryContext(ctx, commissionMonthlyGlobalGrants+`
  SELECT r.group_id, r.bucket_date::text, COALESCE(SUM(r.actual_cost - r.commission_excluded_cost), 0) AS actual_cost
  FROM usage_group_daily_rollups r
  JOIN global_grants gg ON gg.group_id = r.group_id
  WHERE r.bucket_date >= gg.granted_date
   AND r.bucket_date >= $1::date AND r.bucket_date < $2::date
  GROUP BY r.group_id, r.bucket_date
  ORDER BY r.group_id, r.bucket_date
 `, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	return scanMonthlyCommissionCostRows(rows, costs)
}

func (r *subAdminCommissionRepository) loadMonthlyCommissionLiveCosts(ctx context.Context, start, end time.Time, costs map[int64]map[string]float64) error {
	rows, err := r.sql.QueryContext(ctx, commissionMonthlyGlobalGrants+`
  SELECT ul.group_id, (ul.created_at AT TIME ZONE $3)::date::text AS date,
   COALESCE(SUM(ul.actual_cost), 0) AS actual_cost
  FROM usage_logs ul
  JOIN global_grants gg ON gg.group_id = ul.group_id
  WHERE (ul.created_at AT TIME ZONE $3)::date >= gg.granted_date
   AND ul.created_at >= $1 AND ul.created_at < $2
   AND NOT EXISTS (
    SELECT 1 FROM sub_admin_commission_exclusions ce
    WHERE ce.user_id = ul.user_id AND ce.started_at <= ul.created_at
     AND (ce.ended_at IS NULL OR ul.created_at < ce.ended_at)
   )
  GROUP BY ul.group_id, 2
  ORDER BY ul.group_id, 2
 `, start.UTC(), end.UTC(), service.GroupUsageTimezoneName())
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	return scanMonthlyCommissionCostRows(rows, costs)
}

func scanMonthlyCommissionCostRows(rows *sql.Rows, costs map[int64]map[string]float64) error {
	for rows.Next() {
		var id int64
		var date string
		var cost float64
		if err := rows.Scan(&id, &date, &cost); err != nil {
			return err
		}
		if costs[id] == nil {
			costs[id] = make(map[string]float64)
		}
		costs[id][date] += cost
	}
	return rows.Err()
}
