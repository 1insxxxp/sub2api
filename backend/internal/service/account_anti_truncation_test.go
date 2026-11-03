//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAccountAntiTruncationCreateRejectsInvalidPolicyBeforePersisting(t *testing.T) {
	_, err := (&adminServiceImpl{}).CreateAccount(context.Background(), &CreateAccountInput{
		Platform: PlatformAnthropic,
		Extra:    antiTruncationExtraForTest(t, `{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":4}]}`),
	})
	require.Error(t, err)
}

func TestAccountAntiTruncationExtraUpdateRejectsInvalidPolicyBeforePersisting(t *testing.T) {
	err := (&adminServiceImpl{}).UpdateAccountExtra(context.Background(), 1,
		antiTruncationExtraForTest(t, `{"version":2,"rules":[]}`))
	require.Error(t, err)
}

func TestAccountAntiTruncationBulkUpdateRejectsInvalidPolicyBeforePersisting(t *testing.T) {
	_, err := (&adminServiceImpl{}).BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{1}, Extra: antiTruncationExtraForTest(t, `{"version":2,"rules":[]}`),
	})
	require.Error(t, err)
}

func antiTruncationExtraForTest(t *testing.T, policy string) map[string]any {
	t.Helper()
	var value any
	require.NoError(t, json.Unmarshal([]byte(policy), &value))
	return map[string]any{"anti_truncation": value}
}

func TestAccountAntiTruncationPolicyResolution(t *testing.T) {
	tests := []struct {
		name, policy, requested, route string
		want                           domain.AntiTruncationPolicy
		managed                        bool
	}{
		{name: "missing policy is off"},
		{name: "explicit empty is off", policy: `{"version":1,"rules":[]}`, managed: true},
		{name: "unmatched rule is off", policy: `{"version":1,"rules":[{"model":"other","enabled":true}]}`, requested: "public", route: "route", managed: true},
		{name: "original alias takes precedence", policy: `{"version":1,"rules":[{"model":"route","enabled":true,"max_rounds":3},{"model":"public","enabled":true,"max_rounds":2}]}`, requested: "public", route: "route", want: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2}, managed: true},
		{name: "explicit original alias off wins", policy: `{"version":1,"rules":[{"model":"route","enabled":true},{"model":"public","enabled":false}]}`, requested: "public", route: "route", managed: true},
		{name: "route model fallback", policy: `{"version":1,"rules":[{"model":"route","enabled":true,"max_output_tokens":100}]}`, requested: "public", route: "route", want: domain.AntiTruncationPolicy{Enabled: true, MaxOutputTokens: 100}, managed: true},
		{name: "no final upstream reverse matching", policy: `{"version":1,"rules":[{"model":"upstream","enabled":true}]}`, requested: "normal", route: "normal", managed: true},
		{name: "model case remains exact", policy: `{"version":1,"rules":[{"model":"Public","enabled":true}]}`, requested: "public", route: "public", managed: true},
		{name: "unknown version fails closed", policy: `{"version":2,"rules":[{"model":"public","enabled":true}]}`, requested: "public", managed: true},
		{name: "invalid limits fail closed", policy: `{"version":1,"rules":[{"model":"public","enabled":true,"max_rounds":4}]}`, requested: "public", managed: true},
		{name: "null fails closed", policy: `null`, requested: "public", managed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Credentials: map[string]any{"model_mapping": map[string]any{"normal": "upstream"}}}
			if tt.policy != "" {
				a.Extra = antiTruncationExtraForTest(t, tt.policy)
			}
			got, managed := ResolveAccountAntiTruncationPolicy(a, tt.requested, tt.route)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.managed, managed)
		})
	}
}

func TestAccountAntiTruncationExtraValidation(t *testing.T) {
	require.NoError(t, ValidateAccountAntiTruncationExtra(nil))
	for _, raw := range []string{
		`{"version":1,"rules":[]}`,
		`{"version":1,"rules":[{"model":"防截断/gemini-4-argon","enabled":true,"max_rounds":3,"max_output_tokens":0,"max_duration_seconds":300}]}`,
		`{"version":1,"rules":[{"model":"normal","enabled":false}]}`,
	} {
		require.NoError(t, ValidateAccountAntiTruncationExtra(antiTruncationExtraForTest(t, raw)), raw)
	}
	for _, raw := range []string{
		`null`, `true`, `{"version":2,"rules":[]}`, `{"version":1}`, `{"version":1,"rules":null}`,
		`{"version":1,"rules":[{"model":"x"}]}`,
		`{"version":1,"rules":[{"model":"","enabled":true}]}`,
		`{"version":1,"rules":[{"model":"has space","enabled":true}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true},{"model":"x","enabled":false}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":4}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_output_tokens":32769}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_duration_seconds":301}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":-1}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":1.5}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":"1"}]}`,
		`{"version":1,"rules":[{"model":"x","enabled":true,"unexpected":1}]}`,
	} {
		require.Error(t, ValidateAccountAntiTruncationExtra(antiTruncationExtraForTest(t, raw)), raw)
	}
}

func TestAccountAntiTruncationUpdateRejectsInvalidPolicyBeforePersisting(t *testing.T) {
	_, err := (&adminServiceImpl{}).UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Extra: antiTruncationExtraForTest(t, `{"version":1,"rules":[{"model":"x","enabled":true,"max_rounds":4}]}`),
	})
	require.Error(t, err)
}

func TestAccountAntiTruncationAdminRoundTripAndOmittedUpdate(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	svc := &adminServiceImpl{accountRepo: repo}
	extra := antiTruncationExtraForTest(t, `{"version":1,"rules":[{"model":"public","enabled":true,"max_rounds":2}]}`)
	created, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name: "continuation", Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test"}, Extra: extra, SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.Equal(t, extra[AccountExtraAntiTruncation], created.Extra[AccountExtraAntiTruncation])
	updated, err := svc.UpdateAccount(context.Background(), created.ID, &UpdateAccountInput{Extra: map[string]any{"other": true}})
	require.NoError(t, err)
	require.Equal(t, extra[AccountExtraAntiTruncation], updated.Extra[AccountExtraAntiTruncation])
	disabled := antiTruncationExtraForTest(t, `{"version":1,"rules":[]}`)
	updated, err = svc.UpdateAccount(context.Background(), created.ID, &UpdateAccountInput{Extra: disabled})
	require.NoError(t, err)
	require.Equal(t, disabled[AccountExtraAntiTruncation], updated.Extra[AccountExtraAntiTruncation])
	policy, owned := ResolveAccountAntiTruncationPolicy(updated, "public", "public")
	require.True(t, owned)
	require.False(t, policy.Enabled)
}
