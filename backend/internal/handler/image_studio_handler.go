package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ImageStudioService interface {
	GetConfig(ctx context.Context) (*service.ImageStudioConfig, error)
	GetOptions(ctx context.Context, userID int64) (*service.ImageStudioOptions, error)
	CreateTask(ctx context.Context, input service.ImageStudioTaskCreateInput) (*service.ImageStudioTask, error)
	GetTask(ctx context.Context, userID int64, taskID int64) (*service.ImageStudioTask, error)
	ListTasks(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageStudioTask, *pagination.PaginationResult, error)
	Generate(ctx context.Context, input service.ImageStudioGenerateInput) (*service.ImageStudioImageRecord, error)
	Edit(ctx context.Context, input service.ImageStudioEditInput) (*service.ImageStudioImageRecord, error)
	List(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageStudioImageRecord, *pagination.PaginationResult, error)
	Delete(ctx context.Context, userID int64, imageID int64) error
	OpenLocalFile(ctx context.Context, objectKey string) (*service.ImageStudioLocalFile, error)
}

// ImageStudioHandler handles authenticated user image generation workspace APIs.
type ImageStudioHandler struct {
	imageStudioService ImageStudioService
}

func NewImageStudioHandler(imageStudioService ImageStudioService) *ImageStudioHandler {
	return &ImageStudioHandler{imageStudioService: imageStudioService}
}

func (h *ImageStudioHandler) GetConfig(c *gin.Context) {
	cfg, err := h.imageStudioService.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ImageStudioHandler) GetOptions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	options, err := h.imageStudioService.GetOptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, options)
}

type imageStudioGenerateRequest struct {
	APIKeyID    *int64 `json:"api_key_id"`
	GroupID     *int64 `json:"group_id"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt" binding:"required"`
	AspectRatio string `json:"aspect_ratio"`
	Quality     string `json:"quality"`
}

func (h *ImageStudioHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req imageStudioGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	record, err := h.imageStudioService.Generate(c.Request.Context(), service.ImageStudioGenerateInput{
		UserID:      subject.UserID,
		APIKeyID:    req.APIKeyID,
		GroupID:     req.GroupID,
		Model:       req.Model,
		Prompt:      req.Prompt,
		AspectRatio: req.AspectRatio,
		Quality:     req.Quality,
		UserAgent:   c.GetHeader("User-Agent"),
		IPAddress:   c.ClientIP(),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

type imageStudioTaskRequest struct {
	Mode        string `json:"mode"`
	APIKeyID    *int64 `json:"api_key_id"`
	GroupID     *int64 `json:"group_id"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt" binding:"required"`
	AspectRatio string `json:"aspect_ratio"`
	Quality     string `json:"quality"`
}

func (h *ImageStudioHandler) CreateTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req imageStudioTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = service.ImageStudioModeGeneration
	}
	input := service.ImageStudioTaskCreateInput{}
	switch mode {
	case service.ImageStudioModeGeneration:
		input.Generate = &service.ImageStudioGenerateInput{
			UserID:      subject.UserID,
			APIKeyID:    req.APIKeyID,
			GroupID:     req.GroupID,
			Model:       req.Model,
			Prompt:      req.Prompt,
			AspectRatio: req.AspectRatio,
			Quality:     req.Quality,
			UserAgent:   c.GetHeader("User-Agent"),
			IPAddress:   c.ClientIP(),
		}
	case service.ImageStudioModeEdit:
		response.BadRequest(c, "Image edit tasks are not available yet")
		return
	default:
		response.BadRequest(c, "Invalid image task mode")
		return
	}
	task, err := h.imageStudioService.CreateTask(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, task)
}

func (h *ImageStudioHandler) GetTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid image task id")
		return
	}
	task, err := h.imageStudioService.GetTask(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *ImageStudioHandler) ListTasks(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, pageResult, err := h.imageStudioService.ListTasks(c.Request.Context(), subject.UserID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    pageResult.Total,
		Page:     pageResult.Page,
		PageSize: pageResult.PageSize,
		Pages:    pageResult.Pages,
	})
}

func (h *ImageStudioHandler) Edit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		response.BadRequest(c, "Invalid multipart request: "+err.Error())
		return
	}
	images, err := imageStudioReferenceImagesFromMultipart(c.Request.MultipartForm)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	groupID, err := parseOptionalImageStudioInt64(c.PostForm("group_id"))
	if err != nil {
		response.BadRequest(c, "Invalid group_id")
		return
	}
	apiKeyID, err := parseOptionalImageStudioInt64(c.PostForm("api_key_id"))
	if err != nil {
		response.BadRequest(c, "Invalid api_key_id")
		return
	}
	record, err := h.imageStudioService.Edit(c.Request.Context(), service.ImageStudioEditInput{
		UserID:          subject.UserID,
		APIKeyID:        apiKeyID,
		GroupID:         groupID,
		Model:           c.PostForm("model"),
		Prompt:          c.PostForm("prompt"),
		AspectRatio:     c.PostForm("aspect_ratio"),
		Quality:         c.PostForm("quality"),
		ReferenceImages: images,
		UserAgent:       c.GetHeader("User-Agent"),
		IPAddress:       c.ClientIP(),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

func (h *ImageStudioHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, pageResult, err := h.imageStudioService.List(c.Request.Context(), subject.UserID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    pageResult.Total,
		Page:     pageResult.Page,
		PageSize: pageResult.PageSize,
		Pages:    pageResult.Pages,
	})
}

func (h *ImageStudioHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid image id")
		return
	}
	if err := h.imageStudioService.Delete(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ImageStudioHandler) ServeFile(c *gin.Context) {
	objectKey := strings.TrimLeft(strings.TrimSpace(c.Param("objectKey")), "/")
	file, err := h.imageStudioService.OpenLocalFile(c.Request.Context(), objectKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if file.Close != nil {
		defer file.Close()
	}
	if file.ContentType != "" {
		c.Header("Content-Type", file.ContentType)
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(c.Writer, c.Request, file.Name, file.ModTime, file.Reader)
}

func imageStudioReferenceImagesFromMultipart(form *multipart.Form) ([]service.ImageStudioReferenceImage, error) {
	if form == nil || len(form.File) == 0 {
		return nil, nil
	}
	var out []service.ImageStudioReferenceImage
	for field, files := range form.File {
		if field != "image" && !strings.HasPrefix(field, "image[") && field != "reference_image" {
			continue
		}
		for _, header := range files {
			file, err := header.Open()
			if err != nil {
				return nil, err
			}
			data, readErr := io.ReadAll(io.LimitReader(file, 64<<20))
			closeErr := file.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
			if contentType == "" {
				contentType = http.DetectContentType(data)
			}
			out = append(out, service.ImageStudioReferenceImage{
				FileName:    header.Filename,
				ContentType: contentType,
				Data:        bytes.Clone(data),
			})
		}
	}
	return out, nil
}

func parseOptionalImageStudioInt64(raw string) (*int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value <= 0 {
		if err == nil {
			err = strconv.ErrSyntax
		}
		return nil, err
	}
	return &value, nil
}
