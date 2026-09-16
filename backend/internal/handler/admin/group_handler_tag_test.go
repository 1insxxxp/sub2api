//go:build unit

package admin

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTagBinding(t *testing.T) {
	for _, tag := range []string{"", "chat", "image", "airp"} {
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
	for _, tag := range []string{"unknown", "AIRP", "chat,image"} {
		for _, req := range []any{&CreateGroupRequest{}, &UpdateGroupRequest{}} {
			require.Error(t, bindGroupPlatformJSON(t, req, fmt.Sprintf(`{"name":"group","tag":%q}`, tag)))
		}
	}
}
