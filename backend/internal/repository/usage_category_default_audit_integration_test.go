//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (s *UsageLogRepoSuite) TestCategoryDefaultEntryAuditRoundTripsAllInsertPaths() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "category-audit@test.invalid"})
	key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-category-audit", Name: "audit"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "category-audit"})
	source := mustCreateGroup(s.T(), s.client, &service.Group{Name: "category-source", Platform: service.PlatformGemini})
	entry := mustCreateGroup(s.T(), s.client, &service.Group{Name: "category-entry", Platform: service.PlatformComposite})
	for _, path := range []string{"single", "batch", "best-effort", "no-result"} {
		s.Run(path, func() {
			log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: fmt.Sprintf("category-audit-%s", path),
				Model: "gemini-test", GroupID: &source.ID, CategoryDefaultGroupID: &entry.ID, ActualCost: 0.17, TotalCost: 0.2, CreatedAt: time.Now().UTC()}
			prepared := prepareUsageLogInsert(log)
			switch path {
			case "single":
				_, err := s.repo.Create(s.ctx, log)
				s.Require().NoError(err)
			case "batch":
				batchKey := usageLogBatchKey(log.RequestID, key.ID)
				query, args := buildUsageLogBatchInsertQuery([]string{batchKey}, map[string]usageLogInsertPrepared{batchKey: prepared})
				_, err := s.repo.sql.ExecContext(s.ctx, query, args...)
				s.Require().NoError(err)
			case "best-effort":
				query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
				_, err := s.repo.sql.ExecContext(s.ctx, query, args...)
				s.Require().NoError(err)
			case "no-result":
				s.Require().NoError(execUsageLogInsertNoResult(s.ctx, s.repo.sql, prepared))
			}
			var id int64
			s.Require().NoError(scanSingleRow(s.ctx, s.repo.sql, "SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", []any{log.RequestID, key.ID}, &id))
			got, err := s.repo.GetByID(s.ctx, id)
			s.Require().NoError(err)
			s.Require().Equal(&source.ID, got.GroupID)
			s.Require().Equal(&entry.ID, got.CategoryDefaultGroupID)
			s.Require().Equal(0.17, got.ActualCost)
		})
	}
}

func TestCategoryDefaultBillingOutboxKeepsSourceAuditAndNeverRebills(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	suffix := uuid.NewString()
	source := mustCreateGroup(t, client, &service.Group{Name: "audit-source-" + suffix, Platform: service.PlatformGemini})
	entry := mustCreateGroup(t, client, &service.Group{Name: "audit-entry-" + suffix, Platform: service.PlatformComposite, KeyDisplayCategory: "gemini", IsCategoryDefault: true})
	// The shared fixture only inserts legacy fields; persist the actual entry
	// type so this financial regression exercises a real category-bound key.
	require.NoError(t, client.Group.UpdateOneID(entry.ID).SetIsCategoryDefault(true).SetKeyDisplayCategory("gemini").Exec(ctx))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1,$2)", source.ID, entry.ID)
		require.NoError(t, err)
	})
	for _, viaDefault := range []bool{false, true} {
		t.Run(fmt.Sprintf("default=%v", viaDefault), func(t *testing.T) {
			f := newUsagePersistenceFixture(t, client)
			f.cmd.UsageLog.GroupID = &source.ID
			boundGroup := source.ID
			if viaDefault {
				boundGroup = entry.ID
				f.cmd.UsageLog.CategoryDefaultGroupID = &entry.ID
			}
			_, err := integrationDB.ExecContext(ctx, "UPDATE api_keys SET group_id=$1 WHERE id=$2", boundGroup, f.keyID)
			require.NoError(t, err)
			billing := NewUsageBillingRepository(client, integrationDB)
			first, err := billing.Apply(ctx, f.cmd)
			require.NoError(t, err)
			require.True(t, first.Applied)
			replay, err := billing.Apply(ctx, f.cmd)
			require.NoError(t, err)
			require.False(t, replay.Applied)
			outbox := NewUsagePersistenceOutboxRepository(client, integrationDB)
			event := claimUsagePersistenceFixture(t, outbox, "category-audit-worker")
			require.NoError(t, outbox.Deliver(ctx, event.ID, "category-audit-worker"))
			require.NoError(t, outbox.Deliver(ctx, event.ID, "category-audit-worker"))
			balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
			require.Equal(t, 8.0, balance)
			require.Equal(t, 2.0, quota)
			require.Equal(t, 1, usage)
			require.Equal(t, 1, outcome)
			require.Equal(t, 1, dedup)
			require.Zero(t, pending)
			var sourceID, keyGroup int64
			var entryID *int64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT ul.group_id,ul.category_default_group_id,k.group_id FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE ul.api_key_id=$1`, f.keyID).Scan(&sourceID, &entryID, &keyGroup))
			require.Equal(t, source.ID, sourceID)
			require.Equal(t, boundGroup, keyGroup, "request resolution must not rewrite the saved key binding")
			if viaDefault {
				require.Equal(t, &entry.ID, entryID)
			} else {
				require.Nil(t, entryID)
			}
		})
	}
}
