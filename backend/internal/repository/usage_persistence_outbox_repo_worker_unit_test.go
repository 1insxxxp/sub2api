//go:build unit

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func usagePersistenceWorkerPayload(t *testing.T) ([]byte, *service.UsageLog) {
	t.Helper()
	log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: "durable-worker", Model: "gemini-2.5-flash", InputTokens: 12, OutputTokens: 2, UpstreamOutputTokens: 10000, ActualCost: 2, RateMultiplier: 1, CreatedAt: time.Date(2026, 10, 23, 2, 3, 4, 0, time.UTC),
		Outcome: &service.ResponseOutcome{HTTPStatus: 200, HasText: true, OutputBytes: 14, CollectorVersion: 1}}
	cmd := &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
	raw, err := encodeUsagePersistencePayload(cmd, 0.5)
	require.NoError(t, err)
	return raw, log
}
func expectUsagePersistenceOwnedRow(mock sqlmock.Sqlmock, raw []byte, worker any) {
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT request_id,api_key_id,payload,claimed_by FROM usage_persistence_outbox WHERE id=\$1 FOR UPDATE`).WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "api_key_id", "payload", "claimed_by"}).AddRow("durable-worker", 2, raw, worker))
}
func expectUsagePersistenceInsert(mock sqlmock.Sqlmock, created time.Time) {
	args := make([]driver.Value, len(usageLogInsertArgTypes))
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	mock.ExpectQuery(`(?s)INSERT INTO usage_logs.*ON CONFLICT \(request_id, api_key_id\) DO NOTHING.*RETURNING id, created_at`).WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(42, created))
}

func TestUsagePersistenceOutboxClaim_LeasesBoundedBatchWithSkipLocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	created := time.Now().UTC()
	mock.ExpectQuery(`(?s)WITH candidates AS.*available_at<=NOW\(\).*claimed_at.*FOR UPDATE SKIP LOCKED.*UPDATE usage_persistence_outbox.*claimed_by=\$1.*RETURNING o.id,o.attempts,o.created_at`).WithArgs("replica-a", 100, int64(60)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "attempts", "created_at"}).AddRow(99, 3, created))
	events, err := NewUsagePersistenceOutboxRepository(nil, db).Claim(context.Background(), "replica-a", 999, time.Minute)
	require.NoError(t, err)
	require.Equal(t, []service.UsagePersistenceEvent{{ID: 99, Attempts: 3, CreatedAt: created}}, events)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsagePersistenceOutboxDeliver_UsageOutcomeAndAckShareOneTransaction(t *testing.T) {
	for _, failOutcome := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic commit", true: "atomic rollback"}[failOutcome], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			raw, log := usagePersistenceWorkerPayload(t)
			expectUsagePersistenceOwnedRow(mock, raw, "replica-a")
			expectUsagePersistenceInsert(mock, log.CreatedAt)
			outcome := mock.ExpectExec(`(?s)INSERT INTO usage_response_outcomes.*FROM usage_logs ul`)
			if failOutcome {
				outcome.WillReturnError(errors.New("injected outcome failure"))
				mock.ExpectRollback()
			} else {
				outcome.WillReturnResult(sqlmock.NewResult(42, 1))
				mock.ExpectExec(`DELETE FROM usage_persistence_outbox WHERE id=\$1 AND claimed_by=\$2`).WithArgs(int64(99), "replica-a").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = NewUsagePersistenceOutboxRepository(nil, db).Deliver(context.Background(), 99, "replica-a")
			if failOutcome {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet(), "only usage/outcome/ack may execute, never wallet/quota/billing mutations")
		})
	}
}

func TestUsagePersistenceOutboxDeliver_ExistingHistoryIsNeverOverwritten(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	raw, log := usagePersistenceWorkerPayload(t)
	expectUsagePersistenceOwnedRow(mock, raw, "replica-a")
	mock.ExpectQuery(`(?s)INSERT INTO usage_logs`).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT id, created_at, threshold_exempt_cost FROM usage_logs`).WithArgs(log.RequestID, log.APIKeyID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "threshold_exempt_cost"}).AddRow(42, log.CreatedAt, 0.125))
	mock.ExpectExec(`DELETE FROM usage_persistence_outbox`).WithArgs(int64(99), "replica-a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, NewUsagePersistenceOutboxRepository(nil, db).Deliver(context.Background(), 99, "replica-a"))
	require.NoError(t, mock.ExpectationsWereMet(), "no UPDATE usage_logs or response outcome upsert is permitted for prior history")
}

func TestUsagePersistenceOutboxDeliver_OwnerAndMalformedPayloadFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     any
		malformed bool
	}{{"other worker", "replica-b", false}, {"unclaimed", nil, false}, {"corrupt payload", "replica-a", true}} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			raw, _ := usagePersistenceWorkerPayload(t)
			if tc.malformed {
				raw = []byte(`{"version":9}`)
			}
			expectUsagePersistenceOwnedRow(mock, raw, tc.owner)
			mock.ExpectRollback()
			require.Error(t, NewUsagePersistenceOutboxRepository(nil, db).Deliver(context.Background(), 99, "replica-a"))
			require.NoError(t, mock.ExpectationsWereMet(), "must retain the event without touching usage on invalid ownership/payload")
		})
	}
}

