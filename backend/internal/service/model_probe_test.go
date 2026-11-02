package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type modelProbeGroupsStub struct {
	group         *Group
	calls         int
	all           []Group
	visible       []Group
	visibilitySet bool
	visibilityErr error
	actors        []int64
}

func (s *modelProbeGroupsStub) GetByID(context.Context, int64) (*Group, error) {
	s.calls++
	return s.group, nil
}
func (s *modelProbeGroupsStub) ListActive(context.Context) ([]Group, error) {
	s.calls++
	if s.all != nil {
		return s.all, nil
	}
	return []Group{*s.group}, nil
}
func (s *modelProbeGroupsStub) GetAvailableGroups(_ context.Context, actor int64) ([]Group, error) {
	s.actors = append(s.actors, actor)
	if s.visibilityErr != nil {
		return nil, s.visibilityErr
	}
	if s.visibilitySet {
		return s.visible, nil
	}
	return []Group{*s.group}, nil
}

type modelProbeAccessStub struct {
	allowed bool
	calls   int
}

func (s *modelProbeAccessStub) CanAccess(context.Context, int64) (bool, error) {
	s.calls++
	return s.allowed, nil
}

type modelProbeGatewayStub struct {
	models       []string
	catalogCalls int
	selected     int
	released     int
	onCatalog    func()
}

func (s *modelProbeGatewayStub) ModelsForProbe(context.Context, *Group) ([]string, error) {
	s.catalogCalls++
	if s.onCatalog != nil {
		s.onCatalog()
	}
	return s.models, nil
}
func (s *modelProbeGatewayStub) SelectForProbe(context.Context, *Group, string) (*ModelProbeSelection, error) {
	s.selected++
	return &ModelProbeSelection{AccountID: 8, Model: "private-native-model", Release: func() { s.released++ }}, nil
}

type modelProbeRunnerStub struct {
	account int64
	model   string
	result  ModelProbeExecution
	err     error
}

func (s *modelProbeRunnerStub) RunModelProbe(_ context.Context, id int64, model string) (ModelProbeExecution, error) {
	s.account = id
	s.model = model
	return s.result, s.err
}
func modelProbeFixture() (*ModelProbeService, *modelProbeGroupsStub, *modelProbeAccessStub, *modelProbeGatewayStub, *modelProbeRunnerStub) {
	groups := &modelProbeGroupsStub{group: &Group{ID: 7, Name: "Public group", Platform: PlatformGemini, Status: StatusActive, Hydrated: true}}
	access := &modelProbeAccessStub{allowed: true}
	gateway := &modelProbeGatewayStub{models: []string{"public-model"}}
	runner := &modelProbeRunnerStub{result: ModelProbeExecution{Status: "success"}}
	return NewModelProbeService(groups, access, gateway, runner, groups, probeSettingsFor(7), probeRoot()), groups, access, gateway, runner
}
func TestModelProbeDeniedBeforeCatalogOrSelection(t *testing.T) {
	svc, g, a, gw, _ := modelProbeFixture()
	a.allowed = false
	if _, err := svc.Groups(context.Background(), 4); !errors.Is(err, ErrUserSupportForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrUserSupportForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if g.calls != 0 || gw.selected != 0 {
		t.Fatal("denied actor touched infrastructure")
	}
}
func TestModelProbeUnknownModelCannotSelectAccount(t *testing.T) {
	svc, _, _, gw, r := modelProbeFixture()
	if _, err := svc.Run(context.Background(), 4, 7, "secret-native-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("want invalid, got %v", err)
	}
	if gw.selected != 0 || r.account != 0 {
		t.Fatal("unknown public model reached probe")
	}
}
func TestModelProbePublicAliasStaysPublicAndSlotReleased(t *testing.T) {
	svc, _, _, gw, r := modelProbeFixture()
	v, err := svc.Run(context.Background(), 4, 7, "public-model")
	if err != nil || v.Status != "success" || v.Model != "public-model" || v.GroupID != 7 {
		t.Fatalf("unexpected result %#v %v", v, err)
	}
	if r.account != 8 || r.model != "private-native-model" || gw.released != 1 {
		t.Fatal("selected route or slot release lost")
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "private-native") || strings.Contains(string(raw), "account") {
		t.Fatal("private selection escaped DTO")
	}
}
func TestModelProbeRunnerErrorsStayClosedAndRelease(t *testing.T) {
	svc, _, _, gw, r := modelProbeFixture()
	r.err = errors.New("https://secret.example auth sk-private")
	v, err := svc.Run(context.Background(), 4, 7, "public-model")
	if err != nil || v.Status != "failed" || gw.released != 1 {
		t.Fatalf("bad failure %#v %v", v, err)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "sk-private") {
		t.Fatal("raw error leaked")
	}
}
func TestModelProbeDisabledGroupCannotRun(t *testing.T) {
	svc, g, _, gw, _ := modelProbeFixture()
	g.group.Status = StatusDisabled
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("want invalid, got %v", err)
	}
	if gw.selected != 0 {
		t.Fatal("disabled group selected an account")
	}
}
func TestModelProbeCatalogDeduplicatesAndFiltersPrivateLabels(t *testing.T) {
	svc, g, _, gw, _ := modelProbeFixture()
	g.group.Name = "https://private.example"
	gw.models = []string{"public-model", "public-model", "https://private.example/model", ""}
	v, err := svc.Models(context.Background(), 4, 7)
	if err != nil || len(v.Items) != 1 || v.Group.Name != "[REDACTED]" {
		t.Fatalf("unexpected catalog %#v %v", v, err)
	}
}

