//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type traversalTimeoutSettingsRepo struct{ service.SettingRepository }

func (*traversalTimeoutSettingsRepo) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":1}}`, nil
}

func TestModelFirstOutputTraversalTimeoutReturnsProtocolError(t *testing.T) {
	for _, protocol := range []string{"chat", "messages", "gemini"} {
		for _, streaming := range []bool{false, true} {
			t.Run(protocol+map[bool]string{true: "_started", false: "_unstarted"}[streaming], func(t *testing.T) {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil)
				c.Set("sub2api.model_first_output_budget_start", time.Now().Add(-2*time.Second))
				cleanup := service.StartModelFirstOutputTraversal(c, service.NewSettingService(&traversalTimeoutSettingsRepo{}, nil), service.PlatformGemini, "gemini-3.1-pro-preview", true)
				defer cleanup()
				select {
				case <-c.Request.Context().Done():
				case <-time.After(time.Second):
					t.Fatal("traversal budget did not expire")
				}
				if streaming {
					c.Writer.Header().Set("Content-Type", "text/event-stream")
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Flush()
				}
				if !handleModelFirstOutputTraversalTimeout(c, protocol) {
					t.Fatal("budget expiration was not handled")
				}
				wantStatus := http.StatusGatewayTimeout
				if streaming {
					wantStatus = http.StatusOK
					if !strings.Contains(recorder.Body.String(), "data: ") {
						t.Fatalf("started stream needs an SSE error, got %q", recorder.Body.String())
					}
					if protocol == "messages" && !strings.Contains(recorder.Body.String(), "event: error\ndata: ") {
						t.Fatalf("Anthropic needs an explicit error event, got %q", recorder.Body.String())
					}
				}
				if recorder.Code != wantStatus || !strings.Contains(recorder.Body.String(), service.ModelFirstOutputTraversalTimeoutMessage) {
					t.Fatalf("got status=%d body=%s", recorder.Code, recorder.Body.String())
				}
				if protocol == "gemini" && !strings.Contains(recorder.Body.String(), "DEADLINE_EXCEEDED") {
					t.Fatalf("native Gemini needs a Google error code: %s", recorder.Body.String())
				}
			})
		}
	}
}

func TestModelFirstOutputTraversalTimeoutDoesNotReportClientCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil).WithContext(ctx)
	cancel()
	if handleModelFirstOutputTraversalTimeout(c, "chat") || c.Writer.Written() {
		t.Fatal("an ordinary client cancellation must not produce a budget error")
	}
}
