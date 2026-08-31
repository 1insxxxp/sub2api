package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type systemCustomGroupAdminService interface {
	Create(ctx context.Context, req service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error)
	Get(ctx context.Context, groupID int64) (*service.SystemCustomGroup, error)
	Update(ctx context.Context, groupID int64, req service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error)
	Candidates(ctx context.Context) ([]service.SystemCustomGroupCandidate, error)
	SyncPreview(ctx context.Context, groupID int64) (*service.SystemCustomGroupSyncPreview, error)
	Delete(ctx context.Context, groupID int64) error
}

// SystemCustomGroupHandler exposes administrator-only management of shared
// subscription groups. Validation and persistence invariants stay in service.
type SystemCustomGroupHandler struct {
	service systemCustomGroupAdminService
}

type systemCustomGroupContainerResponse struct {
	ID                         int64     `json:"id"`
	Name                       string    `json:"name"`
	Description                string    `json:"description"`
	Platform                   string    `json:"platform"`
	RateMultiplier             float64   `json:"rate_multiplier"`
	IsExclusive                bool      `json:"is_exclusive"`
	Status                     string    `json:"status"`
	SubscriptionType           string    `json:"subscription_type"`
	SystemCustomRoutingEnabled bool      `json:"system_custom_routing_enabled"`
	DailyLimitUSD              *float64  `json:"daily_limit_usd"`
	WeeklyLimitUSD             *float64  `json:"weekly_limit_usd"`
	MonthlyLimitUSD            *float64  `json:"monthly_limit_usd"`
	DefaultValidityDays        int       `json:"default_validity_days"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type systemCustomGroupSourceResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Platform         string `json:"platform,omitempty"`
	Status           string `json:"status,omitempty"`
	SubscriptionType string `json:"subscription_type,omitempty"`
}

type systemCustomGroupModelResponse struct {
	ID            int64                            `json:"id"`
	GroupID       int64                            `json:"group_id"`
	PublicModel   string                           `json:"public_model"`
	SourceGroupID int64                            `json:"source_group_id"`
	SourceModel   string                           `json:"source_model"`
	Enabled       bool                             `json:"enabled"`
	SourceGroup   *systemCustomGroupSourceResponse `json:"source_group,omitempty"`
	CreatedAt     time.Time                        `json:"created_at"`
	UpdatedAt     time.Time                        `json:"updated_at"`
}

type systemCustomGroupSourceSummaryResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Platform         string `json:"platform"`
	Status           string `json:"status"`
	SubscriptionType string `json:"subscription_type"`
}

type systemCustomGroupSourceReferenceResponse struct {
	ID            int64                                   `json:"id"`
	GroupID       int64                                   `json:"group_id"`
	SourceGroupID int64                                   `json:"source_group_id"`
	Priority      int                                     `json:"priority"`
	Group         *systemCustomGroupSourceSummaryResponse `json:"group"`
	CreatedAt     time.Time                               `json:"created_at"`
	UpdatedAt     time.Time                               `json:"updated_at"`
}

type systemCustomGroupSummaryResponse struct {
	UniqueModels       int `json:"unique_models"`
	FallbackRoutes     int `json:"fallback_routes"`
	UnavailableSources int `json:"unavailable_sources"`
	UnpricedRoutes     int `json:"unpriced_routes"`
}

type systemCustomGroupResponse struct {
	Group   systemCustomGroupContainerResponse         `json:"group"`
	Sources []systemCustomGroupSourceReferenceResponse `json:"sources"`
	Summary systemCustomGroupSummaryResponse           `json:"summary"`
	Models  []systemCustomGroupModelResponse           `json:"models"`
}

type systemCustomGroupCandidateResponse struct {
	Group  systemCustomGroupSourceResponse `json:"group"`
	Models []string                        `json:"models"`
}

type systemCustomGroupSyncPreviewResponse struct {
	Added       []service.SystemCustomGroupSyncAdded    `json:"added"`
	Missing     []systemCustomGroupModelResponse        `json:"missing"`
	Conflicting []service.SystemCustomGroupSyncConflict `json:"conflicting"`
}

func NewSystemCustomGroupHandler(service *service.SystemCustomGroupService) *SystemCustomGroupHandler {
	return &SystemCustomGroupHandler{service: service}
}

func (h *SystemCustomGroupHandler) Candidates(c *gin.Context) {
	candidates, err := h.service.Candidates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, systemCustomGroupCandidatesResponse(candidates))
}

func (h *SystemCustomGroupHandler) Create(c *gin.Context) {
	var req service.CreateSystemCustomGroupRequest
	if !decodeSystemCustomGroupBody(c, &req) {
		return
	}
	created, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, systemCustomGroupToResponse(created))
}

func (h *SystemCustomGroupHandler) Get(c *gin.Context) {
	groupID, ok := parseSystemCustomGroupID(c)
	if !ok {
		return
	}
	group, err := h.service.Get(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, systemCustomGroupToResponse(group))
}

func (h *SystemCustomGroupHandler) Update(c *gin.Context) {
	groupID, ok := parseSystemCustomGroupID(c)
	if !ok {
		return
	}
	var req service.UpdateSystemCustomGroupRequest
	if !decodeSystemCustomGroupBody(c, &req) {
		return
	}
	updated, err := h.service.Update(c.Request.Context(), groupID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, systemCustomGroupToResponse(updated))
}

// SyncPreview remains available for legacy administrator clients during the
// source-based contract migration.
func (h *SystemCustomGroupHandler) SyncPreview(c *gin.Context) {
	groupID, ok := parseSystemCustomGroupID(c)
	if !ok {
		return
	}
	preview, err := h.service.SyncPreview(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, systemCustomGroupSyncPreviewToResponse(preview))
}

type DeleteSystemCustomGroupResponse struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func (h *SystemCustomGroupHandler) Delete(c *gin.Context) {
	groupID, ok := parseSystemCustomGroupID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), groupID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, DeleteSystemCustomGroupResponse{ID: groupID, Deleted: true})
}

func parseSystemCustomGroupID(c *gin.Context) (int64, bool) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.ErrorFrom(c, service.ErrSystemCustomGroupInvalidInput)
		return 0, false
	}
	return groupID, true
}

func decodeSystemCustomGroupBody(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		response.ErrorFrom(c, service.ErrSystemCustomGroupInvalidInput)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.ErrorFrom(c, service.ErrSystemCustomGroupInvalidInput)
		return false
	}
	return true
}

func systemCustomGroupToResponse(group *service.SystemCustomGroup) systemCustomGroupResponse {
	sources := make([]systemCustomGroupSourceReferenceResponse, 0, len(group.Sources))
	for i := range group.Sources {
		sources = append(sources, systemCustomGroupSourceReferenceToResponse(group.Sources[i]))
	}
	models := make([]systemCustomGroupModelResponse, 0, len(group.Models))
	for i := range group.Models {
		models = append(models, systemCustomGroupModelToResponse(group.Models[i]))
	}
	return systemCustomGroupResponse{
		Group:   systemCustomGroupContainerToResponse(group.Group),
		Sources: sources,
		Summary: systemCustomGroupSummary(group),
		Models:  models,
	}
}

func systemCustomGroupContainerToResponse(group service.Group) systemCustomGroupContainerResponse {
	return systemCustomGroupContainerResponse{
		ID: group.ID, Name: group.Name, Description: group.Description, Platform: group.Platform,
		RateMultiplier: group.RateMultiplier, IsExclusive: group.IsExclusive, Status: group.Status,
		SubscriptionType: group.SubscriptionType, SystemCustomRoutingEnabled: group.SystemCustomRoutingEnabled,
		DailyLimitUSD: group.DailyLimitUSD, WeeklyLimitUSD: group.WeeklyLimitUSD, MonthlyLimitUSD: group.MonthlyLimitUSD,
		DefaultValidityDays: group.DefaultValidityDays, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
	}
}

func systemCustomGroupSourceToResponse(group service.Group) systemCustomGroupSourceResponse {
	return systemCustomGroupSourceResponse{
		ID: group.ID, Name: group.Name, Description: group.Description, Platform: group.Platform,
		Status: group.Status, SubscriptionType: group.SubscriptionType,
	}
}

func systemCustomGroupSourceReferenceToResponse(source service.SystemCustomGroupSource) systemCustomGroupSourceReferenceResponse {
	out := systemCustomGroupSourceReferenceResponse{
		ID: source.ID, GroupID: source.GroupID, SourceGroupID: source.SourceGroupID, Priority: source.Priority,
		CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt,
	}
	if source.SourceGroup != nil {
		out.Group = &systemCustomGroupSourceSummaryResponse{
			ID: source.SourceGroup.ID, Name: source.SourceGroup.Name, Description: source.SourceGroup.Description,
			Platform: source.SourceGroup.Platform, Status: source.SourceGroup.Status,
			SubscriptionType: source.SourceGroup.SubscriptionType,
		}
	}
	return out
}

func systemCustomGroupSummary(group *service.SystemCustomGroup) systemCustomGroupSummaryResponse {
	routeCounts := make(map[string]int)
	for i := range group.Models {
		model := group.Models[i]
		if !model.Enabled {
			continue
		}
		publicModel := strings.ToLower(strings.TrimSpace(model.PublicModel))
		if publicModel != "" {
			routeCounts[publicModel]++
		}
	}

	summary := systemCustomGroupSummaryResponse{UniqueModels: len(routeCounts)}
	for _, routeCount := range routeCounts {
		if routeCount > 1 {
			summary.FallbackRoutes += routeCount - 1
		}
	}
	for i := range group.Sources {
		if !service.IsEligibleSystemCustomSource(group.Sources[i].SourceGroup) {
			summary.UnavailableSources++
		}
	}
	// Effective pricing coverage is added by Task 5; retained routes alone cannot establish it.
	summary.UnpricedRoutes = 0
	return summary
}

func systemCustomGroupModelToResponse(model service.SystemCustomGroupModel) systemCustomGroupModelResponse {
	out := systemCustomGroupModelResponse{
		ID: model.ID, GroupID: model.GroupID, PublicModel: model.PublicModel, SourceGroupID: model.SourceGroupID,
		SourceModel: model.SourceModel, Enabled: model.Enabled, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
	if model.SourceGroup != nil {
		source := systemCustomGroupSourceToResponse(*model.SourceGroup)
		out.SourceGroup = &source
	}
	return out
}

func systemCustomGroupCandidatesResponse(candidates []service.SystemCustomGroupCandidate) []systemCustomGroupCandidateResponse {
	out := make([]systemCustomGroupCandidateResponse, 0, len(candidates))
	for i := range candidates {
		out = append(out, systemCustomGroupCandidateResponse{
			Group: systemCustomGroupSourceToResponse(candidates[i].Group), Models: candidates[i].Models,
		})
	}
	return out
}

func systemCustomGroupSyncPreviewToResponse(preview *service.SystemCustomGroupSyncPreview) systemCustomGroupSyncPreviewResponse {
	missing := make([]systemCustomGroupModelResponse, 0, len(preview.Missing))
	for i := range preview.Missing {
		missing = append(missing, systemCustomGroupModelToResponse(preview.Missing[i]))
	}
	return systemCustomGroupSyncPreviewResponse{Added: preview.Added, Missing: missing, Conflicting: preview.Conflicting}
}
