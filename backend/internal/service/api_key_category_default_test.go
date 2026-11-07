package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type categoryAPIKeyResolverFake struct {
	source   *Group
	userSeen *User
}

func (f *categoryAPIKeyResolverFake) ResolveForUser(_ context.Context, entry int64, user *User, model string) (*CategoryDefaultModelRoute, error) {
	f.userSeen = user
	if !user.CanBindGroup(f.source.ID, f.source.IsExclusive) {
		return nil, ErrGroupNotAllowed
	}
	return &CategoryDefaultModelRoute{EntryGroupID: entry, SourceGroup: f.source, PublicModel: model, SourceModel: model}, nil
}
func (f *categoryAPIKeyResolverFake) ModelsForUser(_ context.Context, _ int64, user *User, _ string) ([]string, error) {
	if !user.CanBindGroup(f.source.ID, f.source.IsExclusive) {
		return []string{}, nil
	}
	return []string{"gemini-test"}, nil
}
func (f *categoryAPIKeyResolverFake) CanBind(ctx context.Context, id int64, user *User) (bool, error) {
	models, err := f.ModelsForUser(ctx, id, user, "")
	return len(models) > 0, err
}

type categoryAPIKeyUserRepo struct {
	UserRepository
	user *User
}

func (r categoryAPIKeyUserRepo) GetByID(context.Context, int64) (*User, error) {
	u := *r.user
	return &u, nil
}

type categoryAPIKeyRepo struct {
	APIKeyRepository
	key *APIKey
}

func (r categoryAPIKeyRepo) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }

type categoryAPIKeyRates struct {
	UserGroupRateRepository
	groupID int64
	rpm     int
}

type categoryAPIKeyGroups struct {
	GroupRepository
	groups []Group
}

func (r categoryAPIKeyGroups) ListActive(context.Context) ([]Group, error) { return r.groups, nil }

type categoryAPIKeySubscriptions struct{ UserSubscriptionRepository }

func (categoryAPIKeySubscriptions) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func (r *categoryAPIKeyRates) GetRPMOverrideByUserAndGroup(_ context.Context, _ int64, group int64) (*int, error) {
	r.groupID = group
	return &r.rpm, nil
}

