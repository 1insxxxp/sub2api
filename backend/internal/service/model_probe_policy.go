package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

const modelProbeMaxModels = 200 // Only validates legacy version 1 policies.
const modelProbeMaxConfigGroups = 500
const modelProbePolicyMaxBytes = 64 << 10

type modelProbeSettings interface {
	GetValue(context.Context, string) (string, error)
	GetMultiple(context.Context, []string) (map[string]string, error)
	SetMultiple(context.Context, map[string]string) error
}
type modelProbeUsers interface {
	GetByID(context.Context, int64) (*User, error)
}
type modelProbePolicy struct {
	Version int  `json:"version"`
	Enabled bool `json:"enabled"`
}
type ModelProbeConfigGroup struct {
	ModelProbeGroup
	Enabled bool `json:"enabled"`
}
type ModelProbeConfig struct {
	Group   ModelProbeGroup `json:"group"`
	Enabled bool            `json:"enabled"`
}
type ModelProbeConfigResult struct {
	GroupID int64 `json:"group_id"`
	Enabled bool  `json:"enabled"`
}
type ModelProbeConfigChange = ModelProbeConfigResult

func modelProbePolicyKey(id int64) string {
	return "admin_model_probe_group_" + strconv.FormatInt(id, 10)
}

func validProbeModels(models []string) bool {
	if models == nil || len(models) > modelProbeMaxModels {
		return false
	}
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		if !validProbeModel(model) || strings.ContainsAny(model, "*?") || seen[model] {
			return false
		}
		seen[model] = true
	}
	return true
}

// A strict legacy nonempty model list enables the group. Current models always
// come from the public directory; newly saved settings contain only a switch.
func decodeProbePolicy(value string) (bool, error) {
	invalid := func() (bool, error) { return false, ErrUserSupportUnavailable }
	if len(value) > modelProbePolicyMaxBytes {
		return invalid()
	}
	d := json.NewDecoder(strings.NewReader(value))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return invalid()
	}
	seen := map[string]bool{}
	var version int
	var enabled *bool
	var models []string
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return invalid()
		}
		seen[key] = true
		switch key {
		case "version":
			err = d.Decode(&version)
		case "enabled":
			err = d.Decode(&enabled)
		case "models":
			err = d.Decode(&models)
		default:
			return invalid()
		}
		if err != nil {
			return invalid()
		}
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return invalid()
	}
	var extra any
	if d.Decode(&extra) != io.EOF || len(seen) != 2 || !seen["version"] {
		return invalid()
	}
	if version == 1 && seen["models"] && validProbeModels(models) {
		return len(models) > 0, nil
	}
	if version == 2 && seen["enabled"] && enabled != nil {
		return *enabled, nil
	}
	return invalid()
}

func (s *ModelProbeService) policy(ctx context.Context, id int64) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	value, err := s.settings.GetValue(ctx, modelProbePolicyKey(id))
	if errors.Is(err, ErrSettingNotFound) {
		return false, nil
	}
	if err != nil {
		return false, ErrUserSupportUnavailable
	}
	return decodeProbePolicy(value)
}

func (s *ModelProbeService) configuredGroups(ctx context.Context, rows []Group) (map[int64]bool, error) {
	configured := map[int64]bool{}
	if s.settings == nil {
		return configured, nil
	}
	keys := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].ID > 0 && rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			keys = append(keys, modelProbePolicyKey(rows[i].ID))
		}
	}
	if len(keys) == 0 {
		return configured, nil
	}
	values, err := s.settings.GetMultiple(ctx, keys)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	for i := range rows {
		value, found := values[modelProbePolicyKey(rows[i].ID)]
		if !found {
			continue
		}
		enabled, err := decodeProbePolicy(value)
		if err != nil {
			return nil, err
		}
		configured[rows[i].ID] = enabled
	}
	return configured, nil
}

func (s *ModelProbeService) requireEnabledGroup(ctx context.Context, id int64) error {
	enabled, err := s.policy(ctx, id)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrModelProbeInvalid
	}
	return nil
}

