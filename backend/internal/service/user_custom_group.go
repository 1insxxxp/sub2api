package service

import "time"

const MaxUserCustomGroups = 20
const MaxUserCustomGroupModels = 500

const (
	UserCustomGroupSourceIssueUnavailable      = "source_group_unavailable"
	UserCustomGroupSourceIssueNotAllowed       = "source_group_not_allowed"
	UserCustomGroupSourceIssueModelUnavailable = "source_model_unavailable"
)

type UserCustomGroupModel struct {
	ID              int64     `json:"id"`
	CustomGroupID   int64     `json:"custom_group_id"`
	PublicModel     string    `json:"public_model"`
	SourceGroupID   int64     `json:"source_group_id"`
	SourceModel     string    `json:"source_model"`
	SourceGroup     *Group    `json:"source_group,omitempty"`
	SourceAvailable bool      `json:"source_available"`
	SourceIssue     string    `json:"source_issue,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

type UserCustomGroupModelInput struct {
	PublicModel   string `json:"public_model"`
	SourceGroupID int64  `json:"source_group_id"`
	SourceModel   string `json:"source_model"`
}

type CreateUserCustomGroupRequest struct {
	Name   string                      `json:"name"`
	Models []UserCustomGroupModelInput `json:"models"`
}

type UpdateUserCustomGroupRequest struct {
	Name   *string                      `json:"name,omitempty"`
	Status *string                      `json:"status,omitempty"`
	Models *[]UserCustomGroupModelInput `json:"models,omitempty"`
}
