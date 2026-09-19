//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestGroupTagCreateUpdateAndClear(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 71}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "tagged", Platform: PlatformAnthropic, RateMultiplier: 1, Tag: "chat",
	})
	require.NoError(t, err)
	require.Equal(t, "chat", repo.created.Tag)
	repo.getByID = group

	for _, tag := range []string{"image", "airp", ""} {
		updated, err := svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{Tag: &tag})
		require.NoError(t, err)
		require.Equal(t, tag, updated.Tag)
		require.Equal(t, tag, repo.updated.Tag)
		repo.getByID = updated
	}
}

func TestGroupTagOmissionPreservesExistingTag(t *testing.T) {
	repo := &groupRepoStubForAdmin{getByID: &Group{ID: 72, Name: "tagged", Platform: PlatformAnthropic, Tag: "airp"}}
	svc := &adminServiceImpl{groupRepo: repo}
	updated, err := svc.UpdateGroup(context.Background(), 72, &UpdateGroupInput{})
	require.NoError(t, err)
	require.Equal(t, "airp", updated.Tag)
}

func TestGroupTagAcceptsCustomUnicodeValues(t *testing.T) {
	for _, tag := range []string{"", "chat", "image", "airp", "AIRP", "chat,image", "自定义标签", strings.Repeat("图", 20), strings.Repeat("\U0001f680", 20)} {
		t.Run(tag, func(t *testing.T) {
			require.NoError(t, ValidateGroupTag(tag))
		})
	}
}

func TestGroupTagRejectsInvalidValues(t *testing.T) {
	for _, tag := range []string{strings.Repeat("a", 21), strings.Repeat("图", 21), strings.Repeat("\U0001f680", 21), "a\nb", "a\rb", "a\tb", "\ntag", "tag\n", "a\x00b", "a\x7fb", "a\u0085b", "\xff"} {
		t.Run(tag, func(t *testing.T) {
			require.Error(t, ValidateGroupTag(tag))
		})
	}
}

func TestGroupTagTrimsCreateAndUpdate(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 73}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "custom", Platform: PlatformAnthropic, RateMultiplier: 1, Tag: "  自定义标签  ",
	})
	require.NoError(t, err)
	require.Equal(t, "自定义标签", group.Tag)
	repo.getByID = group
	tag := "\u3000更新标签\u3000"
	updated, err := svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{Tag: &tag})
	require.NoError(t, err)
	require.Equal(t, "更新标签", updated.Tag)
}

func TestGroupTagColorLifecycle(t *testing.T) {
	for _, simple := range []bool{false, true} {
		t.Run(map[bool]string{false: "standard", true: "simple"}[simple], func(t *testing.T) {
			repo := &groupRepoStubForAdmin{createID: 74}
			svc := &adminServiceImpl{groupRepo: repo}
			if simple {
				svc.cfg = &config.Config{RunMode: config.RunModeSimple}
			}
			group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
				Name: "custom", Platform: PlatformAnthropic, RateMultiplier: 1,
				Tag: "  新标签  ", TagColor: " #aB12Ef ",
			})
			require.NoError(t, err)
			require.Equal(t, "新标签", group.Tag)
			require.Equal(t, "#aB12Ef", repo.created.TagColor)
			for _, step := range []struct {
				input      UpdateGroupInput
				tag, color string
			}{
				{UpdateGroupInput{}, "新标签", "#aB12Ef"},
				{UpdateGroupInput{Tag: ptrString("改名")}, "改名", "#aB12Ef"},
				{UpdateGroupInput{TagColor: ptrString("")}, "改名", ""},
				{UpdateGroupInput{TagColor: ptrString("#102030")}, "改名", "#102030"},
				{UpdateGroupInput{Tag: ptrString("  ")}, "", ""},
				{UpdateGroupInput{TagColor: ptrString("#102030")}, "", ""},
				{UpdateGroupInput{Tag: ptrString("chat")}, "chat", ""},
				{UpdateGroupInput{Tag: ptrString(""), TagColor: ptrString("#102030")}, "", ""},
			} {
				repo.getByID = group
				group, err = svc.UpdateGroup(context.Background(), group.ID, &step.input)
				require.NoError(t, err)
				require.Equal(t, step.tag, group.Tag)
				require.Equal(t, step.color, repo.updated.TagColor)
			}
		})
	}
}

