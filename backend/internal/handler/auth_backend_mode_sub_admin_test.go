//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type backendModeSettingRepoStub struct {
	service.SettingRepository
	values map[string]string
}

func (s *backendModeSettingRepoStub) Get(_ context.Context, key string) (*service.Setting, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, service.ErrSettingNotFound
	}
	return &service.Setting{Key: key, Value: value}, nil
}

func (s *backendModeSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (s *backendModeSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *backendModeSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func newBackendModeAuthHandler() *AuthHandler {
	return &AuthHandler{
		settingSvc: service.NewSettingService(&backendModeSettingRepoStub{
			values: map[string]string{service.SettingKeyBackendModeEnabled: "true"},
		}, &config.Config{}),
	}
}

func TestAuthHandlerBackendModeAllowsSubAdminLogin(t *testing.T) {
	h := newBackendModeAuthHandler()

	err := h.ensureBackendModeAllowsUser(context.Background(), &service.User{
		ID:     10,
		Role:   service.RoleSubAdmin,
		Status: service.StatusActive,
	})

	require.NoError(t, err)
}

func TestAuthHandlerBackendModeStillBlocksRegularUserLogin(t *testing.T) {
	h := newBackendModeAuthHandler()

	err := h.ensureBackendModeAllowsUser(context.Background(), &service.User{
		ID:     11,
		Role:   service.RoleUser,
		Status: service.StatusActive,
	})

	require.Error(t, err)
	require.Equal(t, "BACKEND_MODE_MANAGER_ONLY", infraerrors.Reason(err))
}

func TestPasskeyHandlerBackendModeAllowsSubAdminLogin(t *testing.T) {
	auth := newBackendModeAuthHandler()
	h := &PasskeyHandler{settingSvc: auth.settingSvc}

	err := h.ensureBackendModeAllowsUser(context.Background(), &service.User{
		ID:     12,
		Role:   service.RoleSubAdmin,
		Status: service.StatusActive,
	})

	require.NoError(t, err)
}
