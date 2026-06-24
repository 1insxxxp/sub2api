//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type imageStudioAPIKeyProviderStub struct {
	apiKey       *APIKey
	keysByID     map[int64]*APIKey
	keysByGroup  map[int64]*APIKey
	err          error
	defaultCalls int
	keyCalls     []int64
	groupCalls   []int64
}

func (s *imageStudioAPIKeyProviderStub) GetDefaultImageStudioAPIKey(ctx context.Context, userID int64) (*APIKey, error) {
	s.defaultCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.apiKey, nil
}

func (s *imageStudioAPIKeyProviderStub) GetImageStudioAPIKeyByID(ctx context.Context, userID int64, apiKeyID int64) (*APIKey, error) {
	s.keyCalls = append(s.keyCalls, apiKeyID)
	if s.err != nil {
		return nil, s.err
	}
	if s.keysByID != nil {
		return s.keysByID[apiKeyID], nil
	}
	return s.apiKey, nil
}

func (s *imageStudioAPIKeyProviderStub) GetImageStudioAPIKeyForGroup(ctx context.Context, userID int64, groupID int64) (*APIKey, error) {
	s.groupCalls = append(s.groupCalls, groupID)
	if s.err != nil {
		return nil, s.err
	}
	if s.keysByGroup != nil {
		return s.keysByGroup[groupID], nil
	}
	return s.apiKey, nil
}

type imageStudioBillingCheckerStub struct {
	err   error
	calls int
}

func (s *imageStudioBillingCheckerStub) CheckBillingEligibility(ctx context.Context, user *User, apiKey *APIKey, group *Group, subscription *UserSubscription, platform string) error {
	s.calls++
	return s.err
}

type imageStudioGatewayStub struct {
	parseBody        []byte
	parseContentType string
	parsedEndpoint   string
	forwardBody      []byte
	mappingGroupID   *int64
	schedulerGroupID *int64
	usageInput       *OpenAIRecordUsageInput
	forwardResult    *OpenAIForwardResult
	forwardError     error
	recordCalls      int
}

func (s *imageStudioGatewayStub) ParseOpenAIImagesRequest(c *gin.Context, body []byte) (*OpenAIImagesRequest, error) {
	s.parseBody = append([]byte(nil), body...)
	s.parseContentType = c.GetHeader("Content-Type")
	s.parsedEndpoint = c.Request.URL.Path
	if strings.Contains(c.Request.URL.Path, "/edits") {
		return &OpenAIImagesRequest{
			Endpoint:           openAIImagesEditsEndpoint,
			ContentType:        c.GetHeader("Content-Type"),
			Multipart:          true,
			Model:              "gpt-image-1",
			Prompt:             "edit it",
			Size:               "1024x1024",
			SizeTier:           ImageBillingSize1K,
			ResponseFormat:     "b64_json",
			N:                  1,
			RequiredCapability: OpenAIImagesCapabilityNative,
			Uploads:            []OpenAIImagesUpload{{FieldName: "image", FileName: "source.png", ContentType: "image/png", Data: []byte("source")}},
		}, nil
	}
	return &OpenAIImagesRequest{
		Endpoint:           openAIImagesGenerationsEndpoint,
		ContentType:        c.GetHeader("Content-Type"),
		Model:              "gpt-image-1",
		Prompt:             "draw it",
		Size:               "1536x864",
		SizeTier:           ImageBillingSize2K,
		ResponseFormat:     "b64_json",
		N:                  1,
		RequiredCapability: OpenAIImagesCapabilityNative,
	}, nil
}

func (s *imageStudioGatewayStub) ResolveChannelMappingAndRestrict(ctx context.Context, groupID *int64, model string) (ChannelMappingResult, bool) {
	if groupID != nil {
		id := *groupID
		s.mappingGroupID = &id
	} else {
		s.mappingGroupID = nil
	}
	return ChannelMappingResult{MappedModel: model, BillingModelSource: BillingModelSourceChannelMapped}, false
}

func (s *imageStudioGatewayStub) SelectAccountWithSchedulerForImages(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, requiredCapability OpenAIImagesCapability) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if groupID != nil {
		id := *groupID
		s.schedulerGroupID = &id
	} else {
		s.schedulerGroupID = nil
	}
	return &AccountSelectionResult{Account: &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}}, OpenAIAccountScheduleDecision{}, nil
}

