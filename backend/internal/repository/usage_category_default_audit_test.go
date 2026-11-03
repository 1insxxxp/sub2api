//go:build unit

package repository

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultUsageAuditPersistsEntryWithoutChangingBillingIdentity(t *testing.T) {
	entry, source := int64(91), int64(17)
	log := &service.UsageLog{RequestID: "category-audit", APIKeyID: 2, UserID: 1, AccountID: 3, GroupID: &source, CreatedAt: time.Now().UTC()}
	field := reflect.ValueOf(log).Elem().FieldByName("CategoryDefaultGroupID")
	require.True(t, field.IsValid(), "usage must preserve the default entry independently of its billing source")
	field.Set(reflect.ValueOf(&entry))
	prepared := prepareUsageLogInsert(log)
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
	require.Equal(t, sql.NullInt64{Int64: entry, Valid: true}, prepared.args[len(prepared.args)-1])
	require.Equal(t, &source, log.GroupID)
	require.Contains(t, usageLogSelectColumns, "category_default_group_id")

	cmd := &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
	raw, err := encodeUsagePersistencePayload(cmd, 0)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.EqualValues(t, 4, payload["version"])
	decoded, err := decodeUsagePersistencePayload(raw, log.RequestID, log.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, &source, decoded.GroupID)
	require.Equal(t, &entry, reflect.ValueOf(decoded).Elem().FieldByName("CategoryDefaultGroupID").Interface())
}

func TestCategoryDefaultUsageAuditIsNotAcceptedInLegacySnapshots(t *testing.T) {
	for _, version := range []int{1, 2, 3} {
		for _, value := range []string{"null", "91"} {
			raw, err := json.Marshal(map[string]any{"version": version, "usage": map[string]any{
				"request_id": "legacy-entry", "api_key_id": 2, "user_id": 1, "account_id": 3,
				"created_at": "2026-11-03T00:00:00Z", "category_default_group_id": json.RawMessage(value),
			}})
			require.NoError(t, err)
			_, err = decodeUsagePersistencePayload(raw, "legacy-entry", 2)
			require.Error(t, err, "old versions cannot silently carry new entry audit")
		}
	}
}
