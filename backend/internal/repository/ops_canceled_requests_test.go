//go:build unit

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsCanceledRequestFilterIsExplicitAndUsesFinalAttempt(t *testing.T) {
	ordinary, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{View: "all"})
	require.NotContains(t, ordinary, "client_canceled", "provider health queries retain their existing semantics")

	var filter service.OpsErrorLogFilter
	require.NoError(t, json.Unmarshal([]byte(`{"IncludeCanceledRequests":true,"View":"all","UserID":42}`), &filter))
	where, args := buildOpsErrorLogsWhere(&filter)
	require.Contains(t, where, "e.upstream_errors -> -1", "only a final canceled attempt is a canceled request")
	require.Contains(t, where, "client_canceled")
	require.Contains(t, where, "COALESCE(e.status_code, 0) >= 400")
	require.Contains(t, where, "e.user_id = $1", "the cancellation exemption remains inside the caller's ownership scope")
	require.Equal(t, []any{int64(42)}, args)
}

func TestOpsCanceledOnlyFilterMatchesMixedFailureChains(t *testing.T) {
	var filter service.OpsErrorLogFilter
	require.NoError(t, json.Unmarshal([]byte(`{"IncludeCanceledRequests":true,"CanceledOnly":true}`), &filter))
	where, _ := buildOpsErrorLogsWhere(&filter)
	require.Contains(t, where, "client_canceled")
	require.NotContains(t, where, "e.error_type = ANY", "an earlier provider failure may own the stored top-level type")
}

func TestUserOpsFilterExcludesCanceledEvenWithLegacyErrorStatus(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{ExcludeCanceledRequests: true})
	require.Contains(t, where, "NOT ("+opsFinalClientCanceledExpr+")")
	require.Contains(t, where, "COALESCE(e.status_code, 0) >= 400")
}

func TestOpsCanceledRequestStatusFilterUsesDisplayedHTTPStatus(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{IncludeCanceledRequests: true, StatusCodes: []int{200}})
	require.Contains(t, where, "THEN "+opsCanceledDownstreamStatusExpr, "cancellation status filters must match the captured downstream HTTP status")
	order := opsErrorLogsOrderBy(&service.OpsErrorLogFilter{IncludeCanceledRequests: true, SortBy: "status_code"})
	require.Contains(t, order, "THEN "+opsCanceledDownstreamStatusExpr)
}

func TestOpsCanceledRequestsDoNotMatchAnotherDisplayedCategory(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{IncludeCanceledRequests: true, ErrorPhasesAny: []string{"upstream"}})
	require.Contains(t, where, "NOT ("+opsFinalClientCanceledExpr+")", "a canceled request is not displayed as an upstream error even when an earlier attempt failed")
}
