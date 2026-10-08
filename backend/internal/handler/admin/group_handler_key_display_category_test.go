//go:build unit

package admin

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryBindingAndSimpleMode(t *testing.T) {
	for _, category := range []string{"", "anthropic", "openai", "domestic", "other"} {
		t.Run(category, func(t *testing.T) {
			for _, req := range []any{&CreateGroupRequest{}, &UpdateGroupRequest{}} {
				require.NoError(t, bindGroupPlatformJSON(t, req, fmt.Sprintf(`{"name":"group","key_display_category":%q}`, category)))
				switch req := req.(type) {
				case *CreateGroupRequest:
					sanitizeCreateGroupRequestForSimpleMode(req)
				case *UpdateGroupRequest:
					sanitizeUpdateGroupRequestForSimpleMode(req)
				}
				raw, err := json.Marshal(req)
				require.NoError(t, err)
				var result map[string]any
				require.NoError(t, json.Unmarshal(raw, &result))
				require.Equal(t, category, result["key_display_category"])
			}
		})
	}
}

func TestGroupKeyDisplayCategoryUpdateOmissionAndClearing(t *testing.T) {
	var omitted, cleared UpdateGroupRequest
	require.NoError(t, bindGroupPlatformJSON(t, &omitted, `{}`))
	require.Nil(t, omitted.KeyDisplayCategory)
	require.NoError(t, bindGroupPlatformJSON(t, &cleared, `{"key_display_category":""}`))
	require.NotNil(t, cleared.KeyDisplayCategory)
	require.Empty(t, *cleared.KeyDisplayCategory)
}

func TestGroupKeyDisplayCategoryAdminResponses(t *testing.T) {
	group := service.Group{ID: 1, Platform: service.PlatformOpenAI, KeyDisplayCategory: "domestic"}
	for _, response := range []any{groupForSimpleMode(&group), systemCustomGroupContainerToResponse(group)} {
		raw, err := json.Marshal(response)
		require.NoError(t, err)
		var data map[string]any
		require.NoError(t, json.Unmarshal(raw, &data))
		require.Equal(t, "domestic", data["key_display_category"])
		require.Equal(t, service.PlatformOpenAI, data["platform"])
	}
}

func TestGroupKeyDisplayCategoryBindingRejectsUnknown(t *testing.T) {
	for _, category := range []string{"unknown", "DOMESTIC", " domestic ", "gemini"} {
		for _, req := range []any{&CreateGroupRequest{}, &UpdateGroupRequest{}} {
			require.Error(t, bindGroupPlatformJSON(t, req, fmt.Sprintf(`{"name":"group","key_display_category":%q}`, category)))
		}
	}
}
