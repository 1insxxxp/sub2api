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

func TestOpsBilledFailureIsNotAlsoSuccess(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	u := mustCreateUser(t, client, &service.User{})
	k := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID})
	a := mustCreateAccount(t, client, &service.Account{Name: "billed-failure", Platform: service.PlatformGemini})
	g := mustCreateGroup(t, client, &service.Group{Name: "billed-failure", Platform: service.PlatformGemini})
	start := time.Date(2021, 2, 8, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	requestID := fmt.Sprintf("client:billed-failure-%d", u.ID)
	var usageID, errorID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs
 (user_id,api_key_id,account_id,group_id,request_id,model,input_tokens,output_tokens,actual_cost,stream,request_type,duration_ms,created_at)
 VALUES ($1,$2,$3,$4,$5,'gemini-test',33782,5748,1.56305,true,2,132305,$6) RETURNING id`, u.ID, k.ID, a.ID, g.ID, requestID, start).Scan(&usageID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO ops_error_logs
 (user_id,api_key_id,account_id,group_id,request_id,client_request_id,platform,model,status_code,error_phase,error_type,error_message,created_at)
 VALUES ($1,$2,$3,$4,'gateway-failure',$5,'gemini','gemini-test',400,'upstream','api_error','content blocked',$6) RETURNING id`, u.ID, k.ID, a.ID, g.ID, requestID[7:], start).Scan(&errorID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_response_outcomes
 (usage_log_id,request_id,user_id,api_key_id,account_id,group_id,http_status,upstream_status,has_reasoning,stream_completed,finish_reason,disconnect_source,upstream_error_kind,created_at)
 VALUES ($1,$2,$3,$4,$5,$6,200,200,true,true,'prohibited_content','upstream','other',$7)`, usageID, requestID, u.ID, k.ID, a.ID, g.ID, start)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_metrics_hourly WHERE bucket_start=$1", start)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM channel_monitor_v2_metrics_1m WHERE group_id=$1", g.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", a.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id=$1", g.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id=$1", u.ID)
	})
	repo := NewOpsRepository(integrationDB).(*opsRepository)
	filter := &service.OpsDashboardFilter{StartTime: start, EndTime: end, GroupID: &g.ID}
	t.Run("request details have one final error", func(t *testing.T) {
		items, total, err := repo.ListRequestDetails(ctx, &service.OpsRequestDetailFilter{StartTime: &start, EndTime: &end, UserID: &u.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, items, 1)
		require.Equal(t, service.OpsRequestKindError, items[0].Kind)
		require.Equal(t, &errorID, items[0].ErrorID)
	})
	t.Run("raw success excludes billed failure and preserves tokens", func(t *testing.T) {
		success, tokens, err := repo.queryUsageCounts(ctx, filter, start, end)
		require.NoError(t, err)
		require.Zero(t, success)
		require.EqualValues(t, 39530, tokens)
	})
	t.Run("hourly success excludes billed failure and preserves tokens", func(t *testing.T) {
		require.NoError(t, repo.UpsertHourlyMetrics(ctx, start, end))
		var success, tokens int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT success_count,token_consumed FROM ops_metrics_hourly WHERE bucket_start=$1 AND group_id=$2", start, g.ID).Scan(&success, &tokens))
		require.Zero(t, success)
		require.EqualValues(t, 39530, tokens)
	})
	t.Run("channel success excludes billed failure", func(t *testing.T) {
		_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(channelMonitorV2UsageMetricsSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL), start, end)
		require.NoError(t, err)
		var success int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT success_requests FROM channel_monitor_v2_metrics_1m WHERE bucket_start=$1 AND group_id=$2", start, g.ID).Scan(&success))
		require.Zero(t, success)
		var tokens int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT input_tokens+output_tokens FROM channel_monitor_v2_metrics_1m WHERE bucket_start=$1 AND group_id=$2", start, g.ID).Scan(&tokens))
		require.EqualValues(t, 39530, tokens)
		_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(channelMonitorV2UserMetricsSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL), start, end)
		require.NoError(t, err)
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT success_requests FROM channel_monitor_v2_user_metrics_1m WHERE bucket_start=$1 AND group_id=$2 AND user_id=$3", start, g.ID, u.ID).Scan(&success))
		require.Zero(t, success)
		_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(channelMonitorV2HistogramSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL, channelMonitorV2HistogramBoundSQL("latency.value_ms")), start, end)
		require.NoError(t, err)
		var samples int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM channel_monitor_v2_latency_histograms_1m WHERE bucket_start=$1 AND group_id=$2", start, g.ID).Scan(&samples))
		require.Zero(t, samples)
	})
	t.Run("trends and latency exclude duplicate success", func(t *testing.T) {
		trend, err := repo.GetThroughputTrend(ctx, filter, 60)
		require.NoError(t, err)
		require.Len(t, trend.Points, 1)
		require.EqualValues(t, 1, trend.Points[0].RequestCount)
		require.EqualValues(t, 39530, trend.Points[0].TokenConsumed)
		platforms, err := repo.getThroughputBreakdownByPlatform(ctx, start, end)
		require.NoError(t, err)
		require.Len(t, platforms, 1)
		require.EqualValues(t, 1, platforms[0].RequestCount)
		groups, err := repo.getThroughputTopGroupsByPlatform(ctx, start, end, service.PlatformGemini, 10)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		require.EqualValues(t, 1, groups[0].RequestCount)
		duration, _, samples, err := repo.queryUsageLatency(ctx, filter, start, end)
		require.NoError(t, err)
		require.Nil(t, duration.Avg)
		require.Zero(t, samples)
		_, err = repo.GetRealtimeTrafficSummary(ctx, filter)
		require.NoError(t, err)
		_, _, err = repo.queryPeakRates(ctx, filter, start, end)
		require.NoError(t, err)
	})
	t.Run("billing remains intact", func(t *testing.T) {
		var count int
		var charged float64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*),SUM(actual_cost) FROM usage_logs WHERE user_id=$1", u.ID).Scan(&count, &charged))
		require.Equal(t, 1, count)
		require.InDelta(t, 1.56305, charged, 0.0000001)
	})
}

func TestUsageRequestSuccessPreservesFreeRecoveryAndIdentity(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	u := mustCreateUser(t, client, &service.User{})
	k := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID})
	otherKey := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID})
	a := mustCreateAccount(t, client, &service.Account{Name: "request-outcome-boundaries", Platform: service.PlatformGemini})
	start := time.Date(2021, 2, 9, 10, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id=$1", u.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", a.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id=$1", u.ID)
	})
	for _, tc := range []struct {
		name, prefix, finish, disconnect string
		cost                             float64
		httpStatus, errorStatus          int
		noOutcome, otherKey, otherNS     bool
		want                             bool
	}{
		{name: "free completed", httpStatus: 200, want: true},
		{name: "recovered retry diagnostic", httpStatus: 200, errorStatus: 200, want: true},
		{name: "final error overrides stale outcome", cost: 1, httpStatus: 200, errorStatus: 400},
		{name: "other key same id", httpStatus: 200, errorStatus: 400, otherKey: true, want: true},
		{name: "client id not local id", httpStatus: 200, errorStatus: 400, otherNS: true, want: true},
		{name: "local id not client id", prefix: "local:", httpStatus: 200, errorStatus: 400, otherNS: true, want: true},
		{name: "local final error", prefix: "local:", cost: 1, httpStatus: 200, errorStatus: 500},
		{name: "legacy final error", prefix: "legacy", cost: 1, noOutcome: true, errorStatus: 400},
		{name: "legacy billed success", cost: 1, noOutcome: true, want: true},
		{name: "paid http failure", cost: 1, httpStatus: 500},
		{name: "paid policy terminal", cost: 1, httpStatus: 200, finish: "prohibited_content"},
		{name: "client interrupted", cost: 1, httpStatus: 200, disconnect: "client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := fmt.Sprintf("boundary-%d-%d", u.ID, time.Now().UnixNano())
			prefix := tc.prefix
			if prefix == "" {
				prefix = "client:"
			}
			requestID := prefix + id
			if prefix == "legacy" {
				requestID = id
			}
			var usageID int64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs
 (user_id,api_key_id,account_id,request_id,model,actual_cost,created_at)
 VALUES ($1,$2,$3,$4,'test',$5,$6) RETURNING id`, u.ID, k.ID, a.ID, requestID, tc.cost, start).Scan(&usageID))
			if !tc.noOutcome {
				disconnect := tc.disconnect
				if disconnect == "" {
					disconnect = "none"
				}
				_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_response_outcomes
 (usage_log_id,request_id,user_id,api_key_id,account_id,http_status,upstream_status,has_text,stream_completed,finish_reason,disconnect_source)
 VALUES ($1,$2,$3,$4,$5,$6,200,true,true,$7,$8)`, usageID, requestID, u.ID, k.ID, a.ID, tc.httpStatus, tc.finish, disconnect)
				require.NoError(t, err)
			}
			if tc.errorStatus > 0 {
				keyID := k.ID
				if tc.otherKey {
					keyID = otherKey.ID
				}
				isClient := prefix == "client:"
				if tc.otherNS {
					isClient = !isClient
				}
				errorRequestID, clientRequestID := id, ""
				if isClient {
					errorRequestID, clientRequestID = "gateway-"+id, id
				}
				_, err := integrationDB.ExecContext(ctx, `INSERT INTO ops_error_logs
 (user_id,api_key_id,request_id,client_request_id,status_code,error_phase,error_type,error_message,created_at)
 VALUES ($1,$2,$3,$4,$5,'upstream','api_error','diagnostic',$6)`, u.ID, keyID, errorRequestID, clientRequestID, tc.errorStatus, start)
				require.NoError(t, err)
			}
			var succeeded bool
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT "+usageRequestSuccessFilterUL+" FROM usage_logs ul WHERE ul.id=$1", usageID).Scan(&succeeded))
			require.Equal(t, tc.want, succeeded)
		})
	}
}