func TestGroupTagInvalidInputDoesNotPersist(t *testing.T) {
	for _, tc := range []struct{ tag, color, reason string }{
		{"a\nb", "", "INVALID_GROUP_TAG"},
		{"\ntag", "", "INVALID_GROUP_TAG"},
		{"tag\n", "", "INVALID_GROUP_TAG"},
		{"a\u2028b", "", "INVALID_GROUP_TAG"},
		{"a\u2029b", "", "INVALID_GROUP_TAG"},
		{strings.Repeat("图", 21), "", "INVALID_GROUP_TAG"},
		{"\xff", "", "INVALID_GROUP_TAG"},
		{"chat", "red", "INVALID_GROUP_TAG_COLOR"},
		{"chat", "#abc", "INVALID_GROUP_TAG_COLOR"},
		{"chat", "#12345678", "INVALID_GROUP_TAG_COLOR"},
		{"chat", "123456", "INVALID_GROUP_TAG_COLOR"},
		{"chat", "#12345g", "INVALID_GROUP_TAG_COLOR"},
		{"", "not-a-color", "INVALID_GROUP_TAG_COLOR"},
	} {
		t.Run(tc.tag+tc.color, func(t *testing.T) {
			repo := &groupRepoStubForAdmin{getByID: &Group{ID: 75, Platform: PlatformAnthropic, Tag: "chat", TagColor: "#123456"}}
			svc := &adminServiceImpl{groupRepo: repo}
			_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
				Name: "invalid", Platform: PlatformAnthropic, RateMultiplier: 1, Tag: tc.tag, TagColor: tc.color,
			})
			require.Equal(t, tc.reason, infraerrors.Reason(err))
			require.Nil(t, repo.created)
			_, err = svc.UpdateGroup(context.Background(), 75, &UpdateGroupInput{Tag: &tc.tag, TagColor: &tc.color})
			require.Equal(t, tc.reason, infraerrors.Reason(err))
			require.Nil(t, repo.updated)
			require.Equal(t, "chat", repo.getByID.Tag)
			require.Equal(t, "#123456", repo.getByID.TagColor)
		})
	}
}

func TestGroupTagCreateClearsOrphanColor(t *testing.T) {
	repo := &groupRepoStubForAdmin{}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "untagged", Platform: PlatformAnthropic, RateMultiplier: 1, Tag: "  ", TagColor: "#ffffff",
	})
	require.NoError(t, err)
	require.Empty(t, group.Tag)
	require.Empty(t, group.TagColor)
}

func TestSystemCustomGroupTagColorLifecycle(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{}
	svc := NewSystemCustomGroupService(repo, systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
		10: activeDirectSystemCustomSource(10, PlatformAnthropic),
	}}, nil)
	created, err := svc.Create(context.Background(), CreateSystemCustomGroupRequest{
		Name: "tagged subscription", SourceGroupIDs: []int64{10}, Tag: "  自定义  ", TagColor: "#Abc123",
	})
	require.NoError(t, err)
	require.Equal(t, "自定义", created.Group.Tag)
	require.Equal(t, "#Abc123", created.Group.TagColor)
	repo.stored = created
	for _, step := range []struct {
		tag, color         *string
		wantTag, wantColor string
	}{
		{nil, nil, "自定义", "#Abc123"},
		{ptrString(" 新名 "), nil, "新名", "#Abc123"},
		{nil, ptrString(""), "新名", ""},
		{nil, ptrString("#010203"), "新名", "#010203"},
		{ptrString(""), nil, "", ""},
	} {
		updated, err := svc.Update(context.Background(), created.Group.ID, UpdateSystemCustomGroupRequest{
			Name: created.Group.Name, SourceGroupIDs: []int64{10}, Tag: step.tag, TagColor: step.color,
		})
		require.NoError(t, err)
		require.Equal(t, step.wantTag, updated.Group.Tag)
		require.Equal(t, step.wantColor, repo.updatedGroup.TagColor)
	}
}

func TestSystemCustomGroupRejectsInvalidTagColor(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{stored: &SystemCustomGroup{Group: Group{
		ID: 76, Name: "custom", Platform: PlatformComposite, SystemCustomRoutingEnabled: true,
		SubscriptionType: SubscriptionTypeSubscription,
		Tag:              "chat", TagColor: "#123456",
	}}}
	svc := NewSystemCustomGroupService(repo, systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
		10: activeDirectSystemCustomSource(10, PlatformAnthropic),
	}}, nil)
	_, err := svc.Create(context.Background(), CreateSystemCustomGroupRequest{
		Name: "invalid", SourceGroupIDs: []int64{10}, Tag: "chat", TagColor: "#abc",
	})
	require.Equal(t, "INVALID_GROUP_TAG_COLOR", infraerrors.Reason(err))
	require.Nil(t, repo.createdGroup)
	_, err = svc.Update(context.Background(), 76, UpdateSystemCustomGroupRequest{
		Name: "custom", SourceGroupIDs: []int64{10}, TagColor: ptrString("red"),
	})
	require.Equal(t, "INVALID_GROUP_TAG_COLOR", infraerrors.Reason(err))
	require.Nil(t, repo.updatedGroup)
	require.Equal(t, "#123456", repo.stored.Group.TagColor)
}
