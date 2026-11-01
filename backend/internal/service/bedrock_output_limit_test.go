//go:build unit

package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBedrockCCOutputLimitUsesModelCapability(t *testing.T) {
	for _, tc := range []struct {
		name, model, field string
		want               int64
		present            bool
	}{
		{name: "known model", model: "claude-opus-4-6", want: 128000, present: true},
		{name: "regional Bedrock model", model: "eu.anthropic.claude-opus-4-6-v1", want: 128000, present: true},
		{name: "older Bedrock model", model: "us.anthropic.claude-sonnet-4-5-20250929-v1:0", want: 64000, present: true},
		{name: "unknown model", model: "private-compatible-model"},
		{name: "unknown Bedrock model", model: "us.anthropic.private-compatible-model-v1:0"},
		{name: "explicit large limit", model: "claude-opus-4-6", field: `,"max_tokens":256000`, want: 256000, present: true},
		{name: "explicit zero", model: "claude-opus-4-6", field: `,"max_tokens":0`, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"messages":[]%s}`, tc.model, tc.field))
			out := sanitizeBedrockCCFields(body)
			limit := gjson.GetBytes(out, "max_tokens")
			require.Equal(t, tc.present, limit.Exists())
			require.Equal(t, tc.want, limit.Int())
		})
	}
}

func TestApplyBedrockCCCompatOutputLimitUsesFinalAccountModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(777)
	channels := &ChannelService{}
	channels.cache.Store(&channelCache{
		loadedAt: time.Now(),
		channelByGroupID: map[int64]*Channel{
			groupID: {Status: StatusActive, FeaturesConfig: map[string]any{featureKeyBedrockCCCompat: true}},
		},
	})
	svc := &GatewayService{channelService: channels}
	for _, tc := range []struct {
		name, target, field string
		metadata            int64
		want                int64
		present             bool
	}{
		{name: "resolved official model", target: "claude-opus-4-6", want: 128000, present: true},
		{name: "resolved account capability", target: "claude-opus-4-6", metadata: 96000, want: 96000, present: true},
		{name: "unknown model no guessed limit", target: "us.anthropic.private-compatible-model-v1:0"},
		{name: "unknown account capability", target: "us.anthropic.private-compatible-model-v1:0", metadata: 112000, want: 112000, present: true},
		{name: "explicit zero preserved", target: "claude-opus-4-6", field: `,"max_tokens":0`, present: true},
		{name: "explicit large preserved", target: "claude-opus-4-6", field: `,"max_tokens":256000`, want: 256000, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock,
				Credentials: map[string]any{"aws_region": "eu-west-1", "model_mapping": map[string]any{"public-alias": tc.target}}}
			resolved, ok := ResolveBedrockModelID(account, "public-alias")
			require.True(t, ok)
			account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
				resolved:       {ID: resolved, MaxOutputTokens: tc.metadata},
				"public-alias": {ID: "public-alias", MaxOutputTokens: 512},
			}})
			body := []byte(`{"model":"public-alias","messages":[]` + tc.field + `}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			out := svc.ApplyBedrockCCCompat(c, body, "public-alias", account, &groupID)
			limit := gjson.GetBytes(out, "max_tokens")
			require.Equal(t, tc.present, limit.Exists())
			require.Equal(t, tc.want, limit.Int())
		})
	}
}

func TestBedrockDefaultThinkingFitsOutputAllowance(t *testing.T) {
	for _, limit := range []int{0, 512, 1024, 1025, 4096, 128000} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":"claude-sonnet-4-5","max_tokens":%d,"thinking":{"type":"enabled"}}`, limit))
			got := sanitizeBedrockThinking(body, "claude-sonnet-4-5")
			require.Equal(t, int64(limit), gjson.GetBytes(got, "max_tokens").Int())
			budget := gjson.GetBytes(got, "thinking.budget_tokens")
			if limit <= 1024 {
				require.False(t, budget.Exists(), "cannot supply a compatible manual thinking budget within this cap")
			} else {
				require.True(t, budget.Exists())
				require.GreaterOrEqual(t, budget.Int(), int64(1024))
				require.Less(t, budget.Int(), int64(limit))
			}
		})
	}
}
