//go:build unit

package admin

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupTagBinding(t *testing.T) {
	for _, tag := range []string{"", "chat", "image", "airp", "AIRP", "chat,image", "自定义标签"} {
		t.Run(tag, func(t *testing.T) {
			for _, req := range []any{&CreateGroupRequest{}, &UpdateGroupRequest{}} {
				require.NoError(t, bindGroupPlatformJSON(t, req, fmt.Sprintf(`{"name":"group","tag":%q}`, tag)))
				data, err := json.Marshal(req)
				require.NoError(t, err)
				var result map[string]any
				require.NoError(t, json.Unmarshal(data, &result))
				require.Equal(t, tag, result["tag"])
			}
		})
	}
}

func TestGroupTagColorBindingAndSimpleMode(t *testing.T) {
	for _, color := range []string{"", "#ab12EF"} {
		create := &CreateGroupRequest{}
		update := &UpdateGroupRequest{}
		payload := fmt.Sprintf(`{"name":"group","tag":"custom","tag_color":%q}`, color)
		require.NoError(t, bindGroupPlatformJSON(t, create, payload))
		require.NoError(t, bindGroupPlatformJSON(t, update, payload))
		sanitizeCreateGroupRequestForSimpleMode(create)
		sanitizeUpdateGroupRequestForSimpleMode(update)
		require.Equal(t, color, create.TagColor)
		require.NotNil(t, update.TagColor)
		require.Equal(t, color, *update.TagColor)
	}
	omitted := &UpdateGroupRequest{}
	require.NoError(t, bindGroupPlatformJSON(t, omitted, `{}`))
	require.Nil(t, omitted.TagColor)
}

func TestGroupTagColorAdminResponses(t *testing.T) {
	group := service.Group{ID: 1, Tag: "custom", TagColor: "#ab12EF"}
	for _, response := range []any{groupForSimpleMode(&group), systemCustomGroupContainerToResponse(group)} {
		raw, err := json.Marshal(response)
		require.NoError(t, err)
		var data map[string]any
		require.NoError(t, json.Unmarshal(raw, &data))
		require.Equal(t, group.Tag, data["tag"])
		require.Equal(t, group.TagColor, data["tag_color"])
	}
}
