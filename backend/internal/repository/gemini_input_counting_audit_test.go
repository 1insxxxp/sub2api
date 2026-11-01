//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func inputCountingAuditSnapshot(version int, fields string) []byte {
	return []byte(fmt.Sprintf(`{"version":%d,"usage":{"request_id":"audit-input","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-11-01T00:00:00Z","input_tokens":667,"outcome":{"collector_version":1,%s}}}`, version, fields))
}

func TestUsagePersistenceInputCountingAuditRoundTripPreservesReportedZeroAndUnknown(t *testing.T) {
	for _, count := range []string{"1037", "0", "null"} {
		t.Run(count, func(t *testing.T) {
			candidate := 667
			if count == "0" {
				candidate = 0
			}
			raw := inputCountingAuditSnapshot(3, fmt.Sprintf(`"upstream_input_tokens":%s,"input_token_policy":"vertex_like_discount_v1","input_token_policy_count":%d`, count, candidate))
			log, err := decodeUsagePersistencePayload(raw, "audit-input", 2)
			require.NoError(t, err)
			require.NotNil(t, log.Outcome)
			outcome := reflect.ValueOf(log.Outcome).Elem()
			rawInput := outcome.FieldByName("UpstreamInputTokens")
			require.True(t, rawInput.IsValid(), "the audit value must survive the durable domain projection")
			if count == "null" {
				require.True(t, rawInput.IsNil())
			} else {
				require.False(t, rawInput.IsNil())
				require.Equal(t, count, fmt.Sprint(rawInput.Elem().Int()))
			}
			require.Equal(t, "vertex_like_discount_v1", outcome.FieldByName("InputTokenPolicy").String())
			require.EqualValues(t, candidate, outcome.FieldByName("InputTokenPolicyCount").Elem().Int())
			encoded, err := encodeUsagePersistencePayload(&service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: 2, UserID: 1, AccountID: 3, UsageLog: log}, 0)
			require.NoError(t, err)
			var snapshot usagePersistencePayload
			require.NoError(t, json.Unmarshal(encoded, &snapshot))
			require.Equal(t, 3, snapshot.Version)
			restored, err := decodeUsagePersistencePayload(encoded, "audit-input", 2)
			require.NoError(t, err)
			require.Equal(t, log.Outcome, restored.Outcome)
			publicJSON, err := json.Marshal(log.Outcome)
			require.NoError(t, err)
			for _, internal := range []string{"upstream_input_tokens", "input_token_policy", "input_token_policy_count", "vertex_like_discount_v1"} {
				require.NotContains(t, string(publicJSON), internal, "ordinary response evidence must not expose internal accounting")
			}
		})
	}
}

func TestUsagePersistenceInputCountingAuditRetainsStrictLegacyContracts(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, field := range []string{"upstream_input_tokens", "input_token_policy", "input_token_policy_count", strings.ToUpper("upstream_input_tokens"), strings.ToUpper("input_token_policy"), strings.ToUpper("input_token_policy_count")} {
			for _, value := range []string{`null`, `0`, `""`} {
				t.Run(fmt.Sprintf("v%d/%s/%s", version, field, value), func(t *testing.T) {
					_, err := decodeUsagePersistencePayload(inputCountingAuditSnapshot(version, fmt.Sprintf("%q:%s", field, value)), "audit-input", 2)
					require.Error(t, err, "new audit keys require v3, including null and empty values")
				})
			}
		}
	}
	_, err := decodeUsagePersistencePayload(inputCountingAuditSnapshot(3, `"upstream_url":"https://private.invalid"`), "audit-input", 2)
	require.Error(t, err, "v3 must not relax strict unknown-field rejection")
}

