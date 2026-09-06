//go:build unit

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelStatusRepositoryReturnsErrorsAndSkipsEmptyScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewModelStatusRepository(db)
	end := time.Now()
	rows, err := repo.Aggregate(context.Background(), end, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
	mock.ExpectBegin()
	mock.ExpectQuery("WITH").WillReturnError(errors.New("query unavailable"))
	mock.ExpectRollback()
	rows, err = repo.Aggregate(context.Background(), end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.ErrorContains(t, err, "query unavailable")
	require.Nil(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelStatusPostgresDeduplicatesTerminalFailuresAndAggregatesLast30Requests(t *testing.T) {
	db := modelStatusTestPostgres(t)
	ctx := context.Background()
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(ctx, `
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false), (2,'openai','active',false), (3,'openai','active',true), (4,'composite','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, model, created_at, output_tokens, first_token_ms, duration_ms)
SELECT 1, 11, 'traffic-' || n, 'shared', 'mapped-private', TIMESTAMPTZ '2026-09-06 11:55:00Z' + n * INTERVAL '1 millisecond', 5, 10, 100 FROM generate_series(1,2501) n;
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, model, created_at, output_tokens) VALUES
 (1,11,'terminal','shared','mapped-private','2026-09-06 11:56:00Z',7),
 (1,11,'recovered','shared','mapped-private','2026-09-06 11:56:01Z',7),
 (1,11,'cancelled','shared','mapped-private','2026-09-06 11:56:02Z',7),
 (1,11,'empty','shared','mapped-private','2026-09-06 11:56:03Z',99),
 (1,11,'unknown','shared','mapped-private','2026-09-06 11:56:04Z',0),
 (1,12,'terminal','shared','mapped-private','2026-09-06 11:56:05Z',7),
 (2,22,'group-two','shared','mapped-private','2026-09-06 11:56:06Z',7),
 (3,33,'private','shared','mapped-private','2026-09-06 11:56:07Z',7),
 (1,11,'hidden-model','hidden','mapped-private','2026-09-06 11:56:08Z',7),
 (4,44,'composite','shared','mapped-private','2026-09-06 11:56:09Z',7),
 (1,11,'old','shared','mapped-private','2026-09-06 11:44:59.999Z',7),
 (1,11,'future','shared','mapped-private','2026-09-06 12:00:00Z',7),
 (1,11,'','shared','mapped-private','2026-09-06 11:56:10Z',7),
 (1,11,'','shared','mapped-private','2026-09-06 11:56:11Z',7);
INSERT INTO usage_response_outcomes (usage_log_id, http_status, upstream_status, has_text, disconnect_source)
 SELECT id,200,200,false,'none' FROM usage_logs WHERE request_id='empty';
INSERT INTO usage_response_outcomes (usage_log_id, http_status, upstream_status, has_text, disconnect_source)
 SELECT id,200,200,true,'client' FROM usage_logs WHERE request_id='cancelled';
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, model, created_at, status_code, error_type, duration_ms) VALUES
 (1,11,'terminal','shared','mapped-private','2026-09-06 11:56:20Z',502,'upstream_error',200),
 (1,11,'terminal','shared','mapped-private','2026-09-06 11:56:21Z',502,'upstream_error',250),
 (1,11,'recovered','shared','mapped-private','2026-09-06 11:56:22Z',200,'upstream_error',200),
 (1,11,'cancelled','shared','mapped-private','2026-09-06 11:56:23Z',500,'upstream_error',200),
 (1,11,'missing-usage','shared','mapped-private','2026-09-06 11:56:24Z',503,'upstream_error',300),
 (1,11,'client-cancel','shared','mapped-private','2026-09-06 11:56:25Z',499,'upstream_error',400),
 (1,11,'','shared','mapped-private','2026-09-06 11:56:26Z',500,'upstream_error',500);
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code, is_count_tokens)
 VALUES (1,11,'count-tokens','shared','2026-09-06 11:56:27Z',500,true);
`)
	require.NoError(t, err)
	scopes := []service.ModelStatusScope{
		{GroupID: 1, Platform: "openai", Model: "shared"},
		{GroupID: 2, Platform: "openai", Model: "shared"},
		{GroupID: 3, Platform: "openai", Model: "shared"},
		{GroupID: 4, Platform: "composite", Model: "shared"},
	}
	rows, err := NewModelStatusRepository(db).Aggregate(ctx, end, scopes)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	byGroup := map[int64]service.ModelStatusAggregate{}
	for _, row := range rows {
		byGroup[row.GroupID] = row
	}
	metrics := byGroup[1].Metrics
	require.Equal(t, int64(30), metrics.Total)
	require.Equal(t, int64(23), metrics.Success)
	require.Equal(t, int64(3), metrics.Failure)
	require.Equal(t, int64(1), metrics.Empty)
	require.Equal(t, int64(3), metrics.Unknown)
	require.Equal(t, metrics.Total, metrics.Success+metrics.Failure+metrics.Empty+metrics.Unknown)
	require.Equal(t, int64(19), metrics.TTFTSamples)
	require.Equal(t, int64(19), metrics.DurationSamples)
	require.InDelta(t, 100, *metrics.AvgDurationMs, 0.0001)
	require.InDelta(t, 10, *metrics.AvgTTFTMs, 0.0001)
	require.Len(t, byGroup[1].Recent, 30)
	for i, recent := range byGroup[1].Recent {
		require.True(t, recent.At.Before(end))
		if i > 0 {
			require.False(t, recent.At.Before(byGroup[1].Recent[i-1].At))
		}
	}
	require.Equal(t, service.UsageOutcomeFailure, byGroup[1].Recent[29].Outcome)
	require.Equal(t, int64(1), byGroup[2].Metrics.Success)
	require.Equal(t, int64(1), byGroup[4].Metrics.Success)
}

func TestModelStatusPostgresClassifiesEffectiveOutputWithoutCostOrStreamCompletionGuesses(t *testing.T) {
	db := modelStatusTestPostgres(t)
	ctx := context.Background()
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(ctx, `
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens, image_output_tokens, image_count, stream) VALUES
 (1,1,'free','shared','2026-09-06 11:59:00Z',5,0,0,false),
 (1,1,'image-tokens','shared','2026-09-06 11:59:01Z',0,5,0,false),
 (1,1,'image-count','shared','2026-09-06 11:59:02Z',0,0,1,false),
 (1,1,'claim','shared','2026-09-06 11:59:03Z',0,0,0,false),
 (1,1,'missing-stream-end','shared','2026-09-06 11:59:04Z',0,0,0,true),
 (1,1,'tool','shared','2026-09-06 11:59:05Z',0,0,0,false),
 (1,1,'protocol','shared','2026-09-06 11:59:06Z',9,0,0,true);
INSERT INTO empty_response_claims (usage_log_id,reason_code) SELECT id,'effective_output' FROM usage_logs WHERE request_id='claim';
INSERT INTO usage_response_outcomes (usage_log_id,http_status,upstream_status,has_text,stream_completed)
 SELECT id,200,200,false,false FROM usage_logs WHERE request_id='missing-stream-end';
INSERT INTO usage_response_outcomes (usage_log_id,http_status,upstream_status,has_tool_call)
 SELECT id,200,200,true FROM usage_logs WHERE request_id='tool';
INSERT INTO usage_response_outcomes (usage_log_id,http_status,upstream_status,has_text,upstream_error_kind)
 SELECT id,200,200,true,'protocol' FROM usage_logs WHERE request_id='protocol';
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(ctx, end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(5), rows[0].Metrics.Success)
	require.Equal(t, int64(1), rows[0].Metrics.Failure)
	require.Equal(t, int64(1), rows[0].Metrics.Unknown)
	require.Zero(t, rows[0].Metrics.Empty)
}

func TestModelStatusPostgresCorrelatesGatewayBillingPrefixesWithOpsIDs(t *testing.T) {
	db := modelStatusTestPostgres(t)
	ctx := context.Background()
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(ctx, `
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens) VALUES
 (1,1,'local:server-id','shared','2026-09-06 11:59:00Z',5),
 (1,1,'client:client-id','shared','2026-09-06 11:59:01Z',5);
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, client_request_id, requested_model, created_at, status_code) VALUES
 (1,1,'server-id','','shared','2026-09-06 11:59:02Z',502),
 (1,1,'another-server-id','client-id','shared','2026-09-06 11:59:03Z',502);
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(ctx, end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Metrics.Total)
	require.Equal(t, int64(2), rows[0].Metrics.Failure)
	require.Zero(t, rows[0].Metrics.Success)
}

func TestModelStatusPostgresCorrelatesEvidenceAcrossSnapshotBoundary(t *testing.T) {
	db := modelStatusTestPostgres(t)
	ctx := context.Background()
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(ctx, `
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens) VALUES
 (1,1,'local:failure-before','shared','2026-09-06 11:45:00.001Z',5),
 (1,1,'local:failure-after','shared','2026-09-06 11:59:59.999Z',5),
 (1,1,'client:cancel-before','shared','2026-09-06 11:44:59.999Z',5),
 (1,1,'client:cancel-after','shared','2026-09-06 12:00:00.001Z',5);
INSERT INTO usage_response_outcomes (usage_log_id,http_status,disconnect_source)
 SELECT id,200,'client' FROM usage_logs WHERE request_id LIKE 'client:%';
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, client_request_id, requested_model, created_at, status_code) VALUES
 (1,1,'failure-before','','shared','2026-09-06 11:44:59.999Z',502),
 (1,1,'failure-after','','shared','2026-09-06 12:00:00.001Z',502),
 (1,1,'server-before','cancel-before','shared','2026-09-06 11:45:00.001Z',502),
 (1,1,'server-after','cancel-after','shared','2026-09-06 11:59:59.999Z',502);
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(ctx, end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(3), rows[0].Metrics.Total)
	require.Equal(t, int64(2), rows[0].Metrics.Unknown)
	require.Zero(t, rows[0].Metrics.Success)
	require.Equal(t, int64(1), rows[0].Metrics.Failure)
	require.Len(t, rows[0].Recent, 3)
	require.WithinDuration(t, end.Add(-15*time.Minute-time.Millisecond), rows[0].Recent[0].At, 0)
	require.WithinDuration(t, end.Add(-15*time.Minute+time.Millisecond), rows[0].Recent[1].At, 0)
	require.WithinDuration(t, end.Add(-time.Millisecond), rows[0].Recent[2].At, 0)
}

func TestModelStatusPostgresRecentRequestsHaveNoTimeLowerBound(t *testing.T) {
	db := modelStatusTestPostgres(t)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens, first_token_ms, duration_ms) VALUES
 (1,1,'old','shared','2025-01-01 00:00:00Z',5,10,100),
 (1,1,'recent','shared','2026-09-06 11:59:00Z',5,30,300),
 (1,1,'at-end','shared','2026-09-06 12:00:00Z',5,1000,1000);
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(context.Background(), end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Metrics.Total)
	require.Equal(t, int64(2), rows[0].Metrics.Success)
	require.InDelta(t, 20, *rows[0].Metrics.AvgTTFTMs, 0.0001)
	require.InDelta(t, 200, *rows[0].Metrics.AvgDurationMs, 0.0001)
	require.Len(t, rows[0].Recent, 2)
	require.Equal(t, 2025, rows[0].Recent[0].At.Year())
}

func TestModelStatusPostgresExpandsCandidatesPastDuplicateErrors(t *testing.T) {
	db := modelStatusTestPostgres(t)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code)
 SELECT 1,1,'old-' || n,'shared',TIMESTAMPTZ '2026-09-06 08:00:00Z' + n * INTERVAL '1 second',502 FROM generate_series(1,40) n;
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code)
 SELECT 1,1,'duplicated','shared',TIMESTAMPTZ '2026-09-06 11:59:00Z' + n * INTERVAL '1 millisecond',502 FROM generate_series(1,130) n;
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(context.Background(), end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(30), rows[0].Metrics.Total)
	require.Equal(t, int64(30), rows[0].Metrics.Failure)
	require.Len(t, rows[0].Recent, 30)
	require.Equal(t, time.Date(2026, 9, 6, 8, 0, 12, 0, time.UTC), rows[0].Recent[0].At.UTC())
	require.Equal(t, time.Date(2026, 9, 6, 11, 59, 0, 130000000, time.UTC), rows[0].Recent[29].At.UTC())
}

func TestModelStatusPostgresExpandsWhenTerminalTimestampPrecedesCandidateBoundary(t *testing.T) {
	db := modelStatusTestPostgres(t)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens, first_token_ms, duration_ms)
 SELECT 1,1,'ok-' || n,'shared',TIMESTAMPTZ '2026-09-06 10:00:00Z' + n * INTERVAL '1 second',5,10,100 FROM generate_series(1,40) n;
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens)
 SELECT 1,1,'late-' || n,'shared',TIMESTAMPTZ '2026-09-06 11:59:00Z' + n * INTERVAL '1 millisecond',5 FROM generate_series(1,40) n;
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code)
 SELECT 1,1,'late-' || n,'shared',TIMESTAMPTZ '2026-09-06 09:00:00Z' + n * INTERVAL '1 second',502 FROM generate_series(1,40) n;
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(context.Background(), end, []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(30), rows[0].Metrics.Total)
	require.Equal(t, int64(30), rows[0].Metrics.Success)
	require.Equal(t, int64(30), rows[0].Metrics.DurationSamples)
	require.Zero(t, rows[0].Metrics.Failure)
	require.InDelta(t, 100, *rows[0].Metrics.AvgDurationMs, 0.0001)
	require.Equal(t, time.Date(2026, 9, 6, 10, 0, 11, 0, time.UTC), rows[0].Recent[0].At.UTC())
}

func TestModelStatusPostgresUsesStableIdentityOrderForTiedTimes(t *testing.T) {
	db := modelStatusTestPostgres(t)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens)
 SELECT 1,1,'tie-' || LPAD(n::text,2,'0'),'shared','2026-09-06 11:59:00Z',5 FROM generate_series(40,1,-1) n;
INSERT INTO usage_response_outcomes (usage_log_id,http_status,has_text)
 SELECT id,200,false FROM usage_logs WHERE request_id='tie-40';
`)
	require.NoError(t, err)
	repo := NewModelStatusRepository(db)
	scopes := []service.ModelStatusScope{{GroupID: 1, Platform: "openai", Model: "shared"}}
	rows, err := repo.Aggregate(context.Background(), end, scopes)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(30), rows[0].Metrics.Total)
	require.Equal(t, int64(29), rows[0].Metrics.Success)
	require.Equal(t, int64(1), rows[0].Metrics.Empty)
	require.Equal(t, service.UsageOutcomeEmpty, rows[0].Recent[29].Outcome)
	again, err := repo.Aggregate(context.Background(), end, scopes)
	require.NoError(t, err)
	require.Equal(t, rows, again)
}

func TestModelStatusPostgresKeepsRequestIdentitiesWithinTheirPublicScope(t *testing.T) {
	db := modelStatusTestPostgres(t)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false),(2,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens) VALUES
 (1,1,'shared-id','shared','2026-09-06 11:00:00Z',5),
 (2,1,'local:shared-id','shared','2026-09-06 11:00:01Z',5);
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code)
 SELECT 2,1,'shared-id','shared',TIMESTAMPTZ '2026-09-06 11:59:00Z' + n * INTERVAL '1 millisecond',502 FROM generate_series(1,70) n;
