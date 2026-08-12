package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const MaxSystemCustomGroupModels = 200

var (
	ErrSystemCustomGroupNotFound             = infraerrors.NotFound("SYSTEM_CUSTOM_GROUP_NOT_FOUND", "system custom group not found")
	ErrSystemCustomGroupRouteNotFound        = infraerrors.NotFound("SYSTEM_CUSTOM_GROUP_ROUTE_NOT_FOUND", "system custom group model route not found")
	ErrSystemCustomGroupDuplicatePublicModel = infraerrors.Conflict("SYSTEM_CUSTOM_GROUP_DUPLICATE_PUBLIC_MODEL", "system custom group public model already exists")
	ErrSystemCustomGroupDuplicateSourceModel = infraerrors.Conflict("SYSTEM_CUSTOM_GROUP_DUPLICATE_SOURCE_MODEL", "system custom group source model already exists")
	ErrSystemCustomGroupInvalidSourceGroup   = infraerrors.BadRequest("SYSTEM_CUSTOM_GROUP_INVALID_SOURCE_GROUP", "system custom group source group is invalid")
	ErrSystemCustomGroupMissingSourceModel   = infraerrors.BadRequest("SYSTEM_CUSTOM_GROUP_MISSING_SOURCE_MODEL", "system custom group source model is unavailable")
	ErrSystemCustomGroupSelfReference        = infraerrors.BadRequest("SYSTEM_CUSTOM_GROUP_SELF_REFERENCE", "system custom group cannot route to itself")
	ErrSystemCustomGroupInvalidRoute         = infraerrors.BadRequest("SYSTEM_CUSTOM_GROUP_INVALID_ROUTE", "system custom group model route is invalid")
	ErrSystemCustomGroupInvalidInput         = infraerrors.BadRequest("SYSTEM_CUSTOM_GROUP_INVALID_INPUT", "system custom group input is invalid")
	ErrSystemCustomGroupInUse                = infraerrors.Conflict("SYSTEM_CUSTOM_GROUP_IN_USE", "system custom group is in use")
	ErrSystemCustomGroupManagedOnly          = infraerrors.Conflict("SYSTEM_CUSTOM_GROUP_MANAGED_ONLY", "system custom groups must be managed through the dedicated API")
	ErrSystemCustomGroupRetryableConflict    = infraerrors.ServiceUnavailable("SYSTEM_CUSTOM_GROUP_RETRYABLE_CONFLICT", "system custom group changed concurrently; retry the request")
	ErrSystemCustomGroupModelNotAllowed      = infraerrors.Forbidden("SYSTEM_CUSTOM_GROUP_MODEL_NOT_ALLOWED", "the requested model is not enabled for this subscription group")
	ErrSystemCustomGroupSourceUnavailable    = infraerrors.ServiceUnavailable("SYSTEM_CUSTOM_GROUP_SOURCE_UNAVAILABLE", "the selected model source is temporarily unavailable")
)

// SystemCustomGroupRouteError preserves machine-readable error identity while
// carrying the exact route that violated an invariant. Handlers can use both
// errors.Is and errors.As without parsing human-readable text.
type SystemCustomGroupRouteError struct {
	Kind          error
	PublicModel   string
	SourceGroupID int64
	SourceModel   string
}

func (e *SystemCustomGroupRouteError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v: public_model=%q source_group_id=%d source_model=%q", e.Kind, e.PublicModel, e.SourceGroupID, e.SourceModel)
}

func (e *SystemCustomGroupRouteError) Unwrap() error {
	if e == nil {
		return nil
	}
	var appErr *infraerrors.ApplicationError
	if !errors.As(e.Kind, &appErr) {
		return e.Kind
	}
	metadata := make(map[string]string, len(appErr.Metadata)+3)
	for key, value := range appErr.Metadata {
		metadata[key] = value
	}
	metadata["public_model"] = e.PublicModel
	metadata["source_group_id"] = strconv.FormatInt(e.SourceGroupID, 10)
	metadata["source_model"] = e.SourceModel
	return appErr.WithMetadata(metadata)
}

// SystemCustomGroupDeleteImpact contains cache identities collected in the
// same transaction that retired a system custom group.
type SystemCustomGroupDeleteImpact struct {
	APIKeyValues []string
	UserIDs      []int64
}

type SystemCustomGroupModelInput struct {
	PublicModel   string `json:"public_model"`
	SourceGroupID int64  `json:"source_group_id"`
	SourceModel   string `json:"source_model"`
	Enabled       bool   `json:"enabled"`
}

type CreateSystemCustomGroupRequest struct {
	Name                string                        `json:"name"`
	Description         *string                       `json:"description"`
	DailyLimitUSD       *float64                      `json:"daily_limit_usd"`
	WeeklyLimitUSD      *float64                      `json:"weekly_limit_usd"`
	MonthlyLimitUSD     *float64                      `json:"monthly_limit_usd"`
	DefaultValidityDays int                           `json:"default_validity_days"`
	Models              []SystemCustomGroupModelInput `json:"models"`
}

type UpdateSystemCustomGroupRequest struct {
	Name                string                        `json:"name"`
	Description         *string                       `json:"description"`
	DailyLimitUSD       *float64                      `json:"daily_limit_usd"`
	WeeklyLimitUSD      *float64                      `json:"weekly_limit_usd"`
	MonthlyLimitUSD     *float64                      `json:"monthly_limit_usd"`
	DefaultValidityDays int                           `json:"default_validity_days"`
	Models              []SystemCustomGroupModelInput `json:"models"`
}

type SystemCustomGroupModel struct {
	ID            int64     `json:"id"`
	GroupID       int64     `json:"group_id"`
	PublicModel   string    `json:"public_model"`
	SourceGroupID int64     `json:"source_group_id"`
	SourceModel   string    `json:"source_model"`
	Enabled       bool      `json:"enabled"`
	SourceGroup   *Group    `json:"source_group,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SystemCustomGroup struct {
	Group  Group                    `json:"group"`
	Models []SystemCustomGroupModel `json:"models"`
}

type SystemCustomGroupCandidate struct {
	Group  Group    `json:"group"`
	Models []string `json:"models"`
}

type SystemCustomGroupSyncAdded struct {
	PublicModel   string `json:"public_model"`
	SourceGroupID int64  `json:"source_group_id"`
	SourceModel   string `json:"source_model"`
	Selected      bool   `json:"selected"`
}

type SystemCustomGroupSyncConflict struct {
	PublicModel   string `json:"public_model"`
	SourceGroupID int64  `json:"source_group_id"`
	SourceModel   string `json:"source_model"`
	Reason        string `json:"reason"`
	Err           error  `json:"-"`
}

type SystemCustomGroupSyncPreview struct {
	Added       []SystemCustomGroupSyncAdded    `json:"added"`
	Missing     []SystemCustomGroupModel        `json:"missing"`
	Conflicting []SystemCustomGroupSyncConflict `json:"conflicting"`
}
