package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAICompatibleHandlersRejectInvalidStreamFieldType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		body string
		run  func(*gin.Context)
	}{
		{
			name: "gateway_responses_string_stream",
			path: "/v1/responses",
			body: `{"model":"gpt-5","stream":"true","input":"hello"}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{}).Responses(c)
			},
		},
		{
			name: "gateway_responses_number_stream",
			path: "/v1/responses",
			body: `{"model":"gpt-5","stream":1,"input":"hello"}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{}).Responses(c)
			},
		},
		{
			name: "gateway_chat_completions_string_stream",
			path: "/v1/chat/completions",
			body: `{"model":"gpt-5","stream":"true","messages":[{"role":"user","content":"hello"}]}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{}).ChatCompletions(c)
			},
		},
		{
			name: "gateway_chat_completions_number_stream",
			path: "/v1/chat/completions",
			body: `{"model":"gpt-5","stream":1,"messages":[{"role":"user","content":"hello"}]}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{}).ChatCompletions(c)
			},
		},
		{
			name: "openai_chat_completions_string_stream",
			path: "/openai/v1/chat/completions",
			body: `{"model":"gpt-5","stream":"true","messages":[{"role":"user","content":"hello"}]}`,
			run: func(c *gin.Context) {
				newOpenAIHandlerForPreviousResponseIDValidation(t, nil).ChatCompletions(c)
			},
		},
		{
			name: "openai_chat_completions_number_stream",
			path: "/openai/v1/chat/completions",
			body: `{"model":"gpt-5","stream":1,"messages":[{"role":"user","content":"hello"}]}`,
			run: func(c *gin.Context) {
				newOpenAIHandlerForPreviousResponseIDValidation(t, nil).ChatCompletions(c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newOpenAICompatibleStreamValidationContext(tt.path, tt.body, false)

			tt.run(c)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, invalidStreamFieldTypeMessage, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
			require.Contains(t, rec.Body.String(), "invalid_request_error")
		})
	}
}

func TestGatewayOpenAICompatibleHandlersAllowBooleanStreamToContinue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		body string
		run  func(*gin.Context)
	}{
		{
			name: "responses_false",
			path: "/v1/responses",
			body: `{"model":"gpt-5","stream":false,"input":"hello"}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{gatewayService: &service.GatewayService{}}).Responses(c)
			},
		},
		{
			name: "chat_completions_true",
			path: "/v1/chat/completions",
			body: `{"model":"gpt-5","stream":true,"messages":[{"role":"user","content":"hello"}]}`,
			run: func(c *gin.Context) {
				(&GatewayHandler{gatewayService: &service.GatewayService{}}).ChatCompletions(c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newOpenAICompatibleStreamValidationContext(tt.path, tt.body, true)

			tt.run(c)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Contains(t, rec.Body.String(), "This group is restricted to Claude Code clients")
		})
	}
}

func TestOpenAICompatibleRelayStreamModeCoercesStreamBeforeHandlerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		path        string
		body        string
		mode        openAICompatibleRelayStreamMode
		run         func(*gin.Context)
		wantMessage string
	}{
		{
			name: "chat_completions_force_stream_replaces_invalid_string_stream",
			path: "/relay-stream/v1/chat/completions",
			body: `{"model":"gpt-image-1","stream":"false","messages":[{"role":"user","content":"draw"}]}`,
			mode: openAICompatibleRelayStreamModeForceStream,
			run: func(c *gin.Context) {
				newOpenAIHandlerForPreviousResponseIDValidation(t, nil).ChatCompletions(c)
			},
			wantMessage: "This model is not supported on the Chat Completions endpoint",
		},
		{
			name: "responses_force_non_stream_replaces_invalid_string_stream",
			path: "/relay-nonstream/v1/responses",
			body: `{"model":"gpt-5","stream":"true","previous_response_id":"msg_123","input":"hello"}`,
			mode: openAICompatibleRelayStreamModeForceNonStream,
			run: func(c *gin.Context) {
				newOpenAIHandlerForPreviousResponseIDValidation(t, nil).Responses(c)
			},
			wantMessage: "previous_response_id must be a response.id (resp_*), not a message id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newOpenAICompatibleStreamValidationContext(tt.path, tt.body, false)
			markOpenAICompatibleRelayStreamMode(c, tt.mode)

			tt.run(c)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, tt.wantMessage, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
			require.NotContains(t, rec.Body.String(), invalidStreamFieldTypeMessage)
		})
	}
}

func TestGatewayRelayStreamModeSetsRequestTypeForNonOpenAICompatiblePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		body       string
		mode       openAICompatibleRelayStreamMode
		run        func(*gin.Context)
		wantStream bool
	}{
		{
			name:       "chat_completions_force_stream",
			path:       "/relay-stream/v1/chat/completions",
			body:       `{"model":"gemini-2.5-flash","stream":false,"messages":[{"role":"user","content":"hello"}]}`,
			mode:       openAICompatibleRelayStreamModeForceStream,
			run:        (&GatewayHandler{gatewayService: &service.GatewayService{}}).ChatCompletions,
			wantStream: true,
		},
		{
			name:       "responses_force_non_stream",
			path:       "/relay-nonstream/v1/responses",
			body:       `{"model":"gemini-2.5-flash","stream":true,"input":"hello"}`,
			mode:       openAICompatibleRelayStreamModeForceNonStream,
			run:        (&GatewayHandler{gatewayService: &service.GatewayService{}}).Responses,
			wantStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newOpenAICompatibleStreamValidationContext(tt.path, tt.body, true)
			markOpenAICompatibleRelayStreamMode(c, tt.mode)

			tt.run(c)

			require.Equal(t, http.StatusForbidden, rec.Code)
			gotStream, ok := c.Get(opsStreamKey)
			require.True(t, ok)
			require.Equal(t, tt.wantStream, gotStream)
			gotRequestType, ok := c.Get(opsRequestTypeKey)
			require.True(t, ok)
			require.Equal(t, int16(service.RequestTypeFromLegacy(tt.wantStream, false)), gotRequestType)
		})
	}
}

func newOpenAICompatibleStreamValidationContext(path, body string, claudeCodeOnly bool) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(7)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, ClaudeCodeOnly: claudeCodeOnly},
		User:    &service.User{ID: 13},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 13, Concurrency: 1})

	return c, rec
}
