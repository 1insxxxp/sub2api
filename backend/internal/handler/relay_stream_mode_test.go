package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestApplyOpenAICompatibleRelayStreamMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		mode      openAICompatibleRelayStreamMode
		body      []byte
		wantBody  string
		wantApply bool
		wantErr   bool
	}{
		{
			name:      "force stream adds missing stream true",
			mode:      openAICompatibleRelayStreamModeForceStream,
			body:      []byte(`{"model":"gpt-test","messages":[]}`),
			wantBody:  `{"model":"gpt-test","messages":[],"stream":true}`,
			wantApply: true,
		},
		{
			name:      "force stream converts false to true",
			mode:      openAICompatibleRelayStreamModeForceStream,
			body:      []byte(`{"model":"gpt-test","stream":false}`),
			wantBody:  `{"model":"gpt-test","stream":true}`,
			wantApply: true,
		},
		{
			name:      "force non stream converts true to false",
			mode:      openAICompatibleRelayStreamModeForceNonStream,
			body:      []byte(`{"model":"gpt-test","stream":true}`),
			wantBody:  `{"model":"gpt-test","stream":false}`,
			wantApply: true,
		},
		{
			name:      "normal leaves body unchanged",
			mode:      openAICompatibleRelayStreamModeNormal,
			body:      []byte(`{"model":"gpt-test","stream":false}`),
			wantBody:  `{"model":"gpt-test","stream":false}`,
			wantApply: false,
		},
		{
			name:    "invalid json returns error",
			mode:    openAICompatibleRelayStreamModeForceStream,
			body:    []byte(`{"model":`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/relay-stream/v1/chat/completions", nil)
			markOpenAICompatibleRelayStreamMode(c, tt.mode)

			got, applied, err := applyOpenAICompatibleRelayStreamMode(c, tt.body)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantApply, applied)
			require.JSONEq(t, tt.wantBody, string(got))
			require.Equal(t, gjson.Get(tt.wantBody, "stream").String(), gjson.GetBytes(got, "stream").String())
		})
	}
}
