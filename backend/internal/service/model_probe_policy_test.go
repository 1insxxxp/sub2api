package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type modelProbeSettingsStub struct {
	values map[string]string
	err    error
	sets   int
}

func (s *modelProbeSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (s *modelProbeSettingsStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	values := map[string]string{}
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			values[key] = v
		}
	}
	return values, nil
}
func (s *modelProbeSettingsStub) Set(_ context.Context, key, value string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	s.sets++
	return nil
}
func probeSettingsFor(ids ...int64) *modelProbeSettingsStub {
	s := &modelProbeSettingsStub{values: map[string]string{}}
	for _, id := range ids {
		s.values[modelProbePolicyKey(id)] = `{"version":1,"models":["public-model"]}`
	}
	return s
}

type modelProbeUserStub struct {
	user  *User
	err   error
	calls int
}

func (s *modelProbeUserStub) GetByID(context.Context, int64) (*User, error) {
	s.calls++
	return s.user, s.err
}
func probeRoot() *modelProbeUserStub {
	return &modelProbeUserStub{user: &User{ID: 4, Role: RoleAdmin, Status: StatusActive}}
}

func TestModelProbePolicyExactIntersectionAndBeforeDiscovery(t *testing.T) {
	svc, _, _, gateway, runner := modelProbeFixture()
	gateway.models = []string{"public-model", "other-model"}
	got, err := svc.Models(context.Background(), 4, 7)
	if err != nil || len(got.Items) != 1 || got.Items[0].ID != "public-model" {
		t.Fatalf("unexpected intersection %#v %v", got, err)
	}
	before := gateway.catalogCalls
	if _, err = svc.Run(context.Background(), 4, 7, "other-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("unconfigured model accepted: %v", err)
	}
	if gateway.catalogCalls != before || gateway.selected != 0 || runner.account != 0 {
		t.Fatal("unconfigured model touched upstream discovery or selection")
	}
}

