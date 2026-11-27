package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicGatewaySettingsRepo struct {
	service.SettingRepository
	settings string
	reads    int
}

func (r *publicGatewaySettingsRepo) GetValue(context.Context, string) (string, error) {
	r.reads++
	return r.settings, nil
}

func TestPublicGatewayConfiguredErrors(t *testing.T) {
	settings := service.DefaultPublicGatewayErrorSettings()
	settings.Rules = []service.PublicGatewayErrorRule{{ID: "custom", Name: "自定义中断提示", Enabled: true, Pattern: "incomplete_stream", Status: 503, Code: "stream_interrupted", Message: "生成中断，请重试。"}}
	// A catch-all makes trusted local error precedence observable.
	settings.Fallback = service.PublicGatewayError{Status: 502, Code: "custom_failure", Message: "请稍后再试。"}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, path string
		status     int
		body, want string
		wantStatus int
		wantReads  int
	}{
		{"http", "/v1/chat/completions", 502, `{"error":{"message":"incomplete_stream private.invalid"}}`, "stream_interrupted", 503, 1},
		{"gemini", "/v1beta/models/test", 502, `{"error":{"code":502,"message":"incomplete_stream private.invalid"}}`, "INTERNAL", 503, 1},
		{"sse", "/v1/chat/completions", 200, "data: {\"error\":{\"code\":\"incomplete_stream\",\"message\":\"private.invalid\"}}\n\n", "stream_interrupted", 200, 1},
		{"responses", "/v1/responses", 200, "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"incomplete_stream\"}}}\n\n", "stream_interrupted", 200, 1},
		{"async task", "/v1/videos/job", 200, `{"status":"failed","error":{"code":"incomplete_stream","message":"private.invalid"}}`, "stream_interrupted", 200, 1},
		{"success", "/v1/chat/completions", 200, `{"choices":[{"message":{"content":"hello"}}]}`, "hello", 200, 0},
		{"successful stream", "/v1/chat/completions", 200, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n", "hello", 200, 0},
		{"local balance", "/v1/chat/completions", 402, `{"error":{"code":"INSUFFICIENT_BALANCE","message":"insufficient balance"}}`, "insufficient_balance", 402, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &publicGatewaySettingsRepo{settings: string(raw)}
			svc := service.NewSettingService(repo, &config.Config{})
			engine := gin.New()
			engine.Use(PublicGatewayErrorMiddleware(false, svc))
			engine.GET(tc.path, func(c *gin.Context) {
				if tc.name == "sse" || tc.name == "responses" || tc.name == "successful stream" {
					c.Data(tc.status, "text/event-stream", []byte(tc.body))
					return
				}
				c.Data(tc.status, "application/json", []byte(tc.body))
			})
			out := httptest.NewRecorder()
			engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, tc.wantStatus, out.Code)
			require.Contains(t, out.Body.String(), tc.want)
			require.NotContains(t, out.Body.String(), "private.invalid")
			require.Equal(t, tc.wantReads, repo.reads)
		})
	}
}

func TestPublicGatewayConfiguredWSFailover(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		want    string
	}{
		{"short reason", "请求繁忙，请稍后重试。", "请求繁忙，请稍后重试。"},
		{"long UTF8 reason", "请求繁忙 " + strings.Repeat("中", 45), "请求繁忙 " + strings.Repeat("中", 35)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testPublicGatewayConfiguredWSFailover(t, tc.message, tc.want)
		})
	}
}

func testPublicGatewayConfiguredWSFailover(t *testing.T, message, want string) {
	t.Helper()
	settings := service.DefaultPublicGatewayErrorSettings()
	settings.Rules = []service.PublicGatewayErrorRule{{ID: "rate_limit", Name: "上游限流", Enabled: true, UpstreamStatuses: []int{429}, Status: 503, Code: "retry_later", Message: message}}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	repo := &publicGatewaySettingsRepo{settings: string(raw)}
	svc := service.NewSettingService(repo, &config.Config{})
	reads := make(chan int, 1)
	router := gin.New()
	router.Use(PublicGatewayErrorMiddleware(false, svc))
	router.GET("/v1/responses", func(c *gin.Context) {
		conn, acceptErr := coderws.Accept(c.Writer, c.Request, nil)
		if acceptErr != nil {
			return
		}
		closeOpenAIWSFailoverExhausted(c, conn, &service.UpstreamFailoverError{StatusCode: 429})
		reads <- repo.reads
	})
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	_, _, err = conn.Read(ctx)
	var closeErr coderws.CloseError
	require.ErrorAs(t, err, &closeErr)
	require.Equal(t, coderws.StatusTryAgainLater, closeErr.Code)
	require.Equal(t, want, closeErr.Reason)
	select {
	case count := <-reads:
		require.Equal(t, 1, count)
	case <-ctx.Done():
		t.Fatal("websocket close did not finish")
	}
}

