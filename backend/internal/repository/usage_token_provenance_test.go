//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageTokenProvenanceMetadataOnlyPersistedWithoutResponseEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	log := &service.UsageLog{RequestID: "metadata-only", APIKeyID: 2, UserID: 1, AccountID: 3}
	require.NoError(t, json.Unmarshal([]byte(`{"InputTokensSource":"upstream","OutputTokensSource":"unreported"}`), log))
	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectExec(`(?s)INSERT INTO usage_response_outcomes.*input_tokens_source.*output_tokens_source`).WithArgs(
		log.RequestID, log.APIKeyID, log.UserID, log.AccountID, log.GroupID,
		0, 0, false, false, false, false, int64(0), 0, false, "", "none", "none", int16(0), 0, "upstream", "unreported",
	).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.UpsertResponseOutcome(context.Background(), log))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageTokenProvenanceHydratesWithoutPhantomOutcome(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`(?s)SELECT ul.id, erc.id, erc.status, erc.reason_code.*input_tokens_source.*output_tokens_source`).WithArgs("{42,43}").WillReturnRows(sqlmock.NewRows([]string{
		"usage_id", "claim_id", "claim_status", "reason_code", "http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version", "anti_truncation_rounds", "input_tokens_source", "output_tokens_source",
	}).AddRow(42, nil, nil, nil, 0, 0, false, false, false, false, 0, 0, false, "", "none", "none", 0, 0, "upstream", "unreported").AddRow(43, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	logs := []service.UsageLog{{ID: 42}, {ID: 43}}
	require.NoError(t, repo.hydrateUsageLogCompensation(context.Background(), logs))
	require.Nil(t, logs[0].Outcome)
	require.Nil(t, logs[1].Outcome)
	raw, err := json.Marshal(logs)
	require.NoError(t, err)
	var fields []map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.Equal(t, "upstream", fields[0]["InputTokensSource"])
	require.Equal(t, "unreported", fields[0]["OutputTokensSource"])
	require.Equal(t, "unknown", fields[1]["InputTokensSource"])
	require.Equal(t, "unknown", fields[1]["OutputTokensSource"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageTokenProvenanceSurvivesDurableSnapshot(t *testing.T) {
	log := &service.UsageLog{RequestID: "source-roundtrip", APIKeyID: 2, UserID: 1, AccountID: 3, CreatedAt: time.Now().UTC().Round(0)}
	require.NoError(t, json.Unmarshal([]byte(`{"InputTokensSource":"unreported","OutputTokensSource":"estimated"}`), log))
	raw, err := encodeUsagePersistencePayload(&service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: 2, UserID: 1, AccountID: 3, UsageLog: log}, 0)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	usage := payload["usage"].(map[string]any)
	require.Equal(t, "unreported", usage["input_tokens_source"])
	require.Equal(t, "estimated", usage["output_tokens_source"])
	replay, err := decodeUsagePersistencePayload(raw, log.RequestID, log.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, log, replay)
}
