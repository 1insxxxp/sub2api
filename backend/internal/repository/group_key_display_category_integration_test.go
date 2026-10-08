//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryPersistence(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newGroupRepositoryWithSQL(tx.Client(), tx)
	created := &service.Group{Name: "key-category-persistence", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 1, KeyDisplayCategory: "domestic"}
	require.NoError(t, repo.Create(ctx, created))
	lite, err := repo.GetByIDLite(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "domestic", lite.KeyDisplayCategory)
	full, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "domestic", full.KeyDisplayCategory)
	list, _, err := repo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 100}, "openai", "", created.Name, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "domestic", list[0].KeyDisplayCategory)
	duplicate := *created
	duplicate.ID = 0
	duplicate.Name = "key-category-persistence-copy"
	duplicate.DuplicateOperationID = "key-category-integration-duplicate"
	require.NoError(t, repo.CreateFromSource(ctx, &duplicate, created.ID))
	copied, err := repo.GetByIDLite(ctx, duplicate.ID)
	require.NoError(t, err)
	require.Equal(t, "domestic", copied.KeyDisplayCategory)
	created.KeyDisplayCategory = ""
	require.NoError(t, repo.Update(ctx, created))
	cleared, err := repo.GetByIDLite(ctx, created.ID)
	require.NoError(t, err)
	require.Empty(t, cleared.KeyDisplayCategory)
	require.Equal(t, service.PlatformOpenAI, cleared.Platform)
}
