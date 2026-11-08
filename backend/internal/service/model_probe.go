package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
)

var ErrModelProbeUnsupported = errors.New("model requires a different native probe")

var ErrModelProbeInvalid = errors.New("invalid model probe request")

type ModelProbeGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}
type ModelProbeModel struct {
	ID string `json:"id"`
}
type ModelProbeCatalog struct {
	Group ModelProbeGroup   `json:"group"`
	Items []ModelProbeModel `json:"items"`
}
type ModelProbeResult struct {
	GroupID    int64     `json:"group_id"`
	Model      string    `json:"model"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"duration_ms"`
	StartedAt  time.Time `json:"started_at"`
	ErrorCode  string    `json:"error_code,omitempty"`
}
type ModelProbeExecution struct {
	Status    string
	ErrorCode string
}

// ModelProbeSelection is private infrastructure data and must never be serialized.
type ModelProbeSelection struct {
	AccountID int64
	Account   *Account `json:"-"`
	Model     string
	Release   func()
	Context   context.Context
}
type ModelProbeGateway interface {
	ModelsForProbe(context.Context, *Group) ([]string, error)
	ExecuteForProbe(context.Context, *Group, string) (ModelProbeExecution, error)
}
type modelProbeGroups interface {
	GetByID(context.Context, int64) (*Group, error)
	ListActive(context.Context) ([]Group, error)
}
type modelProbeGroupVisibility interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
}
type modelProbeAccess interface {
	CanAccess(context.Context, int64) (bool, error)
}
type ModelProbeService struct {
	groups           modelProbeGroups
	access           modelProbeAccess
	gateway          ModelProbeGateway
	visible          modelProbeGroupVisibility
	settings         modelProbeSettings
	users            modelProbeUsers
	categoryDefaults *CategoryDefaultGroupService
}

func NewModelProbeService(groups modelProbeGroups, access modelProbeAccess, gateway ModelProbeGateway, visible modelProbeGroupVisibility, settings modelProbeSettings, users modelProbeUsers) *ModelProbeService {
	return &ModelProbeService{groups: groups, access: access, gateway: gateway, visible: visible, settings: settings, users: users}
}

func (s *ModelProbeService) SetCategoryDefaultGroupService(defaults *CategoryDefaultGroupService) {
	s.categoryDefaults = defaults
}

func (s *ModelProbeService) categoryDefaultActor(ctx context.Context, actor int64) (*User, error) {
	if s.users == nil || s.categoryDefaults == nil {
		return nil, ErrUserSupportUnavailable
	}
	return s.probeActor(ctx, actor)
}

func (s *ModelProbeService) probeActor(ctx context.Context, actor int64) (*User, error) {
	if s.users == nil {
		return nil, ErrUserSupportUnavailable
	}
	user, err := s.users.GetByID(ctx, actor)
	if err != nil || user == nil || user.ID != actor || user.Status != StatusActive || user.DeletedAt != nil {
		return nil, ErrUserSupportForbidden
	}
	return user, nil
}

func (s *ModelProbeService) categoryDefaultCatalog(ctx context.Context, actor int64, group *Group) ([]ModelProbeModel, error) {
	user, err := s.categoryDefaultActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	models, err := s.categoryDefaults.ModelsForUser(ctx, group.ID, user, "")
	if err != nil {
		return nil, err
	}
	out := make([]ModelProbeModel, 0, len(models))
	for _, model := range models {
		if validProbeModel(model) {
			out = append(out, ModelProbeModel{ID: model})
		}
	}
	return out, nil
}
func (s *ModelProbeService) authorize(ctx context.Context, id int64) error {
	if s == nil || s.access == nil || id <= 0 {
		return ErrUserSupportForbidden
	}
	ok, err := s.access.CanAccess(ctx, id)
	if err != nil {
		return ErrUserSupportUnavailable
	}
	if !ok {
		return ErrUserSupportForbidden
	}
	return nil
}
func probeGroupDTO(g *Group) ModelProbeGroup {
	return ModelProbeGroup{g.ID, PublicUserSupportGroupName(g.Name), g.Platform}
}
func validProbeModel(v string) bool {
	return v != "" && len(v) <= 256 && v == strings.TrimSpace(v) && strings.IndexFunc(v, unicode.IsControl) < 0 && PublicUserSupportGroupName(v) == v
}
func (s *ModelProbeService) Groups(ctx context.Context, actor int64) ([]ModelProbeGroup, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return nil, err
	}
	rows, err := s.availableGroups(ctx, actor)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	configured, err := s.configuredGroups(ctx, rows)
	if err != nil {
		return nil, err
	}
	out := make([]ModelProbeGroup, 0, len(rows))
	for i := range rows {
		if configured[rows[i].ID] && rows[i].ID > 0 && rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			out = append(out, probeGroupDTO(&rows[i]))
		}
	}
	return out, nil
}
func (s *ModelProbeService) availableGroups(ctx context.Context, actor int64) ([]Group, error) {
	if s.visible == nil {
		return nil, ErrUserSupportUnavailable
	}
	rows, err := s.visible.GetAvailableGroups(ctx, actor)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	return rows, nil
}