func TestModelProbePolicyFailuresAreClosed(t *testing.T) {
	for name, value := range map[string]string{
		"empty": "", "broken": "{", "version": `{"version":2,"models":["public-model"]}`,
		"unknown":  `{"version":1,"models":["public-model"],"all":true}`,
		"wildcard": `{"version":1,"models":["*"]}`, "null": `{"version":1,"models":null}`,
		"duplicate": `{"version":1,"models":["public-model","public-model"]}`,
		"trailing":  `{"version":1,"models":[]} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, _, gateway, _ := modelProbeFixture()
			svc.settings = &modelProbeSettingsStub{values: map[string]string{modelProbePolicyKey(7): value}}
			if _, err := svc.Groups(context.Background(), 4); !errors.Is(err, ErrUserSupportUnavailable) {
				t.Fatalf("bad settings not closed: %v", err)
			}
			if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrUserSupportUnavailable) {
				t.Fatalf("bad run settings not closed: %v", err)
			}
			if gateway.catalogCalls != 0 || gateway.selected != 0 {
				t.Fatal("invalid config reached gateway")
			}
		})
	}
	svc, _, _, gateway, _ := modelProbeFixture()
	svc.settings = &modelProbeSettingsStub{err: errors.New("private DB error")}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrUserSupportUnavailable) {
		t.Fatalf("repository failure not closed: %v", err)
	}
	if gateway.catalogCalls != 0 {
		t.Fatal("repository failure reached gateway")
	}
}

func TestModelProbePolicyRevocationDuringDiscoveryStopsSelection(t *testing.T) {
	svc, _, _, gateway, runner := modelProbeFixture()
	settings := probeSettingsFor(7)
	svc.settings = settings
	gateway.onCatalog = func() { settings.values[modelProbePolicyKey(7)] = `{"version":1,"models":[]}` }
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("revoked model accepted: %v", err)
	}
	if gateway.selected != 0 || runner.account != 0 {
		t.Fatal("revoked model selected account")
	}
}

func TestModelProbeConfigRequiresFreshActiveRoot(t *testing.T) {
	for _, u := range []*User{nil, {ID: 4, Role: RoleSubAdmin, Status: StatusActive}, {ID: 4, Role: RoleAdmin, Status: StatusDisabled}, {ID: 4, Role: RoleUser, Status: StatusActive}, {ID: 9, Role: RoleAdmin, Status: StatusActive}} {
		svc, groups, _, gateway, _ := modelProbeFixture()
		svc.users = &modelProbeUserStub{user: u}
		if _, err := svc.ConfigGroups(context.Background(), 4); !errors.Is(err, ErrUserSupportForbidden) {
			t.Fatalf("actor %#v accepted: %v", u, err)
		}
		if _, err := svc.Config(context.Background(), 4, 7); !errors.Is(err, ErrUserSupportForbidden) {
			t.Fatalf("actor %#v read config: %v", u, err)
		}
		if _, err := svc.SaveConfig(context.Background(), 4, 7, []string{}); !errors.Is(err, ErrUserSupportForbidden) {
			t.Fatalf("actor %#v saved: %v", u, err)
		}
		if groups.calls != 0 || gateway.catalogCalls != 0 {
			t.Fatal("forbidden config actor touched groups or gateway")
		}
	}
	svc, _, _, _, _ := modelProbeFixture()
	svc.users = &modelProbeUserStub{err: errors.New("private user repository error")}
	if _, err := svc.ConfigGroups(context.Background(), 4); !errors.Is(err, ErrUserSupportUnavailable) {
		t.Fatalf("unexpected user repository error: %v", err)
	}
}

func TestModelProbeConfigRootDirectoryDoesNotWidenOperationalGroups(t *testing.T) {
	svc, groups, _, _, _ := modelProbeFixture()
	hidden := *groups.group
	hidden.ID = 8
	hidden.Name = "other"
	groups.all = []Group{*groups.group, hidden, {ID: 9, Status: StatusDisabled, Hydrated: true}}
	got, err := svc.ConfigGroups(context.Background(), 4)
	if err != nil || len(got) != 2 {
		t.Fatalf("root config needs all active trusted groups: %#v %v", got, err)
	}
	got, err = svc.Groups(context.Background(), 4)
	if err != nil || len(got) != 1 || got[0].ID != 7 {
		t.Fatalf("config widened runtime visibility: %#v %v", got, err)
	}
}

func TestModelProbeConfigSaveExactAndEmptyClosesWithoutDiscovery(t *testing.T) {
	svc, _, _, gateway, _ := modelProbeFixture()
	settings := probeSettingsFor(7)
	svc.settings = settings
	for _, models := range [][]string{nil, {"*"}, {"public-model", "public-model"}, {"https://secret.invalid"}, {"not-public"}} {
		if _, err := svc.SaveConfig(context.Background(), 4, 7, models); !errors.Is(err, ErrModelProbeInvalid) {
			t.Fatalf("bad models %v accepted: %v", models, err)
		}
	}
	models := make([]string, 201)
	for i := range models {
		models[i] = fmt.Sprintf("model-%d", i)
	}
	if _, err := svc.SaveConfig(context.Background(), 4, 7, models); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("too many models accepted: %v", err)
	}
	got, err := svc.SaveConfig(context.Background(), 4, 7, []string{"public-model"})
	if err != nil || got.GroupID != 7 || len(got.Models) != 1 {
		t.Fatalf("save failed %#v %v", got, err)
	}
	before := gateway.catalogCalls
	got, err = svc.SaveConfig(context.Background(), 4, 7, []string{})
	if err != nil || len(got.Models) != 0 || gateway.catalogCalls != before {
		t.Fatalf("empty close discovered upstream %#v %v", got, err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("closed group ran: %v", err)
	}
	config, err := svc.Config(context.Background(), 4, 7)
	if err != nil || len(config.Models) != 0 || len(config.AvailableModels) != 1 {
		t.Fatalf("config distinction lost %#v %v", config, err)
	}
	raw, _ := json.Marshal(config)
	if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "account") {
		t.Fatalf("private fields escaped %s", raw)
	}
}

func TestModelProbeConfigRechecksRootAfterDiscovery(t *testing.T) {
	svc, _, _, gateway, _ := modelProbeFixture()
	root := probeRoot()
	svc.users = root
	settings := probeSettingsFor(7)
	svc.settings = settings
	gateway.onCatalog = func() { root.user.Role = RoleSubAdmin }
	if _, err := svc.SaveConfig(context.Background(), 4, 7, []string{"public-model"}); !errors.Is(err, ErrUserSupportForbidden) {
		t.Fatalf("downgraded root saved: %v", err)
	}
	if settings.sets != 0 || root.calls < 2 {
		t.Fatal("root was not rechecked before write")
	}
}

func TestModelProbeConfigGroupSavesStayIndependent(t *testing.T) {
	svc, groups, _, _, _ := modelProbeFixture()
	settings := probeSettingsFor(7)
	svc.settings = settings
	if _, err := svc.SaveConfig(context.Background(), 4, 7, []string{"public-model"}); err != nil {
		t.Fatal(err)
	}
	groups.group.ID = 8
	if _, err := svc.SaveConfig(context.Background(), 4, 8, []string{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(settings.values[modelProbePolicyKey(7)], "public-model") || settings.values[modelProbePolicyKey(8)] == "" {
		t.Fatal("one group save overwrote another")
	}
}

func TestModelProbeConfigOversizedSerializedPolicyCannotBeStored(t *testing.T) {
	svc, _, _, gateway, _ := modelProbeFixture()
	settings := probeSettingsFor(7)
	svc.settings = settings
	models := make([]string, 200)
	for i := range models {
		models[i] = fmt.Sprintf("%s-%d", strings.Repeat("&", 100), i)
	}
	gateway.models = models
	if _, err := svc.SaveConfig(context.Background(), 4, 7, models); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("oversized serialized policy accepted: %v", err)
	}
	if settings.sets != 0 {
		t.Fatal("unreadable policy was stored")
	}
}
