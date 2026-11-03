package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// CategoryDefaultCategories is the fixed set of selectable default-entry categories.
var CategoryDefaultCategories = []string{"anthropic", "openai", "gemini", "grok", "domestic", "other"}

var (
	ErrCategoryDefaultNotFound            = infraerrors.NotFound("CATEGORY_DEFAULT_NOT_FOUND", "category default group not found")
	ErrCategoryDefaultInvalidCategory     = infraerrors.BadRequest("CATEGORY_DEFAULT_INVALID_CATEGORY", "this category does not support a default group")
	ErrCategoryDefaultInvalidSource       = infraerrors.BadRequest("CATEGORY_DEFAULT_INVALID_SOURCE", "select active standard source groups in the same category")
	ErrCategoryDefaultUnboundedSource     = infraerrors.BadRequest("CATEGORY_DEFAULT_UNBOUNDED_SOURCE", "source requires a finite model declaration or an exact model allowlist")
	ErrCategoryDefaultModelConflict       = infraerrors.Conflict("CATEGORY_DEFAULT_MODEL_CONFLICT", "associated source groups expose duplicate model names; contact an administrator")
	ErrCategoryDefaultStalePreview        = infraerrors.Conflict("CATEGORY_DEFAULT_STALE_PREVIEW", "configuration changed; refresh the synchronization preview")
	ErrCategoryDefaultPendingSync         = infraerrors.ServiceUnavailable("CATEGORY_DEFAULT_PENDING_SYNC", "this model changed and is awaiting administrator confirmation")
	ErrCategoryDefaultModelNotAllowed     = infraerrors.Forbidden("CATEGORY_DEFAULT_MODEL_NOT_ALLOWED", "this model is not published or accessible through the category default group")
	ErrCategoryDefaultSourceUnavailable   = infraerrors.ServiceUnavailable("CATEGORY_DEFAULT_SOURCE_UNAVAILABLE", "the selected model source is unavailable")
	ErrCategoryDefaultEndpointUnsupported = infraerrors.New(501, "CATEGORY_DEFAULT_ENDPOINT_UNSUPPORTED", "use a specific group for this endpoint")
)

type CategoryDefaultPublishedModel struct {
	PublicModel   string `json:"public_model"`
	SourceGroupID int64  `json:"source_group_id"`
	SourceModel   string `json:"source_model"`
	Fingerprint   string `json:"fingerprint"`
	// Platforms is derived from the current declaration, never from user input.
	Platforms    []string `json:"-"`
	GeminiNative bool     `json:"-"`
}

type CategoryDefaultGroupConfig struct {
	GroupID           int64                           `json:"group_id"`
	Category          string                          `json:"category"`
	Name              string                          `json:"name"`
	Enabled           bool                            `json:"enabled"`
	Revision          int64                           `json:"revision"`
	SourceGroupIDs    []int64                         `json:"source_group_ids"`
	Models            []CategoryDefaultPublishedModel `json:"models"`
	ModelsCount       int                             `json:"models_count"`
	InvalidatedModels []string                        `json:"invalidated_models,omitempty"`
}

// SourceSnapshot contains local configuration only. It must not probe upstreams
// or discard declarations merely because an account is temporarily limited.
type CategoryDefaultSourceSnapshot struct {
	Group              *Group
	Accounts           []Account
	Routes             []CompositeModelRoute
	Channel            *Channel
	DeclarationEpoch   int64
	GrokMappingOptions *xai.ModelMappingOptions
}

type CategoryDefaultSourceCandidate struct {
	Group         *Group   `json:"-"`
	Models        []string `json:"models"`
	ModelsCount   int      `json:"models_count"`
	InvalidReason string   `json:"invalid_reason,omitempty"`
}

type CategoryDefaultModelConflict struct {
	PublicModel      string   `json:"public_model"`
	SourceGroupIDs   []int64  `json:"source_group_ids"`
	SourceGroupNames []string `json:"source_group_names"`
}

type CategoryDefaultGroupPreview struct {
	Category       string                          `json:"category"`
	Revision       int64                           `json:"revision"`
	PreviewHash    string                          `json:"preview_hash"`
	SourceGroupIDs []int64                         `json:"source_group_ids"`
	Added          []CategoryDefaultPublishedModel `json:"added"`
	Modified       []CategoryDefaultPublishedModel `json:"modified"`
	Removed        []CategoryDefaultPublishedModel `json:"removed"`
	Unchanged      []CategoryDefaultPublishedModel `json:"unchanged"`
	Conflicts      []CategoryDefaultModelConflict  `json:"conflicts"`
	ModelsCount    int                             `json:"models_count"`
}

type CategoryDefaultGroupApplyRequest struct {
	SourceGroupIDs   []int64 `json:"source_group_ids"`
	ExpectedRevision int64   `json:"expected_revision"`
	PreviewHash      string  `json:"preview_hash"`
}

type CategoryDefaultModelRoute struct {
	EntryGroupID int64
	SourceGroup  *Group
	PublicModel  string
	SourceModel  string
	GeminiNative bool
}

// Validation runs inside the publish transaction against freshly loaded
// source declarations, so a preview cannot authorize a stale client manifest.
type CategoryDefaultPublishValidator func(*CategoryDefaultGroupConfig, []CategoryDefaultSourceSnapshot) ([]CategoryDefaultPublishedModel, error)

type CategoryDefaultGroupRepository interface {
	List(context.Context) ([]CategoryDefaultGroupConfig, error)
	Get(context.Context, string) (*CategoryDefaultGroupConfig, error)
	GetByGroupID(context.Context, int64) (*CategoryDefaultGroupConfig, error)
	Sources(context.Context, []int64) ([]CategoryDefaultSourceSnapshot, error)
	CandidateSources(context.Context, string) ([]CategoryDefaultSourceSnapshot, error)
	Publish(context.Context, string, []int64, int64, CategoryDefaultPublishValidator) (*CategoryDefaultGroupConfig, error)
	SetStatus(context.Context, string, bool) (*CategoryDefaultGroupConfig, error)
}