func (s *imageStudioGatewayStub) ForwardImages(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, channelMappedModel string) (*OpenAIForwardResult, error) {
	s.forwardBody = append([]byte(nil), body...)
	if s.forwardError != nil {
		return nil, s.forwardError
	}
	if s.forwardResult == nil {
		s.forwardResult = &OpenAIForwardResult{
			RequestID:      "req-image",
			Model:          parsed.Model,
			UpstreamModel:  parsed.Model,
			ImageCount:     1,
			ImageSize:      parsed.SizeTier,
			ImageInputSize: parsed.Size,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": []gin.H{{"b64_json": base64.StdEncoding.EncodeToString([]byte("image-bytes"))}}})
	return s.forwardResult, nil
}

func (s *imageStudioGatewayStub) RecordUsage(ctx context.Context, input *OpenAIRecordUsageInput) error {
	s.recordCalls++
	s.usageInput = input
	return nil
}

func TestImageStudioGatewayExecutorGenerateBuildsOpenAIRequestAndDefersUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(3)
	apiKey := &APIKey{
		ID:      22,
		UserID:  7,
		Status:  StatusAPIKeyActive,
		GroupID: &groupID,
		User:    &User{ID: 7, Balance: 10},
		Group:   &Group{ID: groupID, AllowImageGeneration: true},
	}
	gateway := &imageStudioGatewayStub{}
	billing := &imageStudioBillingCheckerStub{}
	executor := NewImageStudioGatewayExecutor(
		&imageStudioAPIKeyProviderStub{apiKey: apiKey},
		billing,
		nil,
		gateway,
	)

	result, err := executor.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:       7,
		Model:        "gpt-image-1",
		Prompt:       "draw it",
		AspectRatio:  "16:9",
		Size:         "1536x864",
		BillingTier:  ImageBillingSize2K,
		OutputFormat: "webp",
		Background:   "transparent",
		UserAgent:    "studio-test",
		IPAddress:    "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("image-bytes"), result.ImageBytes)
	require.Equal(t, "image/png", result.MimeType)
	require.Equal(t, 0, gateway.recordCalls)
	require.Equal(t, 1, billing.calls)
	require.Equal(t, "/v1/images/generations", gateway.parsedEndpoint)
	require.Equal(t, "application/json", gateway.parseContentType)
	require.Equal(t, "gpt-image-1", gjson.GetBytes(gateway.parseBody, "model").String())
	require.Equal(t, "draw it", gjson.GetBytes(gateway.parseBody, "prompt").String())
	require.Equal(t, "1536x864", gjson.GetBytes(gateway.parseBody, "size").String())
	require.Equal(t, "b64_json", gjson.GetBytes(gateway.parseBody, "response_format").String())
	require.Equal(t, "webp", gjson.GetBytes(gateway.parseBody, "output_format").String())
	require.Equal(t, "transparent", gjson.GetBytes(gateway.parseBody, "background").String())
	require.Equal(t, int64(1), gjson.GetBytes(gateway.parseBody, "n").Int())

	require.NoError(t, result.CommitUsage(context.Background()))
	require.Equal(t, 1, gateway.recordCalls)
}

func TestImageStudioGatewayExecutorGenerateUsesSelectedGroupAPIKeyAndScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	selectedGroupID := int64(9)
	apiKey := &APIKey{
		ID:      45,
		UserID:  7,
		Status:  StatusAPIKeyActive,
		GroupID: &selectedGroupID,
		User:    &User{ID: 7, Balance: 10},
		Group:   &Group{ID: selectedGroupID, AllowImageGeneration: true},
	}
	keyProvider := &imageStudioAPIKeyProviderStub{
		keysByGroup: map[int64]*APIKey{selectedGroupID: apiKey},
	}
	gateway := &imageStudioGatewayStub{}
	executor := NewImageStudioGatewayExecutor(
		keyProvider,
		&imageStudioBillingCheckerStub{},
		nil,
		gateway,
	)

	result, err := executor.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		GroupID:     &selectedGroupID,
		Model:       "gpt-image-2",
		Prompt:      "draw it",
		AspectRatio: "16:9",
		Size:        "3840x2160",
		BillingTier: ImageBillingSize4K,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, keyProvider.defaultCalls)
	require.Equal(t, []int64{selectedGroupID}, keyProvider.groupCalls)
	require.NotNil(t, gateway.mappingGroupID)
	require.Equal(t, selectedGroupID, *gateway.mappingGroupID)
	require.NotNil(t, gateway.schedulerGroupID)
	require.Equal(t, selectedGroupID, *gateway.schedulerGroupID)

	require.NoError(t, result.CommitUsage(context.Background()))
	require.NotNil(t, gateway.usageInput)
	require.Equal(t, apiKey.ID, gateway.usageInput.APIKey.ID)
	require.NotNil(t, gateway.usageInput.APIKey.GroupID)
	require.Equal(t, selectedGroupID, *gateway.usageInput.APIKey.GroupID)
}

