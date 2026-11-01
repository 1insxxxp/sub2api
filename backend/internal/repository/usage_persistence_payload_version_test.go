//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsagePersistencePayloadVersion2WriterAndReader(t *testing.T) {
	log := &service.UsageLog{RequestID: "versioned", APIKeyID: 2, UserID: 1, AccountID: 3, CreatedAt: time.Now().UTC(), InputTokensSource: service.UsageTokenSourceUnreported, OutputTokensSource: service.UsageTokenSourceEstimated}
	raw, err := encodeUsagePersistencePayload(&service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: 2, UserID: 1, AccountID: 3, UsageLog: log}, 0)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.EqualValues(t, 2, payload["version"], "ordinary snapshots retain compatibility with deployed v2 workers")
	got, err := decodeUsagePersistencePayload(raw, log.RequestID, log.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, service.UsageTokenSourceUnreported, got.InputTokensSource)
	require.Equal(t, service.UsageTokenSourceEstimated, got.OutputTokensSource)
}

func TestUsagePersistencePayloadWriterScopesVersion3ToInputCountingAudit(t *testing.T) {
	zero := 0
	for _, tc := range []struct {
		name    string
		outcome *service.ResponseOutcome
		version int
	}{
		{"no outcome", nil, 2},
		{"ordinary outcome", &service.ResponseOutcome{CollectorVersion: 1}, 2},
		{"raw reported zero", &service.ResponseOutcome{CollectorVersion: 1, UpstreamInputTokens: &zero}, 3},
		{"policy", &service.ResponseOutcome{CollectorVersion: 1, InputTokenPolicy: "vertex_like_discount_v1"}, 3},
		{"policy count zero", &service.ResponseOutcome{CollectorVersion: 1, InputTokenPolicyCount: &zero}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := &service.UsageLog{RequestID: "version-scope", APIKeyID: 2, UserID: 1, AccountID: 3, CreatedAt: time.Now().UTC(), Outcome: tc.outcome}
			raw, err := encodeUsagePersistencePayload(&service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: 2, UserID: 1, AccountID: 3, UsageLog: log}, 0)
			require.NoError(t, err)
			var snapshot usagePersistencePayload
			require.NoError(t, json.Unmarshal(raw, &snapshot))
			require.Equal(t, tc.version, snapshot.Version)
			restored, err := decodeUsagePersistencePayload(raw, log.RequestID, log.APIKeyID)
			require.NoError(t, err)
			require.Equal(t, log.Outcome, restored.Outcome)
		})
	}
}

func TestUsagePersistencePayloadVersion2KeepsProvenance(t *testing.T) {
	raw := []byte(`{"version":2,"usage":{"request_id":"legacy-v2","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-10-26T00:00:00Z","input_tokens":0,"output_tokens":7,"input_tokens_source":"upstream","output_tokens_source":"estimated","outcome":{"collector_version":1}}}`)
	got, err := decodeUsagePersistencePayload(raw, "legacy-v2", 2)
	require.NoError(t, err)
	require.Equal(t, service.UsageTokenSourceUpstream, got.InputTokensSource)
	require.Equal(t, service.UsageTokenSourceEstimated, got.OutputTokensSource)
	require.Nil(t, got.Outcome.UpstreamInputTokens)
	require.Empty(t, got.Outcome.InputTokenPolicy)
	require.Nil(t, got.Outcome.InputTokenPolicyCount)
}

func TestUsagePersistencePayloadVersion1KeepsUnknownProvenance(t *testing.T) {
	raw := []byte(`{"version":1,"usage":{"request_id":"legacy-v1","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-10-26T00:00:00Z","input_tokens":0,"output_tokens":7}}`)
	got, err := decodeUsagePersistencePayload(raw, "legacy-v1", 2)
	require.NoError(t, err)
	require.Equal(t, 0, got.InputTokens)
	require.Equal(t, 7, got.OutputTokens)
	require.Equal(t, service.UsageTokenSourceUnknown, got.InputTokensSource)
	require.Equal(t, service.UsageTokenSourceUnknown, got.OutputTokensSource)
}

func TestUsagePersistencePayloadVersion1RejectsNewMetadata(t *testing.T) {
	for _, key := range []string{"input_tokens_source", "output_tokens_source"} {
		for _, value := range []string{`"upstream"`, `""`, `null`} {
			t.Run(key+"/"+value, func(t *testing.T) {
				raw := []byte(fmt.Sprintf(`{"version":1,"usage":{"request_id":"legacy-v1","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-10-26T00:00:00Z",%q:%s}}`, key, value))
				_, err := decodeUsagePersistencePayload(raw, "legacy-v1", 2)
				require.Error(t, err, "v1 cannot carry fields the deployed v1 decoder rejects")
			})
		}
	}
}

func TestUsagePersistencePayloadUnsupportedVersionIsNotAcknowledged(t *testing.T) {
	for _, version := range []int{0, 4, 99} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"version":%d,"usage":{"request_id":"future","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-10-26T00:00:00Z"}}`, version))
			_, err := decodeUsagePersistencePayload(raw, "future", 2)
			require.Error(t, err)
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT request_id,api_key_id,payload,claimed_by FROM usage_persistence_outbox`).WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"request_id", "api_key_id", "payload", "claimed_by"}).AddRow("future", 2, raw, "worker"))
			mock.ExpectRollback()
			require.Error(t, NewUsagePersistenceOutboxRepository(nil, db).Deliver(context.Background(), 99, "worker"))
			require.NoError(t, mock.ExpectationsWereMet(), "unsupported snapshots must remain durable; no INSERT usage_logs or DELETE outbox")
		})
	}
}
