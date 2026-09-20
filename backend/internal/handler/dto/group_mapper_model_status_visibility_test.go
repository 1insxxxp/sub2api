package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMapperRoundTripsModelStatusVisibility(t *testing.T) {
	group := &service.Group{ID: 31, Name: "visibility", Platform: service.PlatformOpenAI,
		ModelStatusVisibility: service.GroupModelStatusVisibility{Enabled: true, Models: []string{"gpt-5.4"}},
		ModelAllowlist:        service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}},
	}
	adminJSON, err := json.Marshal(GroupFromServiceAdmin(group))
	require.NoError(t, err)
	var decoded struct {
		Visibility service.GroupModelStatusVisibility `json:"model_status_visibility"`
		Allowlist  service.GroupModelAllowlist        `json:"model_allowlist"`
	}
	require.NoError(t, json.Unmarshal(adminJSON, &decoded))
	require.Equal(t, group.ModelStatusVisibility, decoded.Visibility)
	require.Equal(t, group.ModelAllowlist, decoded.Allowlist)
	userJSON, err := json.Marshal(GroupFromService(group))
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "model_status_visibility")
}
