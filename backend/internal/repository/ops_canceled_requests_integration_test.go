//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsCanceledRequestHistoryPreservesFinalAttemptSemantics(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB).(*opsRepository)
	prefix := fmt.Sprintf("cancel-audit-%d", time.Now().UnixNano())
	cases := []struct {
		name, typ, events            string
		status, upstream, downstream int
		canceled, alwaysVisible      bool
	}{
		{"cancel", "client_cancel", `[{"stage":"inference","kind":"client_cancel","attempt":1,"client_canceled":true,"downstream_status_code":200}]`, 200, 200, 200, true, false},
		{"failure-then-cancel", "upstream_error", `[{"stage":"inference","kind":"failover","attempt":1,"upstream_status_code":503},{"stage":"inference","kind":"client_cancel","attempt":2,"client_canceled":true,"downstream_status_code":200}]`, 200, 503, 200, true, false},
		{"cancel-then-completed", "client_cancel", `[{"stage":"inference","kind":"client_cancel","attempt":1,"client_canceled":true},{"stage":"inference","kind":"success","attempt":2}]`, 200, 200, 200, false, false},
		{"recovered", "upstream_error", `[{"stage":"inference","kind":"failover","attempt":1,"upstream_status_code":503}]`, 200, 503, 200, false, false},
		{"non-inference", "upstream_error", `[{"stage":"account_auth","attempt":1,"client_canceled":true}]`, 200, 503, 200, false, false},
		{"same-attempt-error-cancel", "upstream_error", `[{"stage":"inference","kind":"request_error","attempt":1,"client_canceled":true,"downstream_status_code":200}]`, 200, 504, 200, true, false},
		{"legacy-cancel", "client_cancel", `null`, 200, 200, 0, true, false},
		{"null-diagnostics", "upstream_error", `null`, 200, 503, 200, false, false},
		{"cancel-before-headers", "client_cancel", `[{"stage":"inference","kind":"client_cancel","client_canceled":true,"downstream_status_code":499}]`, 200, 200, 499, true, false},
		{"malformed-status", "client_cancel", `[{"stage":"inference","kind":"client_cancel","client_canceled":true,"downstream_status_code":"not-an-integer"}]`, 200, 200, 0, true, false},
		{"out-of-range-status", "client_cancel", `[{"stage":"inference","kind":"client_cancel","client_canceled":true,"downstream_status_code":999}]`, 200, 200, 0, true, false},
		{"failed-request", "upstream_error", `[]`, 502, 502, 502, false, true},
	}
	ids := make(map[string]int64)
	for _, tc := range cases {
		var id int64
		err := integrationDB.QueryRowContext(ctx, `
			INSERT INTO ops_error_logs (request_id, error_phase, error_type, severity, status_code, upstream_status_code, upstream_errors)
			VALUES ($1, 'upstream', $2, 'P3', $3, $4, $5::jsonb) RETURNING id`,
			prefix+"-"+tc.name, tc.typ, tc.status, tc.upstream, tc.events).Scan(&id)
		require.NoError(t, err)
		ids[tc.name] = id
		t.Cleanup(func() {
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM ops_error_logs WHERE id = $1", id)
		})
	}
	for _, include := range []bool{false, true} {
		list, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all", Query: prefix, IncludeCanceledRequests: include, PageSize: 100})
		require.NoError(t, err)
		got := make(map[int64]*service.OpsErrorLog)
		for _, row := range list.Errors {
			got[row.ID] = row
		}
		expectedCount := 0
		for _, tc := range cases {
			id := ids[tc.name]
			if tc.alwaysVisible || include && tc.canceled {
				expectedCount++
				require.Contains(t, got, id, tc.name)
				require.Equal(t, tc.canceled, got[id].ClientCanceled, tc.name)
				require.Equal(t, tc.downstream, got[id].DownstreamStatusCode, tc.name)
				require.Equal(t, tc.upstream, got[id].StatusCode, "provider status stays intact")
			} else {
				require.NotContains(t, got, id, tc.name)
			}
		}
		require.Equal(t, expectedCount, list.Total, "pagination count must use the same cancellation predicate")
	}
	for _, tc := range cases {
		detail, err := repo.GetErrorLogByID(ctx, ids[tc.name])
		require.NoError(t, err)
		require.Equal(t, tc.canceled, detail.ClientCanceled, tc.name)
		require.Equal(t, tc.downstream, detail.DownstreamStatusCode, tc.name)
	}
	only, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all", Query: prefix, IncludeCanceledRequests: true, CanceledOnly: true, PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, 7, only.Total)
	byStatus, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all", Query: prefix, IncludeCanceledRequests: true, StatusCodes: []int{200}, SortBy: "status_code", PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, 3, byStatus.Total, "canceled requests with a previous provider 503 are displayed and filtered using their real HTTP 200")
	unknownStatus, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all", Query: prefix, IncludeCanceledRequests: true, StatusCodes: []int{0}, PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, 3, unknownStatus.Total, "historical or malformed cancellation evidence must not invent a real HTTP status")
	upstreamOnly, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all", Query: prefix, IncludeCanceledRequests: true, ErrorPhasesAny: []string{"upstream"}, PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, 1, upstreamOnly.Total, "final cancellations have their own displayed category")
}