`)
	require.NoError(t, err)
	rows, err := NewModelStatusRepository(db).Aggregate(context.Background(), end, []service.ModelStatusScope{
		{GroupID: 1, Platform: "openai", Model: "shared"}, {GroupID: 2, Platform: "openai", Model: "shared"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, int64(1), row.Metrics.Total)
		require.Len(t, row.Recent, 1)
		if row.GroupID == 1 {
			require.Equal(t, int64(1), row.Metrics.Success)
		} else {
			require.Equal(t, int64(1), row.Metrics.Failure)
		}
	}
}

func TestModelStatusPostgresCandidateReadsUseScopedIndexes(t *testing.T) {
	db := modelStatusTestPostgres(t)
	_, err := db.Exec(`
INSERT INTO groups (id, platform, status, is_exclusive) VALUES (1,'openai','active',false);
INSERT INTO usage_logs (group_id, api_key_id, request_id, requested_model, created_at, output_tokens)
 SELECT 1,1,'usage-' || n,CASE WHEN n % 100=0 THEN 'shared' ELSE 'other' END,
 TIMESTAMPTZ '2026-09-01 00:00:00Z' + n * INTERVAL '1 second',5 FROM generate_series(1,10000) n;
INSERT INTO ops_error_logs (group_id, api_key_id, request_id, requested_model, created_at, status_code)
 SELECT 1,1,'error-' || n,CASE WHEN n % 100=0 THEN 'shared' ELSE 'other' END,
 TIMESTAMPTZ '2026-09-01 00:00:00Z' + n * INTERVAL '1 second',502 FROM generate_series(1,10000) n;
