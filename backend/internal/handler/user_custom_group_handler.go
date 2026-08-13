package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserCustomGroupHandler struct {
	service *service.UserCustomGroupService
	apiKeys *service.APIKeyService
	gateway *service.GatewayService
}

func NewUserCustomGroupHandler(s *service.UserCustomGroupService, apiKeys *service.APIKeyService, gateway *service.GatewayService) *UserCustomGroupHandler {
	return &UserCustomGroupHandler{service: s, apiKeys: apiKeys, gateway: gateway}
}

func customGroupSubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func customGroupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid custom group ID")
		return 0, false
	}
	return id, true
}

func (h *UserCustomGroupHandler) List(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	items, err := h.service.List(c.Request.Context(), uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *UserCustomGroupHandler) Get(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	id, ok := customGroupID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserCustomGroupHandler) Create(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	var req service.CreateUserCustomGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserCustomGroupHandler) Update(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	id, ok := customGroupID(c)
	if !ok {
		return
	}
	var req service.UpdateUserCustomGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserCustomGroupHandler) Delete(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	id, ok := customGroupID(c)
	if !ok {
		return
	}
	force := false
	if rawForce, exists := c.GetQuery("force"); exists {
		parsedForce, err := strconv.ParseBool(rawForce)
		if err != nil {
			response.BadRequest(c, "Invalid force value")
			return
		}
		force = parsedForce
	}
	unboundCount, err := h.service.Delete(c.Request.Context(), uid, id, force)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "unbound_api_key_count": unboundCount})
}

type customGroupCandidate struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Platform string   `json:"platform"`
	Models   []string `json:"models"`
}

func (h *UserCustomGroupHandler) Candidates(c *gin.Context) {
	uid, ok := customGroupSubject(c)
	if !ok {
		return
	}
	groups, err := h.apiKeys.GetAvailableGroups(c.Request.Context(), uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]customGroupCandidate, 0, len(groups))
	for _, g := range groups {
		if g.Platform == service.PlatformComposite || g.Status != service.StatusActive {
			continue
		}
		models := h.gateway.GetAvailableModels(c.Request.Context(), &g.ID, g.Platform)
		if g.CustomModelsListEnabled() {
			models = filterModelsByCustomList(models, defaultModelIDsForPlatform(g.Platform), g.ModelsListConfig.Models)
		}
		if models == nil {
			models = make([]string, 0)
		}
		out = append(out, customGroupCandidate{ID: g.ID, Name: g.Name, Platform: g.Platform, Models: models})
	}
	response.Success(c, out)
}
