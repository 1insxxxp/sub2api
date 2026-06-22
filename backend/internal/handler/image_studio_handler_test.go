package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageStudioHandlerServiceStub struct {
	config        *service.ImageStudioConfig
	options       *service.ImageStudioOptions
	optionsUserID int64

	generateInput service.ImageStudioGenerateInput
	editInput     service.ImageStudioEditInput
	record        *service.ImageStudioImageRecord

	listUserID int64
	listItems  []service.ImageStudioImageRecord
	listPage   *pagination.PaginationResult

	deleteUserID int64
	deleteID     int64
}

func (s *imageStudioHandlerServiceStub) GetConfig(ctx context.Context) (*service.ImageStudioConfig, error) {
	if s.config != nil {
		return s.config, nil
	}
	return &service.ImageStudioConfig{Enabled: true, DefaultModel: "gpt-image-1"}, nil
}

func (s *imageStudioHandlerServiceStub) GetOptions(ctx context.Context, userID int64) (*service.ImageStudioOptions, error) {
	s.optionsUserID = userID
	if s.options != nil {
		return s.options, nil
	}
	groupID := int64(3)
	return &service.ImageStudioOptions{
		Enabled:        true,
		DefaultGroupID: &groupID,
		DefaultModel:   "gpt-image-2",
		Groups: []service.ImageStudioGroupOption{{
			ID:   groupID,
			Name: "Image Pro",
			Models: []service.ImageStudioModelOption{{
				Model: "gpt-image-2",
				Label: "gpt-image-2",
			}},
		}},
	}, nil
}

func (s *imageStudioHandlerServiceStub) Generate(ctx context.Context, input service.ImageStudioGenerateInput) (*service.ImageStudioImageRecord, error) {
	s.generateInput = input
	if s.record != nil {
		return s.record, nil
	}
	return &service.ImageStudioImageRecord{ID: 9, UserID: input.UserID, ImageURL: "https://assets.example.com/one.png"}, nil
}

func (s *imageStudioHandlerServiceStub) Edit(ctx context.Context, input service.ImageStudioEditInput) (*service.ImageStudioImageRecord, error) {
	s.editInput = input
	if s.record != nil {
		return s.record, nil
	}
	return &service.ImageStudioImageRecord{ID: 10, UserID: input.UserID, ImageURL: "https://assets.example.com/edit.png"}, nil
}

func (s *imageStudioHandlerServiceStub) List(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageStudioImageRecord, *pagination.PaginationResult, error) {
	s.listUserID = userID
	page := s.listPage
	if page == nil {
		page = &pagination.PaginationResult{Total: int64(len(s.listItems)), Page: params.Page, PageSize: params.Limit(), Pages: 1}
	}
	return s.listItems, page, nil
}

func (s *imageStudioHandlerServiceStub) Delete(ctx context.Context, userID int64, imageID int64) error {
	s.deleteUserID = userID
	s.deleteID = imageID
	return nil
}

func TestImageStudioHandlerGetConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &imageStudioHandlerServiceStub{
		config: &service.ImageStudioConfig{
			Enabled:       true,
			AllowedModels: []string{"gpt-image-1"},
			DefaultModel:  "gpt-image-1",
		},
	}
	h := NewImageStudioHandler(svc)
	c, rec := newImageStudioHandlerTestContext(http.MethodGet, "/api/v1/user/images/config", nil)

	h.GetConfig(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data := envelope["data"].(map[string]any)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, "gpt-image-1", data["default_model"])
}

func TestImageStudioHandlerGetOptionsUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(9)
	svc := &imageStudioHandlerServiceStub{
		options: &service.ImageStudioOptions{
			Enabled:        true,
			DefaultGroupID: &groupID,
			DefaultModel:   "gpt-image-2",
			Groups:         []service.ImageStudioGroupOption{{ID: groupID, Name: "Image Pro"}},
		},
	}
	h := NewImageStudioHandler(svc)
	c, rec := newImageStudioHandlerTestContext(http.MethodGet, "/api/v1/user/images/options", nil)

	h.GetOptions(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), svc.optionsUserID)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data := envelope["data"].(map[string]any)
	require.Equal(t, "gpt-image-2", data["default_model"])
	require.Len(t, data["groups"].([]any), 1)
}

func TestImageStudioHandlerGenerateUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &imageStudioHandlerServiceStub{}
	h := NewImageStudioHandler(svc)
	body := bytes.NewBufferString(`{"group_id":9,"model":"gpt-image-2","prompt":"blue portal","aspect_ratio":"16:9","quality":"4K"}`)
	c, rec := newImageStudioHandlerTestContext(http.MethodPost, "/api/v1/user/images/generate", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h.Generate(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), svc.generateInput.UserID)
	require.NotNil(t, svc.generateInput.GroupID)
	require.Equal(t, int64(9), *svc.generateInput.GroupID)
	require.Equal(t, "gpt-image-2", svc.generateInput.Model)
	require.Equal(t, "blue portal", svc.generateInput.Prompt)
	require.Equal(t, "16:9", svc.generateInput.AspectRatio)
	require.Equal(t, "4K", svc.generateInput.Quality)
}

func TestImageStudioHandlerEditForwardsGroupAndQuality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &imageStudioHandlerServiceStub{}
	h := NewImageStudioHandler(svc)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("group_id", "9"))
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "make it cleaner"))
	require.NoError(t, writer.WriteField("aspect_ratio", "9:16"))
	require.NoError(t, writer.WriteField("quality", "2K"))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, rec := newImageStudioHandlerTestContext(http.MethodPost, "/api/v1/user/images/edit", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	h.Edit(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), svc.editInput.UserID)
	require.NotNil(t, svc.editInput.GroupID)
	require.Equal(t, int64(9), *svc.editInput.GroupID)
	require.Equal(t, "gpt-image-2", svc.editInput.Model)
	require.Equal(t, "make it cleaner", svc.editInput.Prompt)
	require.Equal(t, "9:16", svc.editInput.AspectRatio)
	require.Equal(t, "2K", svc.editInput.Quality)
	require.Len(t, svc.editInput.ReferenceImages, 1)
}

func TestImageStudioHandlerListAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	svc := &imageStudioHandlerServiceStub{
		listItems: []service.ImageStudioImageRecord{{ID: 1, UserID: 42, ImageURL: "https://assets.example.com/one.png", CreatedAt: now}},
	}
	h := NewImageStudioHandler(svc)

	listCtx, listRec := newImageStudioHandlerTestContext(http.MethodGet, "/api/v1/user/images?page=1&page_size=10", nil)
	h.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Equal(t, int64(42), svc.listUserID)

	deleteCtx, deleteRec := newImageStudioHandlerTestContext(http.MethodDelete, "/api/v1/user/images/1", nil)
	deleteCtx.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Delete(deleteCtx)
	require.Equal(t, http.StatusOK, deleteRec.Code)
	require.Equal(t, int64(42), svc.deleteUserID)
	require.Equal(t, int64(1), svc.deleteID)
}

func newImageStudioHandlerTestContext(method, target string, body *bytes.Buffer) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var reqBody *bytes.Buffer
	if body != nil {
		reqBody = body
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	c.Request = httptest.NewRequest(method, target, reqBody)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, Concurrency: 2})
	return c, rec
}
