//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleStreamingResponse_UsageSurvivesFirstOutputStaging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	staged := "data: " + `{"type":"message_start","message":{"id":"msg_staged","type":"message","role":"assistant","model":"model","content":[],"usage":{"input_tokens":10,"output_tokens":0,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":6,"ephemeral_1h_input_tokens":14}}}}` + "\n\n" +
		"data: " + `{"type":"message_delta","usage":{"output_tokens":2}}` + "\n\n"
	completed := "data: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n\n" +
		"data: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}` + "\n\n" +
		"event: message_stop\ndata: " + `{"type":"message_stop"}` + "\n\n"

	for _, tt := range []struct {
		name         string
		suffix       string
		readErr      error
		outputTokens int
	}{
		{name: "completed", suffix: completed, readErr: io.EOF, outputTokens: 7},
		{name: "canceled_before_first_output", readErr: context.Canceled, outputTokens: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := newMinimalGatewayService()
			svc.settingService = &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":120}}`}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			ctx, guard, _ := newModelFirstOutputGuard(c.Request.Context(), c, svc.settingService, PlatformAnthropic, "model")
			require.NotNil(t, guard)
			defer guard.Close()
			c.Request = c.Request.WithContext(ctx)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(&errAfterReader{data: strings.NewReader(staged + tt.suffix), err: tt.readErr}),
			}
			defer resp.Body.Close()

			result, err := svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1, Platform: PlatformAnthropic}, time.Now(), "model", "model", false)
			if tt.readErr == io.EOF {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.readErr)
			}
			require.NotNil(t, result)
			require.NotNil(t, result.usage)
			require.Equal(t, 10, result.usage.InputTokens)
			require.Equal(t, tt.outputTokens, result.usage.OutputTokens)
			require.Equal(t, 20, result.usage.CacheCreationInputTokens)
			require.Equal(t, 30, result.usage.CacheReadInputTokens)
			require.Equal(t, 6, result.usage.CacheCreation5mTokens)
			require.Equal(t, 14, result.usage.CacheCreation1hTokens)
			if tt.readErr == io.EOF {
				require.NotNil(t, result.firstTokenMs)
				require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: message_start"))
				require.Contains(t, recorder.Body.String(), "hello")
				require.Contains(t, recorder.Body.String(), "event: message_stop")
			} else {
				require.Nil(t, result.firstTokenMs)
				require.Empty(t, recorder.Body.String(), "usage must be collected without committing staged events")
			}
		})
	}
}
