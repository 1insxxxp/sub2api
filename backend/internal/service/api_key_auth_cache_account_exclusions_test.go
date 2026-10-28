package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotAccountExclusionsRoundtrip(t *testing.T) {
	require.Equal(t, 28, apiKeyAuthSnapshotVersion, "old cached credentials must reload the new access policy")
	svc := &APIKeyService{}
	key := &APIKey{ID: 2, UserID: 7, User: &User{ID: 7}}
	field := reflect.ValueOf(key.User).Elem().FieldByName("GroupAccountExclusions")
	require.True(t, field.IsValid(), "private account exclusions must be available to gateway authentication")
	policy := reflect.MakeMap(field.Type())
	policy.SetMapIndex(reflect.ValueOf(int64(10)), reflect.ValueOf([]int64{21, 22}))
	policy.SetMapIndex(reflect.ValueOf(int64(11)), reflect.ValueOf([]int64{23}))
	field.Set(policy)
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var restored APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(payload, &restored))
	result := svc.snapshotToAPIKey("secret", &restored)
	resultField := reflect.ValueOf(result.User).Elem().FieldByName("GroupAccountExclusions")
	require.Equal(t, field.Interface(), resultField.Interface())
	// An authenticated request may never mutate the shared L1/L2 snapshot.
	resultField.MapIndex(reflect.ValueOf(int64(10))).Index(0).SetInt(99)
	second := svc.snapshotToAPIKey("secret", &restored)
	secondField := reflect.ValueOf(second.User).Elem().FieldByName("GroupAccountExclusions")
	require.Equal(t, int64(21), secondField.MapIndex(reflect.ValueOf(int64(10))).Index(0).Int())
	// Snapshot creation must also detach the original repository result.
	field.MapIndex(reflect.ValueOf(int64(10))).Index(0).SetInt(100)
	third := svc.snapshotToAPIKey("secret", snapshot)
	thirdField := reflect.ValueOf(third.User).Elem().FieldByName("GroupAccountExclusions")
	require.Equal(t, int64(21), thirdField.MapIndex(reflect.ValueOf(int64(10))).Index(0).Int())
}

func TestAPIKeyServiceRejectsV26WithoutAccountExclusions(t *testing.T) {
	svc := &APIKeyService{}
	key, hit, err := svc.applyAuthCacheEntry("secret", &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: 26}})
	require.NoError(t, err)
	require.False(t, hit)
	require.Nil(t, key)
}
