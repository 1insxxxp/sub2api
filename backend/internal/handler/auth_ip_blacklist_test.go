package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authIPBlacklistSettingRepoStub struct {
	values map[string]string
}

func (s *authIPBlacklistSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *authIPBlacklistSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *authIPBlacklistSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *authIPBlacklistSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *authIPBlacklistSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *authIPBlacklistSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *authIPBlacklistSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestAuthHandlerEnsureAuthIPAllowedBlocksCFConnectingIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/send-verify-code", nil)
	c.Request.Header.Set("CF-Connecting-IP", "45.207.193.151")

	h := &AuthHandler{
		settingSvc: service.NewSettingService(&authIPBlacklistSettingRepoStub{values: map[string]string{
			service.SettingKeyAuthIPBlacklistEnabled: "true",
			service.SettingKeyAuthIPBlacklistRules:   `["45.207.193.151"]`,
		}}, nil),
	}

	err := h.ensureAuthIPAllowed(c)
	require.ErrorIs(t, err, service.ErrAuthIPBlocked)
}
