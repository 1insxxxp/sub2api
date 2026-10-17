//go:build unit

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geoip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performIPGeo(t *testing.T, body string, authenticated bool, lookup func([]string) ([]geoip.Result, error)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/ip-geo", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if authenticated {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
	}
	h := &UserHandler{ipGeoLookup: lookup}
	h.GetIPGeo(c)
	require.True(t, c.GetBool("audit_skip"), "read-only lookup must not retain queried IP lists in the audit log")
	return w
}

func TestIPGeoRequiresAuthentication(t *testing.T) {
	w := performIPGeo(t, `{"ips":["8.8.8.8"]}`, false, func([]string) ([]geoip.Result, error) { t.Fatal("unauthenticated lookup"); return nil, nil })
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestIPGeoResponseIsPrivateAndNoStore(t *testing.T) {
	w := performIPGeo(t, `{"ips":["8.8.8.8"]}`, true, func(ips []string) ([]geoip.Result, error) {
		require.Equal(t, []string{"8.8.8.8"}, ips)
		return []geoip.Result{{IP: "8.8.8.8", CountryCode: "US", City: "Mountain View"}}, nil
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	var response struct {
		Data []geoip.Result `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "US", response.Data[0].CountryCode)
}

func TestIPGeoRejectsInvalidOversizedAndTooManyIPs(t *testing.T) {
	tooMany, _ := json.Marshal(map[string]any{"ips": make([]string, 51)})
	bodies := []string{`{}`, `{"ips":[]}`, `{"ips":["host.invalid"]}`, `{"ips":["fe80::1%eth0"]}`, `{"ips":["8.8.8.8:443"]}`, string(tooMany), strings.Repeat(" ", 4097) + `{"ips":["8.8.8.8"]}`}
	for _, body := range bodies {
		w := performIPGeo(t, body, true, func([]string) ([]geoip.Result, error) { t.Fatal("invalid lookup"); return nil, nil })
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestIPGeoUnavailableDoesNotExposeInternalPathsOrFallback(t *testing.T) {
	w := performIPGeo(t, `{"ips":["8.8.8.8"]}`, true, func([]string) ([]geoip.Result, error) { return nil, errors.New("/private/data/secret.mmdb failed") })
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "/private")
	require.Contains(t, w.Body.String(), "Offline IP geolocation is unavailable")
}