CREATE INDEX test_usage_identity ON usage_logs (request_id, api_key_id);
CREATE INDEX test_error_identity ON ops_error_logs (request_id);
CREATE INDEX test_error_client_identity ON ops_error_logs (client_request_id);
`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/241_add_model_status_recent_indexes_notx.sql")
	require.NoError(t, err)
	_, err = db.Exec(strings.ReplaceAll(string(migration), "CONCURRENTLY ", ""))
	require.NoError(t, err)
	_, err = db.Exec("ANALYZE usage_logs; ANALYZE ops_error_logs; ANALYZE groups")
	require.NoError(t, err)
	end := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	var raw []byte
	err = db.QueryRow("EXPLAIN (ANALYZE, FORMAT JSON) "+modelStatusAggregateSQL,
		`[{"group_id":1,"platform":"openai","model":"shared"}]`, end, 30, 30).Scan(&raw)
	require.NoError(t, err)
	var plan any
	require.NoError(t, json.Unmarshal(raw, &plan))
	used := map[string]bool{}
	var inspect func(any)
	inspect = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			if name, ok := node["Index Name"].(string); ok {
				used[name] = true
			}
			if node["Relation Name"] == "usage_logs" || node["Relation Name"] == "ops_error_logs" {
				require.NotEqual(t, "Seq Scan", node["Node Type"], "evidence lookup must not scan history")
			}
			for _, child := range node {
				inspect(child)
			}
		case []any:
			for _, child := range node {
				inspect(child)
			}
		}
	}
	inspect(plan)
	require.True(t, used["idx_usage_logs_model_status_recent"])
	require.True(t, used["idx_ops_error_logs_model_status_recent"])
}

func modelStatusTestPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MODEL_STATUS_TEST_DSN")
	if dsn == "" {
		t.Skip("MODEL_STATUS_TEST_DSN is required for PostgreSQL integration")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
CREATE TEMP TABLE groups (id bigint PRIMARY KEY, platform text, status text, is_exclusive boolean, deleted_at timestamptz);
CREATE TEMP TABLE usage_logs (
 id bigserial PRIMARY KEY, group_id bigint, api_key_id bigint, request_id text, requested_model text, model text,
 created_at timestamptz, output_tokens integer DEFAULT 0, image_output_tokens integer DEFAULT 0, image_count integer DEFAULT 0,
 first_token_ms bigint, duration_ms bigint, request_type smallint DEFAULT 0, stream boolean DEFAULT false);
CREATE TEMP TABLE usage_response_outcomes (
 id bigserial PRIMARY KEY, usage_log_id bigint UNIQUE, http_status integer DEFAULT 0, upstream_status integer DEFAULT 0,
 has_text boolean DEFAULT false, has_tool_call boolean DEFAULT false, has_reasoning boolean DEFAULT false, has_media boolean DEFAULT false,
 stream_completed boolean DEFAULT false, disconnect_source text DEFAULT 'none', upstream_error_kind text DEFAULT 'none');
CREATE TEMP TABLE empty_response_claims (usage_log_id bigint UNIQUE, reason_code text);
CREATE TEMP TABLE ops_error_logs (
 id bigserial PRIMARY KEY, group_id bigint, api_key_id bigint, request_id text, client_request_id text, requested_model text, model text,
 created_at timestamptz, status_code integer, error_type text, is_count_tokens boolean DEFAULT false,
 time_to_first_token_ms bigint, duration_ms bigint);
`)
	require.NoError(t, err)
	return db
}
