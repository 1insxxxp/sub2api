//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestStreamPhaseTimingDurableRoundTripAndHydration(t *testing.T) {
	zero, event, text := 0, 25, 130
	log := &service.UsageLog{RequestID: "stream-phases", UserID: 1, APIKeyID: 2, AccountID: 3, Outcome: &service.ResponseOutcome{
		CollectorVersion: 1, UpstreamFirstEventMs: &event, UpstreamFirstReasoningMs: &zero, UpstreamFirstTextMs: &text,
	}}
	raw, err := encodeUsagePersistencePayload(&service.UsageBillingCommand{RequestID: log.RequestID, UserID: 1, APIKeyID: 2, AccountID: 3, UsageLog: log}, 0)
	require.NoError(t, err)
	var payload struct {
		Version int `json:"version"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, 6, payload.Version)
	replay, err := decodeUsagePersistencePayload(raw, log.RequestID, log.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, log.Outcome.UpstreamFirstEventMs, replay.Outcome.UpstreamFirstEventMs)
	require.Equal(t, log.Outcome.UpstreamFirstReasoningMs, replay.Outcome.UpstreamFirstReasoningMs)
	require.Equal(t, log.Outcome.UpstreamFirstTextMs, replay.Outcome.UpstreamFirstTextMs)
	require.Nil(t, replay.Outcome.DownstreamFirstContentMs)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec(`(?s)INSERT INTO usage_response_outcomes.*upstream_first_event_ms.*COALESCE\(EXCLUDED.upstream_first_event_ms, usage_response_outcomes.upstream_first_event_ms\)`).WithArgs(
		log.RequestID, int64(2), int64(1), int64(3), nil,
		0, 0, false, false, false, false, int64(0), 0, false, "", "none", "none", int16(1), 0, "unknown", "unknown", nil, "", nil, 25, 0, 130, nil,
	).WillReturnResult(sqlmock.NewResult(1, 1))
	repo := newUsageLogRepositoryWithSQL(nil, db)
	require.NoError(t, repo.UpsertResponseOutcome(context.Background(), replay))
	mock.ExpectQuery(`SELECT ul.id, erc.id`).WithArgs("{42,43}").WillReturnRows(sqlmock.NewRows([]string{
		"usage_id", "claim_id", "claim_status", "reason_code", "http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version", "anti_truncation_rounds", "input_tokens_source", "output_tokens_source", "upstream_input_tokens", "input_token_policy", "input_token_policy_count", "upstream_first_event_ms", "upstream_first_reasoning_ms", "upstream_first_text_ms", "downstream_first_content_ms",
	}).AddRow(42, nil, nil, nil, 200, 200, true, false, false, false, 1, 1, true, "stop", "none", "none", 1, 0, "unknown", "unknown", nil, "", nil, 25, 0, 130, nil).
		AddRow(43, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	logs := []service.UsageLog{{ID: 42}, {ID: 43}}
	require.NoError(t, repo.hydrateUsageLogCompensation(context.Background(), logs))
	require.Equal(t, 0, *logs[0].Outcome.UpstreamFirstReasoningMs)
	require.Equal(t, 130, *logs[0].Outcome.UpstreamFirstTextMs)
	require.Nil(t, logs[0].Outcome.DownstreamFirstContentMs)
	require.Nil(t, logs[1].Outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}
