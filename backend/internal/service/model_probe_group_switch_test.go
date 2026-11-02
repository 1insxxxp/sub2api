package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestModelProbeGroupSwitchUsesWholeSafeCatalog(t *testing.T) {
	svc, _, _, gateway, runner := modelProbeFixture()
	// A valid legacy nonempty policy enables the group, without retaining the
	// old model restriction. The current public directory remains authoritative.
	gateway.models = []string{"public-model", "other-public-model", "https://secret.invalid/model"}
	got, err := svc.Models(context.Background(), 4, 7)
	if err != nil || len(got.Items) != 2 {
		t.Fatalf("enabled group must expose both safe public models: %#v %v", got, err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "other-public-model"); err != nil {
		t.Fatalf("enabled group rejected current public model: %v", err)
	}
	if gateway.selected != 1 || runner.account == 0 {
		t.Fatal("current public model did not reach the existing native probe")
	}
}

func TestModelProbeGroupSwitchVersionedPolicies(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		enabled bool
	}{
		{`{"version":2,"enabled":true}`, true}, {`{"version":2,"enabled":false}`, false},
		{`{"version":1,"models":["public-model"]}`, true}, {`{"version":1,"models":[]}`, false},
	} {
		enabled, err := decodeProbePolicy(tc.raw)
		if err != nil || enabled != tc.enabled {
			t.Fatalf("policy %s got %v %v", tc.raw, enabled, err)
		}
	}
	for _, raw := range []string{
		`{"version":2}`, `{"version":2,"enabled":null}`, `{"version":2,"enabled":"true"}`,
		`{"version":2,"enabled":true,"models":[]}`, `{"version":2,"enabled":false,"enabled":true}`,
		`{"version":2,"Enabled":true}`, `{"version":1,"models":[],"models":["public-model"]}`,
		`{"version":1,"models":null}`, `{"version":3,"enabled":true}`, strings.Repeat(" ", 65537),
	} {
		if _, err := decodeProbePolicy(raw); !errors.Is(err, ErrUserSupportUnavailable) {
			t.Fatalf("invalid policy accepted: %s %v", raw[:min(len(raw), 100)], err)
		}
	}
}

func TestModelProbeGroupSwitchConfigListsClosedGroups(t *testing.T) {
	svc, groups, _, gateway, _ := modelProbeFixture()
	closed := *groups.group
	closed.ID = 8
	groups.all = []Group{*groups.group, closed}
	got, err := svc.ConfigGroups(context.Background(), 4)
	if err != nil || len(got) != 2 || !got[0].Enabled || got[1].Enabled {
		t.Fatalf("wrong config directory: %#v %v", got, err)
	}
	if gateway.catalogCalls != 0 {
		t.Fatal("config directory discovered models")
	}
}

func TestModelProbeGroupSwitchBulkWritesAtomicallyAndPreservesUnchanged(t *testing.T) {
	svc, groups, _, gateway, _ := modelProbeFixture()
	other := *groups.group
	other.ID = 8
	groups.all = []Group{*groups.group, other}
	settings := probeSettingsFor(7, 8, 9)
	svc.settings = settings
	unchanged := settings.values[modelProbePolicyKey(9)]
	got, err := svc.SaveConfigs(context.Background(), 4, []ModelProbeConfigChange{{GroupID: 7, Enabled: false}, {GroupID: 8, Enabled: true}})
	if err != nil || len(got) != 2 || got[0].Enabled || !got[1].Enabled {
		t.Fatalf("bulk save failed %#v %v", got, err)
	}
	if settings.multiCalls != 1 || settings.sets != 0 || gateway.catalogCalls != 0 {
		t.Fatal("bulk save was not one directory-free atomic upsert")
	}
	if settings.values[modelProbePolicyKey(9)] != unchanged {
		t.Fatal("changes-only save replaced an unchanged group")
	}
	for id, want := range map[int64]bool{7: false, 8: true} {
		got, err := decodeProbePolicy(settings.values[modelProbePolicyKey(id)])
		if err != nil || got != want {
			t.Fatalf("saved group %d wrong: %v %v", id, got, err)
		}
	}
	settings.setErr = errors.New("private writer failure")
	before := settings.values[modelProbePolicyKey(7)]
	if _, err := svc.SaveConfigs(context.Background(), 4, []ModelProbeConfigChange{{GroupID: 7, Enabled: true}, {GroupID: 8, Enabled: false}}); !errors.Is(err, ErrUserSupportUnavailable) {
		t.Fatalf("writer error escaped: %v", err)
	}
	if settings.values[modelProbePolicyKey(7)] != before {
		t.Fatal("failed bulk writer partially applied changes")
	}
}

func TestModelProbeGroupSwitchBulkValidationAndNoop(t *testing.T) {
	svc, groups, _, _, _ := modelProbeFixture()
	settings := probeSettingsFor(7)
	svc.settings = settings
	got, err := svc.SaveConfigs(context.Background(), 4, []ModelProbeConfigChange{})
	if err != nil || got == nil || len(got) != 0 || settings.multiCalls != 0 || groups.calls != 0 {
		t.Fatalf("empty changes must be a no-op: %#v %v", got, err)
	}
	for _, changes := range [][]ModelProbeConfigChange{nil, {{GroupID: 0}}, {{GroupID: 7}, {GroupID: 7}}, {{GroupID: 7}, {GroupID: 8}}, make([]ModelProbeConfigChange, 501)} {
		if _, err := svc.SaveConfigs(context.Background(), 4, changes); !errors.Is(err, ErrModelProbeInvalid) {
			t.Fatalf("invalid changes accepted: %v", err)
		}
	}
	groups.group.Status = StatusDisabled
	if _, err := svc.SaveConfig(context.Background(), 4, 7, false); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("disabled group accepted: %v", err)
	}
	groups.group.Status = StatusActive
	groups.group.Hydrated = false
	if _, err := svc.SaveConfig(context.Background(), 4, 7, true); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("untrusted group accepted: %v", err)
	}
	if settings.multiCalls != 0 {
		t.Fatal("invalid changes reached writer")
	}
}

func TestModelProbeGroupSwitchBulkRechecksFreshRootBeforeWrite(t *testing.T) {
	for _, mutation := range []func(*User){func(u *User) { u.Role = RoleSubAdmin }, func(u *User) { u.Status = StatusDisabled }, func(u *User) { now := time.Now(); u.DeletedAt = &now }} {
		svc, groups, _, _, _ := modelProbeFixture()
		root := probeRoot()
		svc.users = root
		settings := probeSettingsFor(7)
		svc.settings = settings
		groups.onList = func() { mutation(root.user) }
		if _, err := svc.SaveConfigs(context.Background(), 4, []ModelProbeConfigChange{{GroupID: 7, Enabled: true}}); !errors.Is(err, ErrUserSupportForbidden) {
			t.Fatalf("stale root wrote config: %v", err)
		}
		if settings.multiCalls != 0 || root.calls < 2 {
			t.Fatal("root was not freshly checked before write")
		}
	}
}

func TestModelProbeGroupSwitchConfigDoesNotDiscoverModels(t *testing.T) {
	svc, _, _, gateway, _ := modelProbeFixture()
	got, err := svc.Config(context.Background(), 4, 7)
	if err != nil {
		t.Fatal(err)
	}
	if gateway.catalogCalls != 0 {
		t.Fatal("group configuration queried upstream model discovery")
	}
	raw, _ := json.Marshal(got)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["enabled"]) != "true" || fields["models"] != nil || fields["available_models"] != nil {
		t.Fatalf("configuration must contain group and enabled only: %s", raw)
	}
}
