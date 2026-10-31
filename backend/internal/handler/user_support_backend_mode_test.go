package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type supportLoginAccessStub struct {
	enabled bool
	err     error
}

func (s *supportLoginAccessStub) CanAccess(context.Context, int64) (bool, error) {
	return s.enabled, s.err
}

func TestUserSupportBackendModeLoginRemainsRoleLimited(t *testing.T) {
	h := newBackendModeAuthHandler()
	u := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive}
	checker := &supportLoginAccessStub{enabled: true}
	h.userSupport = checker
	p := NewPasskeyHandler(nil, nil, h.settingSvc)
	p.userSupport = checker
	require.NoError(t, h.ensureBackendModeAllowsUser(context.Background(), u))
	require.NoError(t, p.ensureBackendModeAllowsUser(context.Background(), u))
	require.Equal(t, service.RoleUser, u.Role)
	require.False(t, u.CanAccessAdminWorkbench())
	checker.enabled = false
	require.Error(t, h.ensureBackendModeAllowsUser(context.Background(), u))
	require.Error(t, p.ensureBackendModeAllowsUser(context.Background(), u))
	checker.enabled = true
	checker.err = errors.New("private diagnostic")
	require.Error(t, h.ensureBackendModeAllowsUser(context.Background(), u))
	require.Error(t, p.ensureBackendModeAllowsUser(context.Background(), u))
}
