package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UserCustomGroupSource exposes only the source identity used by the user's
// custom-group editor. Internal routing, account and profit settings stay private.
type UserCustomGroupSource struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

type UserCustomGroupModel struct {
	ID              int64                  `json:"id"`
	CustomGroupID   int64                  `json:"custom_group_id"`
	PublicModel     string                 `json:"public_model"`
	SourceGroupID   int64                  `json:"source_group_id"`
	SourceModel     string                 `json:"source_model"`
	SourceGroup     *UserCustomGroupSource `json:"source_group,omitempty"`
	SourceAvailable bool                   `json:"source_available"`
	SourceIssue     string                 `json:"source_issue,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type UserCustomGroup struct {
	ID        int64                  `json:"id"`
	UserID    int64                  `json:"user_id"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Models    []UserCustomGroupModel `json:"models"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

func UserCustomGroupFromService(group *service.UserCustomGroup) *UserCustomGroup {
	if group == nil {
		return nil
	}
	out := &UserCustomGroup{
		ID: group.ID, UserID: group.UserID, Name: group.Name, Status: group.Status,
		Models:    make([]UserCustomGroupModel, 0, len(group.Models)),
		CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
	}
	for _, model := range group.Models {
		out.Models = append(out.Models, userCustomGroupModelFromService(model))
	}
	return out
}

func userCustomGroupModelFromService(model service.UserCustomGroupModel) UserCustomGroupModel {
	item := UserCustomGroupModel{
		ID: model.ID, CustomGroupID: model.CustomGroupID, PublicModel: model.PublicModel,
		SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel,
		SourceAvailable: model.SourceAvailable, SourceIssue: model.SourceIssue,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
	if source := model.SourceGroup; source != nil {
		item.SourceGroup = &UserCustomGroupSource{
			ID: source.ID, Name: source.Name, Platform: source.Platform, Status: source.Status,
		}
	}
	return item
}