func TestCategoryDefaultAPIKeyClonePreservesIdentityAndSourcePolicies(t *testing.T) {
	entry, sourceID, fallback := int64(51), int64(8), int64(99)
	originalRPM := 123
	original := &APIKey{ID: 4, UserID: 2, GroupID: &entry, Quota: 10, QuotaUsed: 3, RateLimit5h: 5,
		Group: &Group{ID: entry, Platform: PlatformComposite, IsCategoryDefault: true, Status: StatusActive, Hydrated: true},
		User:  &User{ID: 2, Status: StatusActive, UserGroupRPMOverride: &originalRPM, AllowedGroups: []int64{8}},
	}
	source := &Group{ID: sourceID, Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, Hydrated: true, IsExclusive: true, RateMultiplier: 0.75, GeminiInputCountingEnabled: true, FallbackGroupID: &fallback, FallbackGroupIDOnInvalidRequest: &fallback}
	resolver := &categoryAPIKeyResolverFake{source: source}
	fresh := &User{ID: 2, Status: StatusActive, AllowedGroups: []int64{sourceID}, RestrictPublicGroups: true}
	exclusions := GroupAccountExclusions{sourceID: []int64{27}}
	repoKey := *original
	repoUser := *fresh
	repoUser.GroupAccountExclusions = exclusions
	repoKey.User = &repoUser
	rates := &categoryAPIKeyRates{rpm: 7}
	svc := &APIKeyService{userRepo: categoryAPIKeyUserRepo{user: fresh}, apiKeyRepo: categoryAPIKeyRepo{key: &repoKey}, categoryDefaultGroups: resolver, userGroupRateRepo: rates}
	result, err := svc.ResolveCategoryDefaultGroupModel(context.Background(), original, "gemini-test")
	require.NoError(t, err)
	key := result.APIKey
	require.Equal(t, original.ID, key.ID)
	require.Equal(t, original.Quota, key.Quota)
	require.Equal(t, original.QuotaUsed, key.QuotaUsed)
	require.Equal(t, original.RateLimit5h, key.RateLimit5h)
	require.Equal(t, sourceID, *key.GroupID)
	require.Equal(t, entry, *key.CategoryDefaultGroupID)
	require.NotSame(t, source, key.Group)
	require.NotSame(t, original.User, key.User)
	require.Nil(t, key.Group.FallbackGroupID)
	require.Nil(t, key.Group.FallbackGroupIDOnInvalidRequest)
	require.Equal(t, fallback, *source.FallbackGroupID, "must not mutate the source repository snapshot")
	require.Equal(t, sourceID, rates.groupID)
	require.Equal(t, 7, *key.User.UserGroupRPMOverride)
	require.Equal(t, originalRPM, *original.User.UserGroupRPMOverride)
	require.Equal(t, exclusions, key.User.GroupAccountExclusions)
	require.True(t, key.Group.GeminiInputCountingEnabled)
	require.Equal(t, 0.75, key.Group.RateMultiplier)
	ctx, _ := WithGatewayTokenRequestPricing(context.WithValue(context.Background(), ctxkey.Group, key.Group))
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1beta/models/gemini-test:generateContent", nil).WithContext(ctx)
	account := &Account{ID: 3, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	require.True(t, requestGeminiInputCountingEnabled(c, account, "gemini-test"), "source discount must be frozen after default source resolution")
	key.User.GroupAccountExclusions[sourceID][0] = 100
	require.Equal(t, int64(27), exclusions[sourceID][0], "request clone must not mutate shared exclusions")
}

func TestCategoryDefaultAPIKeyRejectsRevokedSourcePermission(t *testing.T) {
	entry := int64(51)
	original := &APIKey{ID: 4, UserID: 2, GroupID: &entry, Group: &Group{ID: entry, IsCategoryDefault: true}, User: &User{ID: 2, Status: StatusActive, AllowedGroups: []int64{8}}}
	fresh := &User{ID: 2, Status: StatusActive, RestrictPublicGroups: true}
	resolver := &categoryAPIKeyResolverFake{source: &Group{ID: 8, IsExclusive: true}}
	svc := &APIKeyService{userRepo: categoryAPIKeyUserRepo{user: fresh}, categoryDefaultGroups: resolver}
	_, err := svc.ResolveCategoryDefaultGroupModel(context.Background(), original, "gemini-test")
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	require.Empty(t, resolver.userSeen.AllowedGroups, "must use the fresh user, not the authorized auth-cache snapshot")
	models, err := svc.ListCategoryDefaultGroupModels(context.Background(), original, "")
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestCategoryDefaultAPIKeyDerivedBindingDoesNotRequireEntryAuthorization(t *testing.T) {
	user := &User{ID: 2, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{8}}
	entry := &Group{ID: 51, Platform: PlatformComposite, IsCategoryDefault: true, Status: StatusActive}
	svc := &APIKeyService{categoryDefaultGroups: &categoryAPIKeyResolverFake{source: &Group{ID: 8, IsExclusive: true}}}
	require.False(t, user.CanBindGroup(entry.ID, false))
	require.True(t, svc.canUserBindGroup(context.Background(), user, entry))
	user.AllowedGroups = nil
	require.False(t, svc.canUserBindGroup(context.Background(), user, entry))
}

func TestCategoryDefaultAvailableGroupsDerivesVisibilityAndModelCount(t *testing.T) {
	user := &User{ID: 2, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{8}}
	entry := Group{ID: 51, Platform: PlatformComposite, IsCategoryDefault: true, Status: StatusActive}
	source := Group{ID: 8, Platform: PlatformGemini, IsExclusive: true, Status: StatusActive}
	svc := &APIKeyService{
		userRepo:              categoryAPIKeyUserRepo{user: user},
		groupRepo:             categoryAPIKeyGroups{groups: []Group{entry, source, {ID: 9, Status: StatusActive}}},
		userSubRepo:           categoryAPIKeySubscriptions{},
		categoryDefaultGroups: &categoryAPIKeyResolverFake{source: &source},
	}
	groups, err := svc.GetAvailableGroups(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, entry.ID, groups[0].ID)
	require.Equal(t, 1, groups[0].AvailableModelCount)
	require.Equal(t, source.ID, groups[1].ID)
	user.AllowedGroups = nil
	groups, err = svc.GetAvailableGroups(context.Background(), user.ID)
	require.NoError(t, err)
	require.Empty(t, groups, "revoked source access hides both the source and its derived default entry")
}

func TestCategoryDefaultAuthCacheRoundtripAndRejectsPreviousVersion(t *testing.T) {
	entry := int64(51)
	svc := &APIKeyService{}
	key := &APIKey{ID: 4, UserID: 2, GroupID: &entry, User: &User{ID: 2}, Group: &Group{ID: entry, IsCategoryDefault: true, Platform: PlatformComposite, Hydrated: true}}
	raw, err := json.Marshal(svc.snapshotFromAPIKey(context.Background(), key))
	require.NoError(t, err)
	var snapshot APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	require.True(t, svc.snapshotToAPIKey("secret", &snapshot).Group.IsCategoryDefault)
	require.Equal(t, 31, snapshot.Version)
	restored, hit, err := svc.applyAuthCacheEntry("secret", &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: 30}})
	require.NoError(t, err)
	require.False(t, hit)
	require.Nil(t, restored)
}
