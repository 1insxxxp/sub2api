//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type supportRepoStub struct {
	allowed    bool
	err        error
	grantCalls int
	usageErr   error
	errorsErr  error
	result     UserSupportGrantResult
	usageRows  []UserSupportUsageRecord
	errorRows  []UserSupportErrorRecord
}

func (r *supportRepoStub) CanAccess(context.Context, int64) (bool, error) { return r.allowed, r.err }
func (r *supportRepoStub) GetAccess(context.Context, int64, int64) (bool, error) {
	return r.allowed, r.err
}
func (r *supportRepoStub) SetAccess(context.Context, int64, int64, bool) error { return r.err }
func (r *supportRepoStub) LookupAndGrant(context.Context, int64, string) (*UserSupportGrantResult, error) {
	r.grantCalls++
	return &r.result, r.err
}
func (r *supportRepoStub) ListUsage(context.Context, int64) ([]UserSupportUsageRecord, error) {
	return r.usageRows, r.usageErr
}
func (r *supportRepoStub) ListErrors(context.Context, int64) ([]UserSupportErrorRecord, error) {
	return r.errorRows, r.errorsErr
}

func TestUserSupportOnlyReturnsSafeRecordValuesAndTenRows(t *testing.T) {
	r := &supportRepoStub{allowed: true, result: UserSupportGrantResult{User: UserSupportUser{ID: 2}, Trial: UserSupportTrial{Status: "already_received", Amount: 15}}}
	for i := 0; i < 12; i++ {
		r.usageRows = append(r.usageRows, UserSupportUsageRecord{ID: int64(i + 1), Model: "https://secret.example/sk-secret", GroupName: "客户分组", Status: "account-secret.example"})
		r.errorRows = append(r.errorRows, UserSupportErrorRecord{ID: int64(i + 1), Model: "private-secret.example", Category: "http://secret.example"})
	}
	got, err := NewUserSupportService(r, nil, nil).Lookup(context.Background(), 1, "2")
	require.NoError(t, err)
	require.Len(t, got.Usage.Items, 10)
	require.Len(t, got.Errors.Items, 10)
	require.Equal(t, "unknown", got.Usage.Items[0].Status)
	require.Equal(t, "客户分组", got.Usage.Items[0].GroupName)
	require.Equal(t, "other", got.Errors.Items[0].Category)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	var schema map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &schema))
	require.ElementsMatch(t, []string{"user", "trial", "usage", "errors"}, supportKeys(schema))
	assertSupportSchema(t, schema["user"], "id", "email", "balance")
	assertSupportSchema(t, schema["trial"], "status", "amount", "granted_at", "legacy")
	for _, panelName := range []string{"usage", "errors"} {
		panel := assertSupportSchema(t, schema[panelName], "status", "items")
		var items []json.RawMessage
		require.NoError(t, json.Unmarshal(panel["items"], &items))
		for _, item := range items {
			if panelName == "usage" {
				assertSupportSchema(t, item, "id", "created_at", "model", "group_name", "input_tokens", "output_tokens", "cache_read_tokens", "charged_amount", "duration_ms", "first_token_ms", "status", "stream")
			} else {
				assertSupportSchema(t, item, "id", "created_at", "model", "category")
			}
		}
	}
}

func assertSupportSchema(t *testing.T, encoded json.RawMessage, keys ...string) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.ElementsMatch(t, keys, supportKeys(fields))
	return fields
}

func supportKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type supportCacheStub struct{ authCalls, balanceCalls int }

func (s *supportCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.balanceCalls++
	return errors.New("private.example")
}
func (s *supportCacheStub) InvalidateAuthCacheByUserID(context.Context, int64)  { s.authCalls++ }
func (s *supportCacheStub) InvalidateAuthCacheByKey(context.Context, string)    {}
func (s *supportCacheStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func TestUserSupportReplayRepairsCacheWithoutRegranting(t *testing.T) {
	r := &supportRepoStub{allowed: true, result: UserSupportGrantResult{User: UserSupportUser{ID: 2}, Trial: UserSupportTrial{Status: "already_received", Amount: 15}}}
	cache := &supportCacheStub{}
	s := NewUserSupportService(r, nil, nil)
	s.authCache = cache
	s.billingCache = cache
	got, err := s.Lookup(context.Background(), 1, "2")
	require.NoError(t, err)
	require.Equal(t, "already_received", got.Trial.Status)
	require.Equal(t, 1, cache.authCalls)
	require.Equal(t, 1, cache.balanceCalls)
}

func TestUserSupportRejectsUnauthorizedAndInvalidLookup(t *testing.T) {
	r := &supportRepoStub{}
	s := NewUserSupportService(r, nil, nil)
	_, err := s.Lookup(context.Background(), 12, "person@example.com")
	require.ErrorIs(t, err, ErrUserSupportForbidden)
	require.Zero(t, r.grantCalls)
	r.allowed = true
	for _, account := range []string{"", "0", "-1", "9223372036854775808", "https://secret.example", "a@example.com\nsecret"} {
		_, err = s.Lookup(context.Background(), 12, account)
		require.ErrorIs(t, err, ErrUserSupportInvalidAccount, account)
	}
	require.Zero(t, r.grantCalls)
}

func TestUserSupportCommittedGrantSurvivesReadFailure(t *testing.T) {
	r := &supportRepoStub{allowed: true, usageErr: errors.New("https://secret.example/sk-secret"), errorsErr: errors.New("account=secret"), result: UserSupportGrantResult{User: UserSupportUser{ID: 14, Email: "a@example.com", Balance: 23}, Trial: UserSupportTrial{Status: "granted", Amount: 15, GrantedAt: time.Unix(100, 0)}}}
	s := NewUserSupportService(r, nil, nil)
	got, err := s.Lookup(context.Background(), 12, " a@example.com ")
	require.NoError(t, err)
	require.Equal(t, "granted", got.Trial.Status)
	require.Equal(t, "unavailable", got.Usage.Status)
	require.Equal(t, "unavailable", got.Errors.Status)
	require.NotNil(t, got.Usage.Items)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
}

func TestUserSupportFixedErrorsAndPublicModelProjection(t *testing.T) {
	r := &supportRepoStub{allowed: true, err: errors.New("dial 10.0.0.1 https://private.example/sk-abc")}
	s := NewUserSupportService(r, nil, nil)
	_, err := s.CanAccess(context.Background(), 1)
	require.ErrorIs(t, err, ErrUserSupportUnavailable)
	require.NotContains(t, err.Error(), "private")
	for _, v := range []string{"", "private.example", "gemini-private.example", "sk-secret", "https://10.0.0.1/secret"} {
		require.Equal(t, "unknown", PublicUserSupportModel(v))
	}
	require.Equal(t, "gemini-4-argon", PublicUserSupportModel("按量vet/gemini-4-argon"))
	require.Equal(t, "gemini-2.5-pro", PublicUserSupportModel("gemini-2.5-pro"))
	require.Equal(t, "service_unavailable", UserSupportErrorCategory("upstream", "https://secret.example"))
	require.Equal(t, "quota", UserSupportErrorCategory("request", "billing_error"))
}
