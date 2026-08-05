package routes

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type customGroupAliasResolverStub struct {
	resolution *service.CustomGroupModelResolution
	err        error
}

func (s customGroupAliasResolverStub) ResolveCustomGroupModel(context.Context, *service.APIKey, string) (*service.CustomGroupModelResolution, error) {
	return s.resolution, s.err
}

func TestRewriteCustomGroupRequestModelUsesRealModel(t *testing.T) {
	body := []byte(`{"model":"claude-opus-discount","messages":[]}`)

	rewritten, err := rewriteCustomGroupRequestModel("application/json", body, "claude-opus-discount", "claude-opus-4-6")

	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-6", gjson.GetBytes(rewritten, "model").String())
}

func TestRewriteCustomGroupRequestModelKeepsMatchingModel(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-6","messages":[]}`)

	rewritten, err := rewriteCustomGroupRequestModel("application/json", body, "claude-opus-4-6", "claude-opus-4-6")

	require.NoError(t, err)
	require.Equal(t, body, rewritten)
}

func TestRewriteCustomGroupRequestModelRewritesMultipartAndPreservesFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "image-alias"))
	file, err := writer.CreateFormFile("image", "sample.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("png-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	rewritten, err := rewriteCustomGroupRequestModel(writer.FormDataContentType(), body.Bytes(), "image-alias", "gpt-image-1")
	require.NoError(t, err)

	reader := multipart.NewReader(bytes.NewReader(rewritten), writer.Boundary())
	form, err := reader.ReadForm(1024)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-1"}, form.Value["model"])
	opened, err := form.File["image"][0].Open()
	require.NoError(t, err)
	defer opened.Close()
	contents, err := io.ReadAll(opened)
	require.NoError(t, err)
	require.Equal(t, []byte("png-bytes"), contents)
}

func TestCustomGroupGeminiTargetMiddlewareRewritesPathAndBindsSourceGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	customGroupID, sourceGroupID := int64(7), int64(42)
	originalKey := &service.APIKey{UserID: 9, CustomGroupID: &customGroupID}
	resolvedKey := &service.APIKey{UserID: 9, CustomGroupID: &customGroupID, GroupID: &sourceGroupID, Group: &service.Group{ID: sourceGroupID, Platform: service.PlatformGemini}}
	resolver := customGroupAliasResolverStub{resolution: &service.CustomGroupModelResolution{APIKey: resolvedKey, PublicModel: "gemini-fast", SourceModel: "gemini-2.5-flash"}}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyAPIKey), originalKey); c.Next() })
	router.Use(customGroupGeminiTargetMiddleware(resolver))
	router.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		key, _ := middleware.GetAPIKeyFromContext(c)
		publicModel, sourceModel, ok := service.CustomGroupModelResolutionFromContext(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{"path_model": compositeGeminiModelFromParams(c), "group_id": *key.GroupID, "public_model": publicModel, "source_model": sourceModel})
	})

	request := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-fast:generateContent", bytes.NewBufferString(`{"contents":[]}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"path_model":"gemini-2.5-flash","group_id":42,"public_model":"gemini-fast","source_model":"gemini-2.5-flash"}`, recorder.Body.String())
}

func TestCustomGroupModelContextPreservesBothNames(t *testing.T) {
	ctx := service.WithCustomGroupModelResolution(context.Background(), "claude-opus-discount", "claude-opus-4-6")

	publicModel, sourceModel, ok := service.CustomGroupModelResolutionFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "claude-opus-discount", publicModel)
	require.Equal(t, "claude-opus-4-6", sourceModel)
}

func TestCustomGroupTargetMiddlewareRewritesAliasAndBindsSourceGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	customGroupID := int64(7)
	sourceGroupID := int64(42)
	originalKey := &service.APIKey{UserID: 9, CustomGroupID: &customGroupID}
	resolvedKey := &service.APIKey{UserID: 9, CustomGroupID: &customGroupID, GroupID: &sourceGroupID}
	resolver := customGroupAliasResolverStub{resolution: &service.CustomGroupModelResolution{
		APIKey:      resolvedKey,
		PublicModel: "claude-opus-discount",
		SourceModel: "claude-opus-4-6",
	}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), originalKey)
		c.Next()
	})
	router.Use(customGroupTargetMiddleware(resolver))
	router.POST("/v1/messages", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		publicModel, sourceModel, ok := service.CustomGroupModelResolutionFromContext(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{
			"model":        gjson.GetBytes(body, "model").String(),
			"group_id":     *key.GroupID,
			"public_model": publicModel,
			"source_model": sourceModel,
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"claude-opus-discount","messages":[]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"model":"claude-opus-4-6","group_id":42,"public_model":"claude-opus-discount","source_model":"claude-opus-4-6"}`, recorder.Body.String())
}
