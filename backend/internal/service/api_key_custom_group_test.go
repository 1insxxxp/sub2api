package service

import (
	"context"
	"testing"
)

type customGroupRouteRepoStub struct {
	UserCustomGroupRepository
	route *UserCustomGroupModel
	err   error
}

func (s customGroupRouteRepoStub) ResolveModel(context.Context, int64, int64, string) (*UserCustomGroupModel, error) {
	return s.route, s.err
}

type customGroupSourceRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s customGroupSourceRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func TestResolveCustomGroupModelUsesConfiguredSourceGroup(t *testing.T) {
	customGroupID := int64(7)
	originalGroupID := int64(3)
	sourceGroupID := int64(42)
	key := &APIKey{
		UserID:        9,
		GroupID:       &originalGroupID,
		CustomGroupID: &customGroupID,
		User:          &User{ID: 9},
	}
	svc := &APIKeyService{
		customGroupRepo: customGroupRouteRepoStub{route: &UserCustomGroupModel{
			PublicModel:   "claude-sonnet-discount",
			SourceModel:   "claude-sonnet-4-5",
			SourceGroupID: sourceGroupID,
		}},
		groupRepo: customGroupSourceRepoStub{groups: map[int64]*Group{
			sourceGroupID: {ID: sourceGroupID, Status: StatusActive, Platform: PlatformAnthropic},
		}},
	}

	resolution, err := svc.ResolveCustomGroupModel(context.Background(), key, "claude-sonnet-discount")
	if err != nil {
		t.Fatalf("ResolveCustomGroupModel() error = %v", err)
	}
	resolved := resolution.APIKey
	if resolved == key {
		t.Fatal("ResolveCustomGroupModel() returned the original key instead of a request-scoped clone")
	}
	if resolved.GroupID == nil || *resolved.GroupID != sourceGroupID {
		t.Fatalf("resolved GroupID = %v, want %d", resolved.GroupID, sourceGroupID)
	}
	if key.GroupID == nil || *key.GroupID != originalGroupID {
		t.Fatalf("original key GroupID was mutated: %v", key.GroupID)
	}
	if resolved.CustomGroupID == nil || *resolved.CustomGroupID != customGroupID {
		t.Fatalf("resolved CustomGroupID = %v, want %d", resolved.CustomGroupID, customGroupID)
	}
	if resolution.PublicModel != "claude-sonnet-discount" {
		t.Fatalf("resolution PublicModel = %q, want alias", resolution.PublicModel)
	}
	if resolution.SourceModel != "claude-sonnet-4-5" {
		t.Fatalf("resolution SourceModel = %q, want real model", resolution.SourceModel)
	}
}

func TestResolveCustomGroupModelRejectsUnavailableSourceWithoutFallback(t *testing.T) {
	customGroupID := int64(7)
	sourceGroupID := int64(42)
	key := &APIKey{UserID: 9, CustomGroupID: &customGroupID, User: &User{ID: 9}}
	svc := &APIKeyService{
		customGroupRepo: customGroupRouteRepoStub{route: &UserCustomGroupModel{SourceGroupID: sourceGroupID}},
		groupRepo: customGroupSourceRepoStub{groups: map[int64]*Group{
			sourceGroupID: {ID: sourceGroupID, Status: StatusDisabled, Platform: PlatformAnthropic},
		}},
	}

	resolution, err := svc.ResolveCustomGroupModel(context.Background(), key, "claude-sonnet-4-5")
	if err == nil {
		t.Fatal("ResolveCustomGroupModel() error = nil, want unavailable source error")
	}
	if resolution != nil {
		t.Fatalf("ResolveCustomGroupModel() resolution = %#v, want nil (no fallback)", resolution)
	}
}