func (s *ModelProbeService) authorizeConfig(ctx context.Context, actor int64) error {
	if actor <= 0 {
		return ErrUserSupportForbidden
	}
	if s == nil || s.users == nil || s.settings == nil || s.groups == nil {
		return ErrUserSupportUnavailable
	}
	u, err := s.users.GetByID(ctx, actor)
	if errors.Is(err, ErrUserNotFound) {
		return ErrUserSupportForbidden
	}
	if err != nil {
		return ErrUserSupportUnavailable
	}
	if u == nil || u.ID != actor || u.Role != RoleAdmin || u.Status != StatusActive || u.DeletedAt != nil {
		return ErrUserSupportForbidden
	}
	return nil
}

func (s *ModelProbeService) ConfigGroups(ctx context.Context, actor int64) ([]ModelProbeConfigGroup, error) {
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	rows, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	enabled, err := s.configuredGroups(ctx, rows)
	if err != nil {
		return nil, err
	}
	out := make([]ModelProbeConfigGroup, 0, len(rows))
	for i := range rows {
		if rows[i].ID > 0 && rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			out = append(out, ModelProbeConfigGroup{ModelProbeGroup: probeGroupDTO(&rows[i]), Enabled: enabled[rows[i].ID]})
		}
	}
	return out, nil
}

func (s *ModelProbeService) trustedGroup(ctx context.Context, id int64) (*Group, error) {
	if id <= 0 || s.groups == nil {
		return nil, ErrModelProbeInvalid
	}
	g, err := s.groups.GetByID(ctx, id)
	if err != nil || g == nil || g.ID != id || g.Status != StatusActive || !IsGroupContextValid(g) {
		return nil, ErrModelProbeInvalid
	}
	return g, nil
}

func (s *ModelProbeService) publicCatalog(ctx context.Context, g *Group) ([]ModelProbeModel, error) {
	if s.gateway == nil {
		return nil, ErrUserSupportUnavailable
	}
	catalogCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	models, err := s.gateway.ModelsForProbe(catalogCtx, g)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	out := []ModelProbeModel{}
	seen := map[string]bool{}
	for _, model := range models {
		if validProbeModel(model) && !strings.ContainsAny(model, "*?") && !seen[model] {
			out = append(out, ModelProbeModel{ID: model})
			seen[model] = true
		}
	}
	return out, nil
}

func (s *ModelProbeService) Config(ctx context.Context, actor, id int64) (*ModelProbeConfig, error) {
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	g, err := s.trustedGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	enabled, err := s.policy(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ModelProbeConfig{Group: probeGroupDTO(g), Enabled: enabled}, nil
}

func (s *ModelProbeService) SaveConfig(ctx context.Context, actor, id int64, enabled bool) (*ModelProbeConfigResult, error) {
	items, err := s.SaveConfigs(ctx, actor, []ModelProbeConfigChange{{GroupID: id, Enabled: enabled}})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// Validate the whole change set before one atomic upsert. Missing groups remain
// untouched, so operators editing different groups cannot overwrite each other.
func (s *ModelProbeService) SaveConfigs(ctx context.Context, actor int64, changes []ModelProbeConfigChange) ([]ModelProbeConfigResult, error) {
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	if changes == nil || len(changes) > modelProbeMaxConfigGroups {
		return nil, ErrModelProbeInvalid
	}
	out := make([]ModelProbeConfigResult, 0, len(changes))
	if len(changes) == 0 {
		return out, nil
	}
	seen := make(map[int64]bool, len(changes))
	for _, change := range changes {
		if change.GroupID <= 0 || seen[change.GroupID] {
			return nil, ErrModelProbeInvalid
		}
		seen[change.GroupID] = true
	}
	rows, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	active := make(map[int64]bool, len(rows))
	for i := range rows {
		if rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			active[rows[i].ID] = true
		}
	}
	values := make(map[string]string, len(changes))
	for _, change := range changes {
		if !active[change.GroupID] {
			return nil, ErrModelProbeInvalid
		}
		value, err := json.Marshal(modelProbePolicy{Version: 2, Enabled: change.Enabled})
		if err != nil {
			return nil, ErrUserSupportUnavailable
		}
		values[modelProbePolicyKey(change.GroupID)] = string(value)
		out = append(out, change)
	}
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	if s.settings.SetMultiple(ctx, values) != nil {
		return nil, ErrUserSupportUnavailable
	}
	return out, nil
}
