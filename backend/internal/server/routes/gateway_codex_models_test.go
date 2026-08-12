package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayRoutesCodexModelsManifestPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	registered := make(map[string]string)
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet {
			registered[route.Path] = route.Handler
		}
	}

	require.NotEmpty(t, registered["/backend-api/codex/models"], "GET /backend-api/codex/models should be registered")
	require.NotEmpty(t, registered["/v1/models"], "GET /v1/models should be registered")
	require.NotEmpty(t, registered["/models"], "GET /models should be registered")
	require.Equal(t, registered["/v1/models"], registered["/models"], "root alias should use the same platform-aware handler")
}

func TestGatewayRoutesClientVersionDoesNotTurnOrdinaryCompositeIntoSystemManifest(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformComposite)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.144.0", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "list", gjson.Get(recorder.Body.String(), "object").String())
	require.False(t, gjson.Get(recorder.Body.String(), "models").Exists())
}
