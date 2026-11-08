package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type categoryDefaultProbeGateway struct {
	runner       *modelProbeRunnerStub
	catalogCalls int
	group        *Group
	model        string
	released     int
}

func (g *categoryDefaultProbeGateway) ModelsForProbe(context.Context, *Group) ([]string, error) {
	g.catalogCalls++
	return []string{"unpublished-model"}, nil
}
func (g *categoryDefaultProbeGateway) SelectForProbe(_ context.Context, group *Group, model string) (*ModelProbeSelection, error) {
	g.group, g.model = group, model
	return &ModelProbeSelection{AccountID: 8, Model: "native-model", Release: func() { g.released++ }}, nil
}

func (g *categoryDefaultProbeGateway) ExecuteForProbe(ctx context.Context, group *Group, model string) (ModelProbeExecution, error) {
	selected, err := g.SelectForProbe(ctx, group, model)
	if err != nil {
		return ModelProbeExecution{}, err
	}
	defer selected.Release()
	return g.runner.RunModelProbe(ctx, selected.AccountID, selected.Model)
}

type categoryDefaultProbeUsers struct {
	calls int
	onGet func(int) *User
}

func (u *categoryDefaultProbeUsers) GetByID(context.Context, int64) (*User, error) {
	u.calls++
	return u.onGet(u.calls), nil
}

func categoryDefaultProbeFixture(t *testing.T) (*ModelProbeService, *categoryDefaultProbeGateway, *modelProbeRunnerStub) {
	t.Helper()
	source := categoryDefaultSource(1, "Gemini source", map[string]any{"public-model": "native-model"})
	source.Group.RateMultiplier = 2.5
	source.Group.GeminiInputCountingEnabled = true
	store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{source}}
	defaults := NewCategoryDefaultGroupService(store, nil)
	preview, err := defaults.Preview(context.Background(), "gemini", []int64{1})
	require.NoError(t, err)
	_, err = defaults.Apply(context.Background(), "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	groups := &modelProbeGroupsStub{group: &Group{ID: 100, Name: "Gemini default", Platform: PlatformComposite, Status: StatusActive, Hydrated: true, IsCategoryDefault: true}}
	gateway := &categoryDefaultProbeGateway{}
	runner := &modelProbeRunnerStub{result: ModelProbeExecution{Status: "success"}}
	user := &User{ID: 4, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{1}}
	users := &categoryDefaultProbeUsers{onGet: func(int) *User { return user }}
	gateway.runner = runner
	svc := NewModelProbeService(groups, &modelProbeAccessStub{allowed: true}, gateway, groups, probeSettingsFor(100), users)
	svc.SetCategoryDefaultGroupService(defaults)
	return svc, gateway, runner
}

func TestCategoryDefaultProbeCatalogUsesPublishedActorModels(t *testing.T) {
	svc, gateway, _ := categoryDefaultProbeFixture(t)
	catalog, err := svc.Models(context.Background(), 4, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), catalog.Group.ID)
	require.Equal(t, []ModelProbeModel{{ID: "public-model"}}, catalog.Items)
	require.Zero(t, gateway.catalogCalls, "a default entry has no direct account catalog")
}

func TestCategoryDefaultProbeSelectsSourceAndKeepsPublicResult(t *testing.T) {
	svc, gateway, runner := categoryDefaultProbeFixture(t)
	result, err := svc.Run(context.Background(), 4, 100, "public-model")
	require.NoError(t, err)
	require.Equal(t, int64(100), result.GroupID)
	require.Equal(t, "public-model", result.Model)
	require.Equal(t, "success", result.Status)
	require.NotNil(t, gateway.group)
	require.Equal(t, int64(1), gateway.group.ID)
	require.Equal(t, 2.5, gateway.group.RateMultiplier)
	require.True(t, gateway.group.GeminiInputCountingEnabled)
	require.Equal(t, "public-model", gateway.model)
	require.Equal(t, int64(8), runner.account)
	require.Equal(t, 1, gateway.released)
}

func TestCategoryDefaultProbeRechecksSourceAccessBeforeSelection(t *testing.T) {
	svc, gateway, runner := categoryDefaultProbeFixture(t)
	svc.users = &categoryDefaultProbeUsers{onGet: func(call int) *User {
		u := &User{ID: 4, Status: StatusActive, RestrictPublicGroups: true}
		if call == 1 {
			u.AllowedGroups = []int64{1}
		}
		return u
	}}
	_, err := svc.Run(context.Background(), 4, 100, "public-model")
	require.Error(t, err)
	require.Nil(t, gateway.group)
	require.Zero(t, runner.account)
}

func TestCategoryDefaultProbeMissingResolverNeverUsesDirectCatalog(t *testing.T) {
	svc, gateway, _ := categoryDefaultProbeFixture(t)
	svc.SetCategoryDefaultGroupService(nil)
	_, err := svc.Models(context.Background(), 4, 100)
	require.ErrorIs(t, err, ErrUserSupportUnavailable)
	require.Zero(t, gateway.catalogCalls)
}