func TestImageStudioGatewayExecutorGeneratePrefersSelectedAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	selectedKeyID := int64(45)
	selectedGroupID := int64(9)
	legacyGroupID := int64(12)
	apiKey := &APIKey{
		ID:      selectedKeyID,
		UserID:  7,
		Status:  StatusAPIKeyActive,
		GroupID: &selectedGroupID,
		User:    &User{ID: 7, Balance: 10},
		Group:   &Group{ID: selectedGroupID, AllowImageGeneration: true},
	}
	keyProvider := &imageStudioAPIKeyProviderStub{
		keysByID: map[int64]*APIKey{selectedKeyID: apiKey},
	}
	gateway := &imageStudioGatewayStub{}
	executor := NewImageStudioGatewayExecutor(
		keyProvider,
		&imageStudioBillingCheckerStub{},
		nil,
		gateway,
	)

	result, err := executor.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		APIKeyID:    &selectedKeyID,
		GroupID:     &legacyGroupID,
		Model:       "gpt-image-2",
		Prompt:      "draw it",
		AspectRatio: "16:9",
		Size:        "3840x2160",
		BillingTier: ImageBillingSize4K,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, keyProvider.defaultCalls)
	require.Equal(t, []int64{selectedKeyID}, keyProvider.keyCalls)
	require.Empty(t, keyProvider.groupCalls)
	require.NotNil(t, gateway.mappingGroupID)
	require.Equal(t, selectedGroupID, *gateway.mappingGroupID)
}

func TestImageStudioGatewayExecutorEditBuildsMultipartRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := &APIKey{
		ID:     23,
		UserID: 7,
		Status: StatusAPIKeyActive,
		User:   &User{ID: 7, Balance: 10},
		Group:  &Group{ID: 3, AllowImageGeneration: true},
	}
	gateway := &imageStudioGatewayStub{}
	executor := NewImageStudioGatewayExecutor(
		&imageStudioAPIKeyProviderStub{apiKey: apiKey},
		&imageStudioBillingCheckerStub{},
		nil,
		gateway,
	)

	result, err := executor.Edit(context.Background(), ImageStudioEditInput{
		UserID:       7,
		Model:        "gpt-image-1",
		Prompt:       "edit it",
		AspectRatio:  "1:1",
		Size:         "1024x1024",
		BillingTier:  ImageBillingSize1K,
		OutputFormat: "jpeg",
		Background:   "opaque",
		ReferenceImages: []ImageStudioReferenceImage{{
			FileName:    "source.png",
			ContentType: "image/png",
			Data:        []byte("source"),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, []byte("image-bytes"), result.ImageBytes)
	require.Contains(t, gateway.parseContentType, "multipart/form-data")
	require.Equal(t, "/v1/images/edits", gateway.parsedEndpoint)

	reader := multipartReaderFromRecordedBody(t, gateway.parseBody, gateway.parseContentType)
	parts := map[string]string{}
	contentTypes := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		parts[part.FormName()] = string(body)
		contentTypes[part.FormName()] = part.Header.Get("Content-Type")
	}
	require.Equal(t, "gpt-image-1", parts["model"])
	require.Equal(t, "edit it", parts["prompt"])
	require.Equal(t, "1024x1024", parts["size"])
	require.Equal(t, "b64_json", parts["response_format"])
	require.Equal(t, "jpeg", parts["output_format"])
	require.Equal(t, "opaque", parts["background"])
	require.Equal(t, "source", parts["image"])
	require.Equal(t, "image/png", contentTypes["image"])
}

func TestExtractImageStudioBytesUsesOutputFormatForInlineBase64(t *testing.T) {
	body := []byte(`{"created":1710000001,"output_format":"webp","data":[{"b64_json":"aW1hZ2UtYnl0ZXM="}]}`)

	decoded, mimeType, err := extractImageStudioBytesFromGatewayBody(body)

	require.NoError(t, err)
	require.Equal(t, []byte("image-bytes"), decoded)
	require.Equal(t, "image/webp", mimeType)
}

func TestImageStudioGatewayExecutorRejectsGroupWithoutImagePermission(t *testing.T) {
	apiKey := &APIKey{
		ID:     24,
		UserID: 7,
		Status: StatusAPIKeyActive,
		User:   &User{ID: 7, Balance: 10},
		Group:  &Group{ID: 3, AllowImageGeneration: false},
	}
	executor := NewImageStudioGatewayExecutor(
		&imageStudioAPIKeyProviderStub{apiKey: apiKey},
		&imageStudioBillingCheckerStub{},
		nil,
		&imageStudioGatewayStub{},
	)

	_, err := executor.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "draw it",
		AspectRatio: "1:1",
		Size:        "1024x1024",
		BillingTier: ImageBillingSize1K,
	})
	require.ErrorContains(t, err, ImageGenerationPermissionMessage())
}

func multipartReaderFromRecordedBody(t *testing.T, body []byte, contentType string) *multipart.Reader {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	boundary := params["boundary"]
	require.NotEmpty(t, boundary)
	return multipart.NewReader(bytes.NewReader(body), boundary)
}