func TestUsageLogRepositoryUpsertInputCountingAuditPreservesIdentity(t *testing.T) {
	log, err := decodeUsagePersistencePayload(inputCountingAuditSnapshot(3, `"upstream_input_tokens":1037,"input_token_policy":"vertex_like_discount_v1","input_token_policy_count":667`), "audit-input", 2)
	require.NoError(t, err)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec(`(?s)INSERT INTO usage_response_outcomes.*upstream_input_tokens, input_token_policy, input_token_policy_count.*ON CONFLICT \(request_id, api_key_id\) DO UPDATE`).WithArgs(
		"audit-input", int64(2), int64(1), int64(3), nil,
		0, 0, false, false, false, false, int64(0), 0, false, "", "none", "none", int16(1), 0, "unknown", "unknown",
		1037, "vertex_like_discount_v1", 667,
	).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, newUsageLogRepositoryWithSQL(nil, db).upsertResponseOutcome(context.Background(), log))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryHydratesInputCountingAuditWithoutPublicProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT ul.id, erc.id, erc.status, erc.reason_code.*upstream_input_tokens.*input_token_policy.*input_token_policy_count`).WithArgs("{42,43}").WillReturnRows(sqlmock.NewRows([]string{
		"usage_id", "claim_id", "claim_status", "reason_code", "http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version", "anti_truncation_rounds", "input_tokens_source", "output_tokens_source", "upstream_input_tokens", "input_token_policy", "input_token_policy_count",
	}).AddRow(42, nil, nil, nil, 200, 200, true, false, false, false, 10, 2, true, "stop", "none", "none", 1, 0, "estimated", "upstream", 1037, "vertex_like_discount_v1", 667).AddRow(43, nil, nil, nil, 200, 200, true, false, false, false, 10, 2, true, "stop", "none", "none", 1, 0, "unknown", "unknown", nil, "", nil))
	logs := []service.UsageLog{{ID: 42}, {ID: 43}}
	require.NoError(t, newUsageLogRepositoryWithSQL(nil, db).hydrateUsageLogCompensation(context.Background(), logs))
	require.NotNil(t, logs[0].Outcome)
	require.Equal(t, 1037, *logs[0].Outcome.UpstreamInputTokens)
	require.Equal(t, "vertex_like_discount_v1", logs[0].Outcome.InputTokenPolicy)
	require.Equal(t, 667, *logs[0].Outcome.InputTokenPolicyCount)
	require.NotNil(t, logs[1].Outcome)
	require.Nil(t, logs[1].Outcome.UpstreamInputTokens)
	require.Empty(t, logs[1].Outcome.InputTokenPolicy)
	require.Nil(t, logs[1].Outcome.InputTokenPolicyCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryInputCountingAuditOnlyKeepsUnknownOutcome(t *testing.T) {
	rawInput, candidate := 1037, 667
	log := &service.UsageLog{RequestID: "audit-only", APIKeyID: 2, UserID: 1, AccountID: 3,
		Outcome: &service.ResponseOutcome{UpstreamInputTokens: &rawInput, InputTokenPolicy: "vertex_like_discount_v1", InputTokenPolicyCount: &candidate}}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectExec(`(?s)INSERT INTO usage_response_outcomes.*upstream_input_tokens`).WithArgs(
		"audit-only", int64(2), int64(1), int64(3), nil,
		0, 0, false, false, false, false, int64(0), 0, false, "", "none", "none", int16(0), 0, "unknown", "unknown",
		1037, "vertex_like_discount_v1", 667,
	).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.UpsertResponseOutcome(context.Background(), log), "accounting audit must not invent collector evidence")
	mock.ExpectQuery(`SELECT ul.id, erc.id, erc.status, erc.reason_code`).WithArgs("{42}").WillReturnRows(sqlmock.NewRows([]string{
		"usage_id", "claim_id", "claim_status", "reason_code", "http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version", "anti_truncation_rounds", "input_tokens_source", "output_tokens_source", "upstream_input_tokens", "input_token_policy", "input_token_policy_count",
	}).AddRow(42, nil, nil, nil, 0, 0, false, false, false, false, 0, 0, false, "", "none", "none", 0, 0, "unknown", "unknown", 1037, "vertex_like_discount_v1", 667))
	read := []service.UsageLog{{ID: 42}}
	require.NoError(t, repo.hydrateUsageLogCompensation(context.Background(), read))
	require.NotNil(t, read[0].Outcome, "internal audit stays readable without implying response collection")
	require.Zero(t, read[0].Outcome.CollectorVersion)
	require.Equal(t, rawInput, *read[0].Outcome.UpstreamInputTokens)
	require.Equal(t, service.UsageRequestUnknown, service.ClassifyUsageRequestStatus(read[0].Outcome))
	require.NoError(t, mock.ExpectationsWereMet())
}
