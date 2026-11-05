//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultPublishedCatalogRealPostgres(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	first := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("default-first-%d", suffix), Platform: service.PlatformGemini, RateMultiplier: 1.3})
	second := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("default-second-%d", suffix), Platform: service.PlatformGemini, RateMultiplier: 1.7})
	accountA := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("default-a-%d", suffix), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"foo": "gemini-2.5-flash"}}})
	accountB := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("default-b-%d", suffix), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"bar": "gemini-2.5-pro"}}})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2),($3,$4)`, accountA.ID, first.ID, accountB.ID, second.ID)
	require.NoError(t, err)
	var channelID int64
	err = integrationDB.QueryRowContext(ctx, `INSERT INTO channels(name,status,model_mapping) VALUES($1,'active',$2::jsonb) RETURNING id`, fmt.Sprintf("default-channel-%d", suffix), `{"gemini":{"foo":"foo"}}`).Scan(&channelID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO channel_groups(channel_id,group_id) VALUES($1,$2)`, channelID, first.ID)
	require.NoError(t, err)
	var entryID int64
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM category_default_group_models WHERE group_id=$1`, entryID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM category_default_group_sources WHERE group_id=$1`, entryID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM category_default_groups WHERE group_id=$1`, entryID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM channels WHERE id=$1`, channelID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id IN ($1,$2)`, accountA.ID, accountB.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, accountA.ID, accountB.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2,$3)`, entryID, first.ID, second.ID)
		require.NoError(t, err)
	})
	repo := NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB)
	sources, err := repo.Sources(ctx, []int64{first.ID})
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.NotNil(t, sources[0].Channel, "the published declaration must read local channel mappings")
	require.Equal(t, "foo", sources[0].Channel.ModelMapping[service.PlatformGemini]["foo"])
	svc := service.NewCategoryDefaultGroupService(repo, nil)
	ids := []int64{first.ID, second.ID}
	preview, err := svc.Preview(ctx, "gemini", ids)
	require.NoError(t, err)
	require.Len(t, preview.Added, 2)
	config, err := svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	entryID = config.GroupID
	require.EqualValues(t, 1, config.Revision)
	require.True(t, config.Enabled)
	group, err := NewGroupRepository(integrationEntClient, integrationDB).GetByIDLite(ctx, entryID)
	require.NoError(t, err)
	require.True(t, group.IsCategoryDefault)
	require.Equal(t, service.SubscriptionTypeStandard, group.SubscriptionType)
	user := &service.User{ID: 100, Status: service.StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{first.ID}}
	models, err := svc.ModelsForUser(ctx, entryID, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, models)
	route, err := svc.ResolveForUser(ctx, entryID, user, "foo")
	require.NoError(t, err)
	require.Equal(t, first.ID, route.SourceGroup.ID)
	require.Equal(t, 1.3, route.SourceGroup.RateMultiplier)
	_, err = svc.ResolveForUser(ctx, entryID, user, "bar")
	require.ErrorIs(t, err, service.ErrCategoryDefaultModelNotAllowed)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}', $1::jsonb) WHERE id=$2`, `{"foo":"gemini-2.5-flash","new":"gemini-2.5-pro"}`, accountA.ID)
	require.NoError(t, err)
	_, err = svc.ResolveForUser(ctx, entryID, user, "new")
	require.NoError(t, err)
	nextPreview, err := svc.Preview(ctx, "gemini", ids)
	require.NoError(t, err)
	require.Empty(t, nextPreview.Added)
	require.Len(t, nextPreview.Unchanged, 3)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}', $1::jsonb) WHERE id=$2`, `{"foo":"gemini-2.5-pro","new":"gemini-2.5-pro"}`, accountA.ID)
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: nextPreview.Revision, PreviewHash: nextPreview.PreviewHash})
	require.ErrorIs(t, err, service.ErrCategoryDefaultStalePreview)
	_, err = svc.ResolveForUser(ctx, entryID, user, "foo")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}', $1::jsonb) WHERE id=$2`, `{"foo":"gemini-2.5-pro","bar":"gemini-2.5-pro"}`, accountB.ID)
	require.NoError(t, err)
	conflict, err := svc.Preview(ctx, "gemini", ids)
	require.NoError(t, err)
	require.Len(t, conflict.Conflicts, 1)
	_, err = svc.Apply(ctx, "gemini", service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: conflict.Revision, PreviewHash: conflict.PreviewHash})
	require.ErrorIs(t, err, service.ErrCategoryDefaultModelConflict)
	stored, err := repo.GetByGroupID(ctx, entryID)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.Revision)
	_, err = svc.ResolveForUser(ctx, entryID, user, "foo")
	require.ErrorIs(t, err, service.ErrCategoryDefaultModelConflict)
	_, err = svc.SetStatus(ctx, "gemini", false)
	require.NoError(t, err)
	_, err = svc.ResolveForUser(ctx, entryID, user, "foo")
	require.ErrorIs(t, err, service.ErrCategoryDefaultSourceUnavailable)
	// Both publishers use the same preview. The serializable transaction and
	// expected revision must admit exactly one complete replacement.
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}', $1::jsonb) WHERE id=$2`, `{"bar":"gemini-2.5-pro"}`, accountB.ID)
	require.NoError(t, err)
	concurrentPreview, err := svc.Preview(ctx, "gemini", ids)
	require.NoError(t, err)
	request := service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: ids, ExpectedRevision: concurrentPreview.Revision, PreviewHash: concurrentPreview.PreviewHash}
	results := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := svc.Apply(ctx, "gemini", request)
			results <- err
		}()
	}
	close(start)
	successful, stale := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successful++
		} else if errors.Is(err, service.ErrCategoryDefaultStalePreview) {
			stale++
		} else {
			t.Fatalf("unexpected concurrent publication result: %v", err)
		}
	}
	require.Equal(t, 1, successful)
	require.Equal(t, 1, stale)
	stored, err = repo.GetByGroupID(ctx, entryID)
	require.NoError(t, err)
	require.Equal(t, concurrentPreview.Revision+1, stored.Revision)
	require.False(t, stored.Enabled, "a subsequent publish must preserve the stopped state")
}

