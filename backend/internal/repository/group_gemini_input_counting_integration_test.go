//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGroupGeminiInputCountingPersistenceAndAuthProjection(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	groups := newGroupRepositoryWithSQL(client, tx)
	keys := newAPIKeyRepositoryWithSQL(client, tx)
	group := &service.Group{
		Name: "group-discount-" + uuid.NewString(), Platform: service.PlatformGemini,
		RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		GeminiInputCountingEnabled: true,
	}
	require.NoError(t, groups.Create(ctx, group))
	loaded, err := groups.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.True(t, loaded.GeminiInputCountingEnabled)

	user := mustCreateUser(t, client, &service.User{Email: "group-discount-" + uuid.NewString() + "@example.com", Concurrency: 1})
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "test-group-discount-" + uuid.NewString(), Name: "group-discount", Status: service.StatusActive}
	require.NoError(t, keys.Create(ctx, key))
	fromAuth, err := keys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, fromAuth.Group)
	require.True(t, fromAuth.Group.Hydrated)
	require.True(t, fromAuth.Group.GeminiInputCountingEnabled, "the explicit auth Select must carry the group policy")

	loaded.GeminiInputCountingEnabled = false
	require.NoError(t, groups.Update(ctx, loaded))
	fromAuth, err = keys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.False(t, fromAuth.Group.GeminiInputCountingEnabled)

	defaultGroup, err := client.Group.Create().SetName("default-group-discount-" + uuid.NewString()).Save(ctx)
	require.NoError(t, err)
	require.True(t, defaultGroup.GeminiInputCountingEnabled)
}

func TestGroupInputProtectionMigrationPreservesLegacySettingsAndEnablesNewGroups(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range []string{"new installation", "before policy column", "saved settings"} {
		t.Run(scenario, func(t *testing.T) {
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()
			exec := func(sql string) {
				t.Helper()
				_, err := tx.ExecContext(ctx, sql)
				require.NoError(t, err)
			}
			migrate := func(file string) {
				t.Helper()
				data, err := migrations.FS.ReadFile(file)
				require.NoError(t, err)
				exec(string(data))
			}
			fieldValue := func(name string) bool {
				t.Helper()
				var enabled bool
				err := tx.QueryRowContext(ctx, "SELECT gemini_input_counting_enabled FROM groups WHERE name = $1", name).Scan(&enabled)
				require.NoError(t, err)
				return enabled
			}
			exec("CREATE TEMP TABLE groups (name text, description text, created_at timestamptz, updated_at timestamptz) ON COMMIT DROP")
			const historicalPolicy = "258_group_gemini_input_counting.sql"
			if scenario == "saved settings" {
				migrate(historicalPolicy)
				exec("INSERT INTO groups(name, gemini_input_counting_enabled) VALUES ('old on', TRUE), ('old off', FALSE)")
			} else if scenario == "before policy column" {
				exec("INSERT INTO groups(name) VALUES ('old off')")
			}

			migrate("007a_input_protection_before_default_group_seed.sql")
			migrate("008_seed_default_group.sql")
			migrate(historicalPolicy)
			migrate("269_group_input_protection_default.sql")
			exec("INSERT INTO groups(name) VALUES ('new')")
			require.True(t, fieldValue("new"))
			if scenario == "new installation" {
				require.True(t, fieldValue("default"))
			} else {
				require.False(t, fieldValue("old off"))
			}
			if scenario == "saved settings" {
				require.True(t, fieldValue("old on"))
			}
		})
	}
}
