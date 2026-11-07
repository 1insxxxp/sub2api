//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type supportModelSourcesStub struct {
	sources []CategoryDefaultSourceSnapshot
	ids     []int64
	calls   int
	err     error
}

func (s *supportModelSourcesStub) Sources(ctx context.Context, ids []int64) ([]CategoryDefaultSourceSnapshot, error) {
	s.calls++
	s.ids = ids
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded catalog read")
	}
	return s.sources, s.err
}

func TestUserSupportModelsFromScopedPublicDeclarations(t *testing.T) {
	source := CategoryDefaultSourceSnapshot{
		Group: &Group{ID: 7, Platform: PlatformGemini},
		Accounts: []Account{{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{
			"enp/new-model-2027": "secret-mapped-model", "定制别名": "secret-mapped-model", "private.example/gemini-4-argon": "secret-mapped-model",
		}}}},
	}
	sources := &supportModelSourcesStub{sources: []CategoryDefaultSourceSnapshot{source}}
	r := &supportRepoStub{allowed: true, result: UserSupportGrantResult{User: UserSupportUser{ID: 2}}}
	for _, tc := range []struct {
		group int64
		name  string
	}{
		{7, "enp/new-model-2027"}, {7, "定制别名"}, {8, "定制别名"}, {7, "secret-mapped-model"},
		{7, "private.example/gemini-4-argon"}, {7, ""}, {7, "removed-alias"}, {7, "enp/gemini-4-argon"}, {7, "/Users/alice/gemini-4-argon"}, {7, "../private/gemini-4-argon"},
	} {
		r.usageRows = append(r.usageRows, UserSupportUsageRecord{Model: "unknown", ModelContext: &UserSupportModelContext{tc.name, tc.group}})
		r.errorRows = append(r.errorRows, UserSupportErrorRecord{Model: "unknown", ModelContext: &UserSupportModelContext{tc.name, tc.group}})
	}
	s := NewUserSupportService(r, nil, nil)
	s.modelSources = sources
	got, err := s.Lookup(context.Background(), 1, "2")
	require.NoError(t, err)
	expected := []string{"enp/new-model-2027", "定制别名", "unknown", "unknown", "unknown", "unknown", "unknown", "enp/gemini-4-argon", "unknown", "unknown"}
	for i, name := range expected {
		require.Equal(t, name, got.Usage.Items[i].Model)
		require.Equal(t, name, got.Errors.Items[i].Model)
		require.Nil(t, got.Usage.Items[i].ModelContext)
		require.Nil(t, got.Errors.Items[i].ModelContext)
	}
	require.Equal(t, 1, sources.calls)
	require.Equal(t, []int64{7, 8}, sources.ids)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	for _, hidden := range []string{"secret-mapped", "private.example", "ModelContext", "group_id", "RequestedModel"} {
		require.NotContains(t, string(raw), hidden)
	}
	sources.err = errors.New("catalog unavailable")
	got, err = s.Lookup(context.Background(), 1, "2")
	require.NoError(t, err)
	require.Equal(t, "unknown", got.Usage.Items[0].Model)
	require.Equal(t, "enp/gemini-4-argon", got.Usage.Items[7].Model)
	require.Equal(t, "ok", got.Usage.Status)
}

func TestUserSupportModelContextCannotBeSerialized(t *testing.T) {
	raw, err := json.Marshal(UserSupportUsageRecord{Model: "unknown", ModelContext: &UserSupportModelContext{"secret-mapped", 7}})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "group_id")
}
