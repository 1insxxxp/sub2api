//go:build unit

package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usagePersistencePayloadArgument struct{ payload map[string]any }

func (a *usagePersistencePayloadArgument) Match(value driver.Value) bool {
	var raw []byte
	switch value := value.(type) {
	case string:
		raw = []byte(value)
	case []byte:
		raw = value
	default:
		return false
	}
	return json.Unmarshal(raw, &a.payload) == nil
}

func expectUsagePersistenceBillingClaim(mock sqlmock.Sqlmock, cmd *service.UsageBillingCommand) {
	cmd.Normalize()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint, threshold_exempt_cost.*FROM usage_billing_dedup_archive`).
		WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnError(sql.ErrNoRows)
}

func TestUsageBillingDurablePersistenceCapability(t *testing.T) {
	repo := &usageBillingRepository{}
	capability, ok := any(repo).(interface{ UsagePersistenceEnabled() bool })
	require.True(t, ok, "real billing repository must advertise its atomic usage outbox")
	require.True(t, capability.UsagePersistenceEnabled())
}

func TestUsageBillingDurablePersistence_EnqueueSharesChargeTransactionAndSafePayload(t *testing.T) {
	for _, enqueueFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "enqueue rollback"}[enqueueFails], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			created := time.Date(2026, 10, 23, 1, 2, 3, 0, time.UTC)
			cmd := &service.UsageBillingCommand{RequestID: "durable-charge", APIKeyID: 2, UserID: 1, AccountID: 3, BalanceCost: 2,
				UsageLog: &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: "durable-charge", Model: "gemini-safe", InputTokens: 12, OutputTokens: 2, UpstreamOutputTokens: 10000, ActualCost: 2, ThresholdExemptCost: 99, CreatedAt: created,
					User: &service.User{Email: "sensitive-user-email"}, APIKey: &service.APIKey{Key: "sensitive-api-key"}, Account: &service.Account{Credentials: map[string]any{"api_key": "sensitive-account-credential"}},
					Outcome: &service.ResponseOutcome{HTTPStatus: 200, HasText: true, OutputBytes: 14, CollectorVersion: 1}}}
			expectUsagePersistenceBillingClaim(mock, cmd)
			mock.ExpectQuery(lockedGiftBalanceDeductSQL).WithArgs(2.0, int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(8.0, true, 0.25))
			mock.ExpectExec(`(?s)UPDATE usage_billing_dedup.*SET threshold_exempt_cost`).WithArgs(0.25, cmd.RequestID, cmd.APIKeyID).
				WillReturnResult(sqlmock.NewResult(0, 1))
			payload := &usagePersistencePayloadArgument{}
			enqueue := mock.ExpectExec(`(?s)INSERT INTO usage_persistence_outbox`).WithArgs(cmd.RequestID, cmd.APIKeyID, payload)
			if enqueueFails {
				enqueue.WillReturnError(errors.New("injected outbox unavailable"))
				mock.ExpectRollback()
			} else {
				enqueue.WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}
			result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
			if enqueueFails {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.True(t, result.Applied)
				require.Equal(t, 0.25, result.ThresholdExemptCost)
			}
			require.NoError(t, mock.ExpectationsWereMet())
			require.EqualValues(t, 1, payload.payload["version"])
			usage := payload.payload["usage"].(map[string]any)
			require.EqualValues(t, 10000, usage["upstream_output_tokens"])
			require.EqualValues(t, 0.25, usage["threshold_exempt_cost"], "wallet allocation, not the caller's stale estimate")
			require.Equal(t, created.Format(time.RFC3339Nano), usage["created_at"])
			raw, err := json.Marshal(payload.payload)
			require.NoError(t, err)
			for _, secret := range []string{"sensitive-user-email", "sensitive-api-key", "sensitive-account-credential", "Credentials", "password_hash"} {
				require.NotContains(t, string(raw), secret)
			}
			for _, field := range []string{"user", "api_key", "account", "group", "subscription", "compensation_eligible", "compensated_cost"} {
				_, exists := usage[field]
				require.False(t, exists, field)
			}
		})
	}
}

func TestUsageBillingDurablePersistence_ReplayDoesNotEnqueueOrOverwrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := &service.UsageBillingCommand{RequestID: "durable-replay", APIKeyID: 2, UserID: 1, BalanceCost: 2,
		UsageLog: &service.UsageLog{UserID: 1, APIKeyID: 2, RequestID: "durable-replay", Model: "changed-history", ActualCost: 100}}
	cmd.Normalize()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT request_fingerprint, threshold_exempt_cost.*FROM usage_billing_dedup\s+WHERE`).WithArgs(cmd.RequestID, cmd.APIKeyID).
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "threshold_exempt_cost"}).AddRow(cmd.RequestFingerprint, 0.25))
	mock.ExpectRollback()
	result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, 0.25, result.ThresholdExemptCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingDurablePersistence_LegacyNoPayloadKeepsPriorBehavior(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := &service.UsageBillingCommand{RequestID: "legacy-no-payload", APIKeyID: 2, UserID: 1}
	expectUsagePersistenceBillingClaim(mock, cmd)
	mock.ExpectExec(`(?s)UPDATE usage_billing_dedup.*SET threshold_exempt_cost`).WithArgs(0.0, cmd.RequestID, cmd.APIKeyID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingDurablePersistence_ArchivedReplayDoesNotCreateNewTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := &service.UsageBillingCommand{RequestID: "archived-durable-replay", APIKeyID: 2, UserID: 1, AccountID: 3, BalanceCost: 2,
		UsageLog: &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: "archived-durable-replay", Model: "gemini-safe", ActualCost: 2}}
	cmd.Normalize()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint, threshold_exempt_cost.*FROM usage_billing_dedup_archive`).WithArgs(cmd.RequestID, cmd.APIKeyID).
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "threshold_exempt_cost"}).AddRow(cmd.RequestFingerprint, 0.25))
	mock.ExpectRollback()
	result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, 0.25, result.ThresholdExemptCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingDurablePersistence_InvalidSnapshotRollsBackClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := &service.UsageBillingCommand{RequestID: "invalid-durable", APIKeyID: 2, UserID: 1, AccountID: 3,
		UsageLog: &service.UsageLog{UserID: 999, APIKeyID: 2, AccountID: 3, RequestID: "invalid-durable", Model: "gemini-safe"}}
	expectUsagePersistenceBillingClaim(mock, cmd)
	mock.ExpectExec(`(?s)UPDATE usage_billing_dedup.*SET threshold_exempt_cost`).WithArgs(0.0, cmd.RequestID, cmd.APIKeyID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
	require.ErrorContains(t, err, "identity mismatch")
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}
