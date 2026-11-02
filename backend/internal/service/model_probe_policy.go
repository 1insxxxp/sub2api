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

const modelProbeMaxModels = 200
const modelProbePolicyMaxBytes = 64 << 10

type modelProbeSettings interface {
	GetValue(context.Context, string) (string, error)
	GetMultiple(context.Context, []string) (map[string]string, error)
	Set(context.Context, string, string) error
}
type modelProbeUsers interface {
	GetByID(context.Context, int64) (*User, error)
}
type modelProbePolicy struct {
	Version int      `json:"version"`
	Models  []string `json:"models"`
}
type ModelProbeConfig struct {
	Group           ModelProbeGroup   `json:"group"`
	Models          []string          `json:"models"`
	AvailableModels []ModelProbeModel `json:"available_models"`
}
type ModelProbeConfigResult struct {
	GroupID int64    `json:"group_id"`
	Models  []string `json:"models"`
}

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

func decodeProbePolicy(value string) ([]string, error) {
	if len(value) > modelProbePolicyMaxBytes {
		return nil, ErrUserSupportUnavailable
	}
	var policy modelProbePolicy
	d := json.NewDecoder(strings.NewReader(value))
	d.DisallowUnknownFields()
	if d.Decode(&policy) != nil || policy.Version != 1 || !validProbeModels(policy.Models) {
		return nil, ErrUserSupportUnavailable
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, ErrUserSupportUnavailable
	}
	return policy.Models, nil
}

func (s *ModelProbeService) policy(ctx context.Context, id int64) ([]string, error) {
	if s.settings == nil {
		return []string{}, nil
	}
	value, err := s.settings.GetValue(ctx, modelProbePolicyKey(id))
	if errors.Is(err, ErrSettingNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, ErrUserSupportUnavailable
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
		models, err := decodeProbePolicy(value)
		if err != nil {
			return nil, err
		}
		configured[rows[i].ID] = len(models) > 0
	}
	return configured, nil
}

func probePolicyContains(models []string, model string) bool {
	for _, candidate := range models {
		if model == candidate {
			return true
		}
	}
	return false
}

func (s *ModelProbeService) requireConfiguredModel(ctx context.Context, id int64, model string) error {
	models, err := s.policy(ctx, id)
	if err != nil {
		return err
	}
	if !probePolicyContains(models, model) {
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

func (s *ModelProbeService) ConfigGroups(ctx context.Context, actor int64) ([]ModelProbeGroup, error) {
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	rows, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	out := make([]ModelProbeGroup, 0, len(rows))
	for i := range rows {
		if rows[i].ID > 0 && rows[i].Status == StatusActive && IsGroupContextValid(&rows[i]) {
			out = append(out, probeGroupDTO(&rows[i]))
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
	models, err := s.policy(ctx, id)
	if err != nil {
		return nil, err
	}
	available, err := s.publicCatalog(ctx, g)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	return &ModelProbeConfig{Group: probeGroupDTO(g), Models: models, AvailableModels: available}, nil
}

func (s *ModelProbeService) SaveConfig(ctx context.Context, actor, id int64, models []string) (*ModelProbeConfigResult, error) {
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	if !validProbeModels(models) {
		return nil, ErrModelProbeInvalid
	}
	g, err := s.trustedGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(models) > 0 {
		available, err := s.publicCatalog(ctx, g)
		if err != nil {
			return nil, err
		}
		allowed := make(map[string]bool, len(available))
		for _, item := range available {
			allowed[item.ID] = true
		}
		for _, model := range models {
			if !allowed[model] {
				return nil, ErrModelProbeInvalid
			}
		}
	}
	// Directory discovery may wait on an upstream manifest. A stale root role or
	// a group disabled during that wait must not authorize a subsequent write.
	if err := s.authorizeConfig(ctx, actor); err != nil {
		return nil, err
	}
	if _, err := s.trustedGroup(ctx, id); err != nil {
		return nil, err
	}
	copyModels := append([]string{}, models...)
	value, err := json.Marshal(modelProbePolicy{Version: 1, Models: copyModels})
	if err != nil {
		return nil, ErrUserSupportUnavailable
	}
	if len(value) > modelProbePolicyMaxBytes {
		return nil, ErrModelProbeInvalid
	}
	if s.settings.Set(ctx, modelProbePolicyKey(id), string(value)) != nil {
		return nil, ErrUserSupportUnavailable
	}
	return &ModelProbeConfigResult{GroupID: id, Models: copyModels}, nil
}