func (s *ModelProbeService) requireVisibleGroup(ctx context.Context, actor, id int64) error {
	rows, err := s.availableGroups(ctx, actor)
	if err != nil {
		return err
	}
	for i := range rows {
		if rows[i].ID == id && rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			return nil
		}
	}
	return ErrModelProbeInvalid
}

func (s *ModelProbeService) catalog(ctx context.Context, actor, id int64) (*Group, *ModelProbeCatalog, error) {
	if id <= 0 {
		return nil, nil, ErrModelProbeInvalid
	}
	// Use the same actor-scoped directory as the ordinary API-key page on every
	// request. A cached UI selection or a guessed group ID is not authorization.
	if err := s.requireVisibleGroup(ctx, actor, id); err != nil {
		return nil, nil, err
	}
	if err := s.requireEnabledGroup(ctx, id); err != nil {
		return nil, nil, err
	}
	g, err := s.trustedGroup(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var models []ModelProbeModel
	if g.IsCategoryDefault {
		models, err = s.categoryDefaultCatalog(ctx, actor, g)
	} else {
		models, err = s.publicCatalog(ctx, g)
	}
	if err != nil {
		return nil, nil, err
	}
	out := &ModelProbeCatalog{Group: probeGroupDTO(g), Items: models}
	return g, out, nil
}
func (s *ModelProbeService) Models(ctx context.Context, actor, id int64) (*ModelProbeCatalog, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return nil, err
	}
	_, v, err := s.catalog(ctx, actor, id)
	return v, err
}
func (s *ModelProbeService) Run(ctx context.Context, actor, id int64, model string) (*ModelProbeResult, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return nil, err
	}
	if !validProbeModel(model) {
		return nil, ErrModelProbeInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	group, err := s.groupForProbe(ctx, actor, id, model)
	if err != nil {
		return nil, err
	}
	// A manifest refresh can take time. Closing a group during discovery must
	// take effect before actual scheduling, even for an already loaded client.
	if err := s.requireEnabledGroup(ctx, id); err != nil {
		return nil, err
	}
	probeModel := model
	if group.IsCategoryDefault {
		user, err := s.categoryDefaultActor(ctx, actor)
		if err != nil {
			return nil, err
		}
		route, err := s.categoryDefaults.ResolveForUser(ctx, id, user, model)
		if err != nil {
			return nil, err
		}
		if route == nil || route.SourceGroup == nil {
			return nil, ErrModelProbeInvalid
		}
		group, probeModel = route.SourceGroup, route.SourceModel
		ctx = WithGroupAccountExclusions(ctx, user.ID, user.GroupAccountExclusions)
	} else {
		user, err := s.probeActor(ctx, actor)
		if err != nil {
			return nil, err
		}
		ctx = WithGroupAccountExclusions(ctx, user.ID, user.GroupAccountExclusions)
	}
	started := time.Now()
	out := &ModelProbeResult{GroupID: id, Model: model, Status: "failed", StartedAt: started, ErrorCode: "probe_unavailable"}
	defer func() { out.DurationMs = time.Since(started).Milliseconds(); setProbeContextOutcome(ctx, out) }()
	result, err := s.gateway.ExecuteForProbe(ctx, group, probeModel)
	if errors.Is(err, ErrModelProbeUnsupported) {
		result, err = ModelProbeExecution{Status: "unsupported"}, nil
	}
	applyProbeExecution(out, result, err)
	return out, nil
}
func (s *ModelProbeService) groupForProbe(ctx context.Context, actor, id int64, model string) (*Group, error) {
	group, catalog, err := s.catalog(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	for _, item := range catalog.Items {
		if item.ID == model {
			return group, nil
		}
	}
	return nil, ErrModelProbeInvalid
}
func applyProbeExecution(out *ModelProbeResult, result ModelProbeExecution, err error) {
	out.Status = "failed"
	out.ErrorCode = "probe_failed"
	if err != nil {
		return
	}
	switch result.Status {
	case "success":
		out.Status = "success"
		out.ErrorCode = ""
	case "timeout":
		out.Status = "timeout"
		out.ErrorCode = "probe_timeout"
	case "cancelled":
		out.Status = "cancelled"
		out.ErrorCode = "probe_cancelled"
	case "unsupported":
		out.Status = "unsupported"
		out.ErrorCode = "probe_unsupported"
	}
}

func setProbeContextOutcome(ctx context.Context, out *ModelProbeResult) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.Status = "timeout"
		out.ErrorCode = "probe_timeout"
	} else if ctx.Err() != nil {
		out.Status = "cancelled"
		out.ErrorCode = "probe_cancelled"
	}
}

// Concrete provider keeps small interfaces testable while Wire resolves existing services.
func ProvideModelProbeService(groups GroupRepository, access *UserSupportService, gateway ModelProbeGateway, apiKeys *APIKeyService, settings SettingRepository, users UserRepository, defaults *CategoryDefaultGroupService) *ModelProbeService {
	svc := NewModelProbeService(groups, access, gateway, apiKeys, settings, users)
	svc.SetCategoryDefaultGroupService(defaults)
	return svc
}