func TestCategoryDefaultSixCategoriesKeepUniqueStableEntryIDs(t *testing.T) {
	ctx := context.Background()
	entryIDs := map[int64]bool{}
	for _, category := range service.CategoryDefaultCategories {
		t.Run(category, func(t *testing.T) {
			platform := category
			if category == "domestic" {
				platform = service.PlatformKimi
			} else if category == "other" {
				platform = service.PlatformComposite
			}
			suffix := fmt.Sprintf("%s-%d", category, time.Now().UnixNano())
			source := &service.Group{Name: "stable-source-" + suffix, Platform: platform, KeyDisplayCategory: category, RateMultiplier: 1,
				Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
			require.NoError(t, NewGroupRepository(integrationEntClient, integrationDB).Create(ctx, source))
			accountPlatform := platform
			if platform == service.PlatformComposite {
				accountPlatform = service.PlatformOpenAI
			}
			public := "stable-model-" + category
			account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "stable-account-" + suffix, Platform: accountPlatform, Type: service.AccountTypeAPIKey,
				Credentials: map[string]any{"model_mapping": map[string]any{public: "upstream-stable"}}})
			_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, source.ID)
			require.NoError(t, err)
			var entryID int64
			t.Cleanup(func() {
				for _, query := range []string{
					`DELETE FROM category_default_group_models WHERE group_id=$1`,
					`DELETE FROM category_default_group_sources WHERE group_id=$1`,
					`DELETE FROM category_default_groups WHERE group_id=$1`,
				} {
					_, cleanupErr := integrationDB.ExecContext(ctx, query, entryID)
					require.NoError(t, cleanupErr)
				}
				_, cleanupErr := integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1`, account.ID)
				require.NoError(t, cleanupErr)
				_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM composite_model_routes WHERE group_id=$1`, source.ID)
				require.NoError(t, cleanupErr)
				_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
				require.NoError(t, cleanupErr)
				_, cleanupErr = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, entryID, source.ID)
				require.NoError(t, cleanupErr)
			})
			if platform == service.PlatformComposite {
				route := &service.CompositeModelRoute{GroupID: source.ID, PublicModel: public, MatchType: service.CompositeRouteMatchExact,
					TargetPlatform: service.PlatformOpenAI, UpstreamModel: public, Endpoint: service.CompositeRouteEndpointAny, Priority: 1, Enabled: true}
				require.NoError(t, NewCompositeModelRouteRepository(integrationEntClient).Create(ctx, route))
			}
			svc := service.NewCategoryDefaultGroupService(NewCategoryDefaultGroupRepository(integrationEntClient, integrationDB), nil)
			publish := func() *service.CategoryDefaultGroupConfig {
				preview, previewErr := svc.Preview(ctx, category, []int64{source.ID})
				require.NoError(t, previewErr)
				require.Equal(t, 1, preview.ModelsCount)
				config, applyErr := svc.Apply(ctx, category, service.CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{source.ID}, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
				require.NoError(t, applyErr)
				return config
			}
			first := publish()
			entryID = first.GroupID
			require.Positive(t, entryID)
			require.False(t, entryIDs[entryID], "categories must not share an entry")
			entryIDs[entryID] = true
			require.True(t, first.Enabled, "first successful publication enables the entry")
			second := publish()
			require.Equal(t, entryID, second.GroupID)
			require.True(t, second.Enabled)
			require.Greater(t, second.Revision, first.Revision)
			_, err = svc.SetStatus(ctx, category, false)
			require.NoError(t, err)
			third := publish()
			require.Equal(t, entryID, third.GroupID)
			require.False(t, third.Enabled, "later publication must preserve an administrator's disabled status")
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM groups WHERE is_category_default AND key_display_category=$1 AND deleted_at IS NULL`, category).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
	require.Len(t, entryIDs, 6)
}