func TestUsagePersistenceOutboxDeliver_AcknowledgedEventIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT request_id,api_key_id,payload,claimed_by FROM usage_persistence_outbox`).WithArgs(int64(99)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	require.NoError(t, NewUsagePersistenceOutboxRepository(nil, db).Deliver(context.Background(), 99, "replica-a"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsagePersistenceOutboxRetry_PreservesTaskForeverAndRedactsRawErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	available := time.Now().Add(time.Minute)
	mock.ExpectExec(`(?s)UPDATE usage_persistence_outbox.*attempts=CASE.*available_at=\$3,last_error=\$4,claimed_at=NULL,claimed_by=NULL.*WHERE id=\$1 AND claimed_by=\$2`).
		WithArgs(int64(99), "replica-a", available, "usage_persistence_delivery_failed").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, NewUsagePersistenceOutboxRepository(nil, db).RetryClaimed(context.Background(), 99, "replica-a", available, "duplicate row contains customer@example.com sk-secret"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsagePersistencePayload_RoundTripsEveryInsertFieldWithoutDomainRelations(t *testing.T) {
	// Fill all explicit snapshot fields, then compare real database insert args.
	// This catches a missing DTO field even when an ordinary public JSON tag hides it.
	domain := &service.UsageLog{}
	target := reflect.ValueOf(domain).Elem()
	dto := reflect.TypeFor[usagePersistenceUsage]()
	require.Equal(t, len(usageLogInsertArgTypes)+2, dto.NumField(), "only insert columns plus outcome and continuation rounds")
	for i := 0; i < dto.NumField(); i++ {
		name := dto.Field(i).Name
		if name == "Outcome" {
			continue
		}
		field := target.FieldByName(name)
		switch field.Kind() {
		case reflect.String:
			field.SetString("snapshot-" + name)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(7)
		case reflect.Float64:
			field.SetFloat(1.25)
		case reflect.Bool:
			field.SetBool(true)
		case reflect.Pointer:
			value := reflect.New(field.Type().Elem())
			switch value.Elem().Kind() {
			case reflect.String:
				value.Elem().SetString("snapshot-" + name)
			case reflect.Int, reflect.Int64:
				value.Elem().SetInt(9)
			case reflect.Float64:
				value.Elem().SetFloat(1.5)
			case reflect.Bool:
				value.Elem().SetBool(true)
			}
			field.Set(value)
		case reflect.Map:
			field.Set(reflect.ValueOf(map[string]int{"1024x1024": 2}))
		case reflect.Struct:
			field.Set(reflect.ValueOf(time.Date(2026, 10, 23, 1, 2, 3, 123000, time.UTC)))
		default:
			t.Fatalf("unhandled snapshot field %s", name)
		}
	}
	domain.RequestType = service.RequestTypeStream
	domain.Outcome = &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, HasToolCall: true, HasReasoning: true, HasMedia: true, OutputBytes: 33, EventCount: 3, StreamCompleted: true, FinishReason: "stop", DisconnectSource: service.DisconnectSourceNone, UpstreamErrorKind: service.UpstreamErrorNone, CollectorVersion: 1}
	domain.User = &service.User{Email: "do-not-persist@example.invalid"}
	domain.APIKey = &service.APIKey{Key: "do-not-persist-key"}
	domain.Account = &service.Account{Credentials: map[string]any{"credential": "do-not-persist-credential"}}
	cmd := &service.UsageBillingCommand{RequestID: domain.RequestID, APIKeyID: domain.APIKeyID, UserID: domain.UserID, AccountID: domain.AccountID, UsageLog: domain}
	raw, err := encodeUsagePersistencePayload(cmd, domain.ThresholdExemptCost)
	require.NoError(t, err)
	restored, err := decodeUsagePersistencePayload(raw, domain.RequestID, domain.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, prepareUsageLogInsert(domain).args, prepareUsageLogInsert(restored).args)
	require.Equal(t, domain.Outcome, restored.Outcome)
	require.Equal(t, domain.AntiTruncationRounds, restored.AntiTruncationRounds)
	require.Nil(t, restored.User)
	require.Nil(t, restored.APIKey)
	require.Nil(t, restored.Account)
	for _, secret := range []string{"do-not-persist@example.invalid", "do-not-persist-key", "do-not-persist-credential"} {
		require.NotContains(t, string(raw), secret)
	}
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	usage := payload["usage"].(map[string]any)
	usage["user"] = map[string]any{"email": "private"}
	corrupt, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = decodeUsagePersistencePayload(corrupt, domain.RequestID, domain.APIKeyID)
	require.Error(t, err, "unknown fields cannot silently expand the persisted contract")
}

func TestUsagePersistencePayload_JSONBFormattingDoesNotStrandAdmittedSnapshot(t *testing.T) {
	_, log := usagePersistenceWorkerPayload(t)
	cmd := &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
	raw, err := encodeUsagePersistencePayload(cmd, 0.5)
	require.NoError(t, err)
	padding := strings.Repeat("u", usagePersistencePayloadMaxBytes-len(raw)-100)
	log.UserAgent = &padding
	raw, err = encodeUsagePersistencePayload(cmd, 0.5)
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), usagePersistencePayloadMaxBytes)
	// JSONB serialization can add whitespace even when the admitted JSON was
	// compact. Its equivalent formatting must not make the task undeliverable.
	var stored bytes.Buffer
	require.NoError(t, json.Indent(&stored, raw, "", "  "))
	require.Greater(t, stored.Len(), usagePersistencePayloadMaxBytes)
	restored, err := decodeUsagePersistencePayload(stored.Bytes(), cmd.RequestID, cmd.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, padding, *restored.UserAgent)
}

func TestUsagePersistencePayload_RejectsNonpositiveIdentityBeforeChargeCommit(t *testing.T) {
	for _, field := range []string{"UserID", "APIKeyID", "AccountID"} {
		t.Run(field, func(t *testing.T) {
			_, log := usagePersistenceWorkerPayload(t)
			reflect.ValueOf(log).Elem().FieldByName(field).SetInt(0)
			cmd := &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
			_, err := encodeUsagePersistencePayload(cmd, 0)
			require.Error(t, err, "do not commit a snapshot whose identity the delivery decoder rejects")
		})
	}
}