func TestModelProbeGroupsUseOrdinaryUserVisibility(t *testing.T) {
	svc, g, _, _, _ := modelProbeFixture()
	hidden := *g.group
	hidden.ID = 8
	hidden.Name = "Hidden group"
	g.all = []Group{*g.group, hidden}
	g.visibilitySet = true
	g.visible = []Group{*g.group}
	got, err := svc.Groups(context.Background(), 4)
	if err != nil || len(got) != 1 || got[0].ID != 7 {
		t.Fatalf("ordinary-user scope expected only group7, got %#v %v", got, err)
	}
	if len(g.actors) != 1 || g.actors[0] != 4 {
		t.Fatalf("visibility must use current actor, got %v", g.actors)
	}
}
func TestModelProbeInvisibleGroupIDCannotReadModelsOrRun(t *testing.T) {
	svc, g, _, gw, r := modelProbeFixture()
	g.visibilitySet = true
	g.visible = []Group{}
	if _, err := svc.Models(context.Background(), 4, 7); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("invisible catalog must be invalid, got %v", err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("invisible run must be invalid, got %v", err)
	}
	if gw.catalogCalls != 0 || gw.selected != 0 || r.account != 0 {
		t.Fatal("invisible ID touched model catalog or upstream")
	}
}
func TestModelProbeVisibilityRecheckedAfterGrantRevoked(t *testing.T) {
	svc, g, _, gw, _ := modelProbeFixture()
	if _, err := svc.Models(context.Background(), 4, 7); err != nil {
		t.Fatal(err)
	}
	g.visibilitySet = true
	g.visible = []Group{}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("revoked group must not run, got %v", err)
	}
	if gw.selected != 0 {
		t.Fatal("stale visibility selected an account")
	}
}
func TestModelProbeVisibilityLookupFailureFailsClosed(t *testing.T) {
	svc, g, _, gw, _ := modelProbeFixture()
	g.visibilityErr = errors.New("private repository error")
	if _, err := svc.Groups(context.Background(), 4); !errors.Is(err, ErrUserSupportUnavailable) {
		t.Fatalf("visibility lookup must fail closed, got %v", err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrUserSupportUnavailable) {
		t.Fatalf("visibility lookup must fail closed, got %v", err)
	}
	if gw.catalogCalls != 0 || gw.selected != 0 {
		t.Fatal("failed scope lookup touched infrastructure")
	}
}

func TestModelProbeMissingPolicyDefaultsClosed(t *testing.T) {
	_, groups, access, gateway, runner := modelProbeFixture()
	// A fresh service with no policy dependency must never inherit all models.
	svc := NewModelProbeService(groups, access, gateway, runner, groups, probeSettingsFor(), probeRoot())
	got, err := svc.Groups(context.Background(), 4)
	if err != nil || len(got) != 0 {
		t.Fatalf("missing configuration must expose no groups, got %#v %v", got, err)
	}
	if _, err := svc.Run(context.Background(), 4, 7, "public-model"); !errors.Is(err, ErrModelProbeInvalid) {
		t.Fatalf("missing configuration must deny probes, got %v", err)
	}
	if gateway.catalogCalls != 0 || gateway.selected != 0 || runner.account != 0 {
		t.Fatal("unconfigured group touched catalog or upstream")
	}
}
