//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type customGroupPrivacyRepo struct {
	service.UserCustomGroupRepository
	group   *service.UserCustomGroup
	sources map[int64]*service.Group
}

func (r *customGroupPrivacyRepo) ListByUserID(_ context.Context, userID int64) ([]service.UserCustomGroup, error) {
	if userID != r.group.UserID {
		return nil, service.ErrUserCustomGroupNotFound
	}
	return []service.UserCustomGroup{*r.group}, nil
}

func (r *customGroupPrivacyRepo) GetOwned(_ context.Context, id, userID int64) (*service.UserCustomGroup, error) {
	if id != r.group.ID || userID != r.group.UserID {
		return nil, service.ErrUserCustomGroupNotFound
	}
	return r.group, nil
}

func (r *customGroupPrivacyRepo) CountByUserID(context.Context, int64) (int, error) { return 0, nil }

func (r *customGroupPrivacyRepo) Create(_ context.Context, group *service.UserCustomGroup, models []service.UserCustomGroupModelInput) error {
	group.ID = r.group.ID
	group.CreatedAt, group.UpdatedAt = r.group.CreatedAt, r.group.UpdatedAt
	r.group = group
	r.replaceModels(models)
	return nil
}

func (r *customGroupPrivacyRepo) Update(_ context.Context, group *service.UserCustomGroup, models *[]service.UserCustomGroupModelInput) error {
	r.group = group
	if models != nil {
		r.replaceModels(*models)
	}
	return nil
}

func (r *customGroupPrivacyRepo) replaceModels(models []service.UserCustomGroupModelInput) {
	r.group.Models = make([]service.UserCustomGroupModel, 0, len(models))
	for i, model := range models {
		r.group.Models = append(r.group.Models, service.UserCustomGroupModel{
			ID: int64(i + 1), CustomGroupID: r.group.ID, PublicModel: model.PublicModel,
			SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel,
			SourceGroup: r.sources[model.SourceGroupID], CreatedAt: r.group.CreatedAt, UpdatedAt: r.group.UpdatedAt,
		})
	}
}

type customGroupPrivacySources struct {
	service.GroupRepository
	sources map[int64]*service.Group
}

func (r customGroupPrivacySources) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if group := r.sources[id]; group != nil {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

func newCustomGroupPrivacyHandler() *UserCustomGroupHandler {
	sources := map[int64]*service.Group{
		11: {
			ID: 11, Name: "Public source", Platform: service.PlatformAnthropic, Status: service.StatusActive,
			ModelRouting: map[string][]int64{"model": {90001}}, ModelRoutingEnabled: true,
			CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{90002}},
			ProfitControlEnabled:      true, ProfitMinMargin: .37, ProfitSafetyBuffer: .11,
			DuplicateOperationID: "synthetic-private-operation",
		},
		12: {
			ID: 12, Name: "Revoked source", Platform: service.PlatformAnthropic, Status: service.StatusActive, IsExclusive: true,
			ModelRouting: map[string][]int64{"model": {90003}}, DuplicateOperationID: "synthetic-private-revoked",
		},
	}
	createdAt := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	repo := &customGroupPrivacyRepo{
		group:   &service.UserCustomGroup{ID: 21, UserID: 9, Name: "My group", Status: service.StatusActive, CreatedAt: createdAt, UpdatedAt: createdAt},
		sources: sources,
	}
	repo.replaceModels([]service.UserCustomGroupModelInput{
		{PublicModel: "alias", SourceGroupID: 11, SourceModel: "model"},
		{PublicModel: "revoked-alias", SourceGroupID: 12, SourceModel: "model"},
	})
	svc := service.NewUserCustomGroupService(repo, customGroupCandidateUserRepoStub{}, customGroupPrivacySources{sources: sources}, nil, nil)
	return NewUserCustomGroupHandler(svc, nil, nil)
}

func TestUserCustomGroupHandlerResponsesOnlyExposePublicSourceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, method, body string
		handle             func(*UserCustomGroupHandler, *gin.Context)
		list, hasRevoked   bool
	}{
		{"list", http.MethodGet, "", (*UserCustomGroupHandler).List, true, true},
		{"get", http.MethodGet, "", (*UserCustomGroupHandler).Get, false, true},
		{"create", http.MethodPost, `{"name":"My group","models":[{"public_model":"alias","source_group_id":11,"source_model":"model"}]}`, (*UserCustomGroupHandler).Create, false, false},
		{"update", http.MethodPut, `{"name":"My group"}`, (*UserCustomGroupHandler).Update, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newCustomGroupPrivacyHandler()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(test.method, "/api/v1/custom-groups/21", strings.NewReader(test.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: "21"}}
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
			test.handle(h, c)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			var group map[string]any
			if test.list {
				var groups []map[string]any
				require.NoError(t, json.Unmarshal(body.Data, &groups))
				require.Len(t, groups, 1)
				group = groups[0]
			} else {
				require.NoError(t, json.Unmarshal(body.Data, &group))
			}
			require.Equal(t, float64(21), group["id"])
			require.Equal(t, float64(9), group["user_id"])
			require.Equal(t, "My group", group["name"])
			require.NotEmpty(t, group["created_at"])
			models := group["models"].([]any)
			first := models[0].(map[string]any)
			require.Equal(t, "alias", first["public_model"])
			require.Equal(t, "model", first["source_model"])
			require.Equal(t, true, first["source_available"])
			require.Equal(t, map[string]any{"id": float64(11), "name": "Public source", "platform": service.PlatformAnthropic, "status": service.StatusActive}, first["source_group"])
			if test.hasRevoked {
				revoked := models[1].(map[string]any)
				require.Equal(t, false, revoked["source_available"])
				require.Equal(t, service.UserCustomGroupSourceIssueNotAllowed, revoked["source_issue"])
				require.Equal(t, map[string]any{"id": float64(12), "name": "Revoked source", "platform": service.PlatformAnthropic, "status": service.StatusActive}, revoked["source_group"])
			}
			for _, forbidden := range []string{"ModelRouting", "CodexModelsManifestConfig", "ProfitMinMargin", "ProfitSafetyBuffer", "DuplicateOperationID", "synthetic-private", "90001", "90002", "90003"} {
				require.NotContains(t, w.Body.String(), forbidden)
			}
		})
	}
}
