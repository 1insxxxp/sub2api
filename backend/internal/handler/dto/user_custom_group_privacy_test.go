//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyDTOCustomGroupDoesNotExposeInternalSourceConfiguration(t *testing.T) {
	key := &service.APIKey{ID: 1, UserID: 9, CustomGroup: &service.UserCustomGroup{
		ID: 21, UserID: 9, Name: "My group", Status: service.StatusActive,
		Models: []service.UserCustomGroupModel{{
			ID: 1, CustomGroupID: 21, PublicModel: "alias", SourceGroupID: 11, SourceModel: "model",
			SourceAvailable: false, SourceIssue: service.UserCustomGroupSourceIssueNotAllowed,
			SourceGroup: &service.Group{
				ID: 11, Name: "Source", Platform: service.PlatformAnthropic, Status: service.StatusActive,
				ModelRouting:              map[string][]int64{"model": {90001}},
				CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{90002}},
				ProfitMinMargin:           .37, DuplicateOperationID: "synthetic-private-operation",
				AccountGroups: []service.AccountGroup{{AccountID: 90001, Account: &service.Account{Credentials: map[string]any{"api_key": "synthetic-private-credential"}}}},
			},
		}},
	}}
	payload, err := json.Marshal(APIKeyFromService(key))
	require.NoError(t, err)
	var body struct {
		CustomGroup struct {
			ID     int64 `json:"id"`
			Models []struct {
				SourceGroup     map[string]any `json:"source_group"`
				SourceAvailable bool           `json:"source_available"`
				SourceIssue     string         `json:"source_issue"`
			} `json:"models"`
		} `json:"custom_group"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	require.Equal(t, int64(21), body.CustomGroup.ID)
	require.Len(t, body.CustomGroup.Models, 1)
	model := body.CustomGroup.Models[0]
	require.False(t, model.SourceAvailable)
	require.Equal(t, service.UserCustomGroupSourceIssueNotAllowed, model.SourceIssue)
	require.Equal(t, map[string]any{"id": float64(11), "name": "Source", "platform": service.PlatformAnthropic, "status": service.StatusActive}, model.SourceGroup)
	for _, forbidden := range []string{"ModelRouting", "CodexModelsManifestConfig", "ProfitMinMargin", "DuplicateOperationID", "AccountGroups", "Credentials", "synthetic-private", "90001", "90002"} {
		require.NotContains(t, string(payload), forbidden)
	}
}

func TestUserCustomGroupDTOHandlesAbsentSources(t *testing.T) {
	require.Nil(t, UserCustomGroupFromService(nil))
	empty := UserCustomGroupFromService(&service.UserCustomGroup{ID: 21})
	require.NotNil(t, empty.Models)
	require.Empty(t, empty.Models)
	group := UserCustomGroupFromService(&service.UserCustomGroup{
		ID: 21,
		Models: []service.UserCustomGroupModel{{
			ID: 1, SourceGroupID: 11, SourceModel: "model", PublicModel: "alias",
			SourceIssue: service.UserCustomGroupSourceIssueUnavailable,
		}},
	})
	require.Len(t, group.Models, 1)
	require.Nil(t, group.Models[0].SourceGroup)
	require.False(t, group.Models[0].SourceAvailable)
	require.Equal(t, service.UserCustomGroupSourceIssueUnavailable, group.Models[0].SourceIssue)
	payload, err := json.Marshal(group)
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"source_group":`)
}