func TestPublicGatewayWSClientCloseReason(t *testing.T) {
	settings := service.DefaultPublicGatewayErrorSettings()
	settings.Fallback.Message = "响应中断，请重试。"
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		status coderws.StatusCode
		cause  error
		want   string
		reads  int
	}{
		{"idle close", coderws.StatusNormalClosure, context.DeadlineExceeded, "", 0},
		{"client canceled", coderws.StatusGoingAway, context.Canceled, "", 0},
		{"upstream error", coderws.StatusInternalError, nil, "响应中断，请重试。", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &publicGatewaySettingsRepo{settings: string(raw)}
			svc := service.NewSettingService(repo, &config.Config{})
			ctx := service.WithPublicGatewayErrorSettings(context.Background(), svc)
			var closeErr *service.OpenAIWSClientCloseError
			require.ErrorAs(t, service.NewOpenAIWSClientCloseError(tc.status, "private.invalid", tc.cause), &closeErr)
			require.Equal(t, tc.want, publicOpenAIWSClientCloseReason(ctx, closeErr))
			require.Equal(t, tc.reads, repo.reads)
		})
	}
}

func TestPublicGatewayErrorMiddleware(t *testing.T) {
	t.Run("HTTP error", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1beta/models/x", func(c *gin.Context) {
			c.Header("Location", "https://upstream.invalid")
			c.JSON(400, gin.H{"error": gin.H{"code": 400, "status": "INVALID_ARGUMENT", "message": "antigravity auth missing project_id", "details": "hidden"}})
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1beta/models/x", nil))
		require.Equal(t, 502, out.Code)
		require.Empty(t, out.Header().Get("Location"))
		require.NotContains(t, out.Body.String(), "antigravity")
		require.Contains(t, out.Body.String(), "INTERNAL")
	})
	t.Run("unknown 400", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1/chat/completions", func(c *gin.Context) {
			c.JSON(400, gin.H{"error": gin.H{"message": "unknown failure at private.example"}})
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil))
		require.Equal(t, 400, out.Code)
		require.JSONEq(t, `{"error":{"type":"invalid_request_error","code":"invalid_request","message":"参数异常"}}`, out.Body.String())
	})
	t.Run("split stream error", func(t *testing.T) {
		engine := gin.New()
		engine.Use(PublicGatewayErrorMiddleware(false))
		engine.GET("/v1/responses", func(c *gin.Context) {
			c.Header("Content-Type", "text/event-stream")
			_, _ = c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
			c.Writer.Flush()
			_, _ = c.Writer.WriteString("event: error\r\ndata: {\"error\":{\"message\":\"antigravity ")
			c.Writer.Flush()
			_, _ = c.Writer.WriteString("auth missing project_id\"}}\r\n\r\n")
		})
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
		require.Equal(t, 200, out.Code)
		require.Contains(t, out.Body.String(), "hello")
		require.Contains(t, out.Body.String(), "service_unavailable")
		require.NotContains(t, out.Body.String(), "antigravity")
	})
}

func TestPublicGatewayPreservesLocalImageTaskMetadata(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicGatewayErrorMiddleware(false))
	engine.GET("/v1/images/tasks/:task_id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": "task-1", "task_id": "task-1", "object": "image.generation.task", "status": "failed", "http_status": 400, "created_at": 123, "error": gin.H{"message": "antigravity https://private.invalid"}})
	})
	engine.GET("/v1/images/batches/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": "batch-1", "object": "image.batch", "status": "failed", "item_count": 3})
	})
	task := httptest.NewRecorder()
	engine.ServeHTTP(task, httptest.NewRequest(http.MethodGet, "/v1/images/tasks/task-1", nil))
	require.JSONEq(t, `{"id":"task-1","task_id":"task-1","object":"image.generation.task","status":"failed","http_status":400,"created_at":123,"error":{"code":"invalid_request","message":"参数异常"}}`, task.Body.String())
	batch := httptest.NewRecorder()
	engine.ServeHTTP(batch, httptest.NewRequest(http.MethodGet, "/v1/images/batches/batch-1", nil))
	require.JSONEq(t, `{"id":"batch-1","object":"image.batch","status":"failed","item_count":3}`, batch.Body.String())
}

func TestPublicGatewayEmbeddedHTTPErrorUsesSavedSettings(t *testing.T) {
	settings := service.DefaultPublicGatewayErrorSettings()
	settings.Rules = []service.PublicGatewayErrorRule{{ID: "tool", Name: "工具参数", Enabled: true, UpstreamStatuses: []int{400}, Pattern: "invalid tool", Status: 422, Code: "invalid_tool_request", Message: "工具参数有误"}}
	settings.BadRequestFallback = service.PublicGatewayError{Status: 409, Code: "custom_bad_request", Message: "请检查请求"}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	for _, tc := range []struct {
		diagnostic, code, message string
		status                    int
	}{
		{"unknown failure at private.invalid", "custom_bad_request", "请检查请求", 409},
		{"invalid tool at private.invalid", "invalid_tool_request", "工具参数有误", 422},
	} {
		t.Run(tc.code, func(t *testing.T) {
			repo := &publicGatewaySettingsRepo{settings: string(raw)}
			svc := service.NewSettingService(repo, &config.Config{})
			engine := gin.New()
			engine.Use(PublicGatewayErrorMiddleware(false, svc))
			engine.GET("/v1/chat/completions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"error": gin.H{"code": "400", "message": tc.diagnostic}})
			})
			out := httptest.NewRecorder()
			engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil))
			require.Equal(t, tc.status, out.Code)
			require.Contains(t, out.Body.String(), tc.code)
			require.Contains(t, out.Body.String(), tc.message)
			require.NotContains(t, out.Body.String(), "private.invalid")
			require.Equal(t, 1, repo.reads)
		})
	}
}
