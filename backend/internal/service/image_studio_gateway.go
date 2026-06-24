package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type ImageStudioAPIKeyProvider interface {
	GetDefaultImageStudioAPIKey(ctx context.Context, userID int64) (*APIKey, error)
	GetImageStudioAPIKeyByID(ctx context.Context, userID int64, apiKeyID int64) (*APIKey, error)
	GetImageStudioAPIKeyForGroup(ctx context.Context, userID int64, groupID int64) (*APIKey, error)
}

type ImageStudioBillingChecker interface {
	CheckBillingEligibility(ctx context.Context, user *User, apiKey *APIKey, group *Group, subscription *UserSubscription, platform string) error
}

type ImageStudioGateway interface {
	ParseOpenAIImagesRequest(c *gin.Context, body []byte) (*OpenAIImagesRequest, error)
	ResolveChannelMappingAndRestrict(ctx context.Context, groupID *int64, model string) (ChannelMappingResult, bool)
	SelectAccountWithSchedulerForImages(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, requiredCapability OpenAIImagesCapability) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ForwardImages(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, channelMappedModel string) (*OpenAIForwardResult, error)
	RecordUsage(ctx context.Context, input *OpenAIRecordUsageInput) error
}

type ImageStudioGatewayExecutor struct {
	apiKeys        ImageStudioAPIKeyProvider
	billingChecker ImageStudioBillingChecker
	subscriptions  UserSubscriptionRepository
	gateway        ImageStudioGateway
}

func NewImageStudioGatewayExecutor(
	apiKeys ImageStudioAPIKeyProvider,
	billingChecker ImageStudioBillingChecker,
	subscriptions UserSubscriptionRepository,
	gateway ImageStudioGateway,
) *ImageStudioGatewayExecutor {
	return &ImageStudioGatewayExecutor{
		apiKeys:        apiKeys,
		billingChecker: billingChecker,
		subscriptions:  subscriptions,
		gateway:        gateway,
	}
}

func (e *ImageStudioGatewayExecutor) Generate(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioExecutionResult, error) {
	body, err := buildImageStudioGenerateBody(input)
	if err != nil {
		return nil, err
	}
	return e.execute(ctx, imageStudioGatewayExecutionInput{
		UserID:           input.UserID,
		APIKeyID:         input.APIKeyID,
		GroupID:          input.GroupID,
		Model:            input.Model,
		Size:             input.Size,
		BillingTier:      input.BillingTier,
		OutputFormat:     input.OutputFormat,
		Background:       input.Background,
		UserAgent:        input.UserAgent,
		IPAddress:        input.IPAddress,
		Endpoint:         openAIImagesGenerationsEndpoint,
		Body:             body,
		ContentType:      "application/json",
		SourceImageCount: 0,
	})
}

func (e *ImageStudioGatewayExecutor) Edit(ctx context.Context, input ImageStudioEditInput) (*ImageStudioExecutionResult, error) {
	body, contentType, err := buildImageStudioEditBody(input)
	if err != nil {
		return nil, err
	}
	return e.execute(ctx, imageStudioGatewayExecutionInput{
		UserID:           input.UserID,
		APIKeyID:         input.APIKeyID,
		GroupID:          input.GroupID,
		Model:            input.Model,
		Size:             input.Size,
		BillingTier:      input.BillingTier,
		OutputFormat:     input.OutputFormat,
		Background:       input.Background,
		UserAgent:        input.UserAgent,
		IPAddress:        input.IPAddress,
		Endpoint:         openAIImagesEditsEndpoint,
		Body:             body,
		ContentType:      contentType,
		SourceImageCount: len(input.ReferenceImages),
	})
}

type imageStudioGatewayExecutionInput struct {
	UserID           int64
	APIKeyID         *int64
	GroupID          *int64
	Model            string
	Size             string
	BillingTier      string
	OutputFormat     string
	Background       string
	UserAgent        string
	IPAddress        string
	Endpoint         string
	Body             []byte
	ContentType      string
	SourceImageCount int
}

func (e *ImageStudioGatewayExecutor) execute(ctx context.Context, input imageStudioGatewayExecutionInput) (*ImageStudioExecutionResult, error) {
	if e == nil || e.apiKeys == nil || e.gateway == nil {
		return nil, infraerrors.ServiceUnavailable("IMAGE_STUDIO_GATEWAY_NOT_CONFIGURED", "image studio gateway is not configured")
	}
	apiKey, err := e.resolveAPIKey(ctx, input.UserID, input.APIKeyID, input.GroupID)
	if err != nil {
		return nil, fmt.Errorf("get image studio api key: %w", err)
	}
	if apiKey == nil {
		return nil, ErrAPIKeyNotFound
	}
	ensureImageStudioAPIKeyContext(apiKey, input.UserID)
	if !GroupAllowsImageGeneration(apiKey.Group) {
		return nil, infraerrors.Forbidden("IMAGE_GENERATION_NOT_ALLOWED", ImageGenerationPermissionMessage())
	}

	subscription, err := e.resolveSubscription(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if e.billingChecker != nil {
		if err := e.billingChecker.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription, QuotaPlatform(ctx, apiKey)); err != nil {
			return nil, err
		}
	}

	ginCtx, recorder := newImageStudioGatewayGinContext(input.Endpoint, input.ContentType, input.Body, input.UserAgent, input.IPAddress)
	parsed, err := e.gateway.ParseOpenAIImagesRequest(ginCtx, input.Body)
	if err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
	}
	channelMapping, _ := e.gateway.ResolveChannelMappingAndRestrict(ctx, apiKey.GroupID, parsed.Model)

	selection, _, err := e.gateway.SelectAccountWithSchedulerForImages(
		WithOpenAIImageGenerationIntent(ctx),
		apiKey.GroupID,
		parsed.StickySessionSeed(),
		parsed.Model,
		nil,
		parsed.RequiredCapability,
	)
	if err != nil {
		return nil, fmt.Errorf("select image account: %w", err)
	}
	if selection == nil || selection.Account == nil {
		return nil, ErrNoAvailableAccounts
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	account := selection.Account

	result, err := e.gateway.ForwardImages(WithOpenAIImageGenerationIntent(ctx), ginCtx, account, input.Body, parsed, channelMapping.MappedModel)
	if err != nil {
		return nil, fmt.Errorf("forward image request: %w", err)
	}
	if result == nil {
		return nil, infraerrors.ServiceUnavailable("IMAGE_GENERATION_EMPTY_RESULT", "image provider returned no image")
	}
	ApplyOpenAIImageBillingResolution(result)

	imageBytes, mimeType, err := extractImageStudioBytesFromGatewayBody(recorder.Body.Bytes())
	if err != nil {
		return nil, err
	}
	outputFormat, background := extractImageStudioOutputMetadataFromGatewayBody(recorder.Body.Bytes())
	usageInput := &OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    input.Endpoint,
		UpstreamEndpoint:   imageStudioUpstreamEndpoint(input.Endpoint, account.Platform),
		UserAgent:          input.UserAgent,
		IPAddress:          input.IPAddress,
		RequestPayloadHash: imageStudioUsagePayloadHash(parsed, input.Body),
		ChannelUsageFields: channelMapping.ToUsageFields(parsed.Model, result.UpstreamModel),
	}

	return &ImageStudioExecutionResult{
		ImageBytes:       imageBytes,
		MimeType:         mimeType,
		OutputFormat:     firstNonEmptyTrimmed(outputFormat, input.OutputFormat),
		Background:       firstNonEmptyTrimmed(background, input.Background),
		Cost:             estimateImageStudioGatewayCost(apiKey, result),
		RequestID:        result.RequestID,
		SourceImageCount: input.SourceImageCount,
		CommitUsage: func(commitCtx context.Context) error {
			return e.gateway.RecordUsage(commitCtx, usageInput)
		},
	}, nil
}

func (e *ImageStudioGatewayExecutor) resolveAPIKey(ctx context.Context, userID int64, apiKeyID *int64, groupID *int64) (*APIKey, error) {
	if apiKeyID != nil && *apiKeyID > 0 {
		return e.apiKeys.GetImageStudioAPIKeyByID(ctx, userID, *apiKeyID)
	}
	if groupID != nil && *groupID > 0 {
		return e.apiKeys.GetImageStudioAPIKeyForGroup(ctx, userID, *groupID)
	}
	return e.apiKeys.GetDefaultImageStudioAPIKey(ctx, userID)
}

func estimateImageStudioGatewayCost(apiKey *APIKey, result *OpenAIForwardResult) float64 {
	if result == nil || result.ImageCount <= 0 {
		return 0
	}
	count := result.ImageCount
	if count <= 0 {
		count = 1
	}
	return estimateImageStudioCost(apiKey.Group, result.ImageSize) * float64(count)
}

func (e *ImageStudioGatewayExecutor) resolveSubscription(ctx context.Context, apiKey *APIKey) (*UserSubscription, error) {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() || e.subscriptions == nil {
		return nil, nil
	}
	sub, err := e.subscriptions.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, *apiKey.GroupID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func buildImageStudioGenerateBody(input ImageStudioGenerateInput) ([]byte, error) {
	body := map[string]any{
		"model":           strings.TrimSpace(input.Model),
		"prompt":          strings.TrimSpace(input.Prompt),
		"size":            strings.TrimSpace(input.Size),
		"n":               1,
		"response_format": "b64_json",
	}
	if outputFormat := strings.TrimSpace(input.OutputFormat); outputFormat != "" {
		body["output_format"] = outputFormat
	}
	if background := strings.TrimSpace(input.Background); background != "" {
		body["background"] = background
	}
	return json.Marshal(body)
}

func buildImageStudioEditBody(input ImageStudioEditInput) ([]byte, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fields := map[string]string{
		"model":           strings.TrimSpace(input.Model),
		"prompt":          strings.TrimSpace(input.Prompt),
		"size":            strings.TrimSpace(input.Size),
		"response_format": "b64_json",
	}
	if outputFormat := strings.TrimSpace(input.OutputFormat); outputFormat != "" {
		fields["output_format"] = outputFormat
	}
	if background := strings.TrimSpace(input.Background); background != "" {
		fields["background"] = background
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", fmt.Errorf("write image edit field %s: %w", key, err)
		}
	}
	for i, image := range input.ReferenceImages {
		fieldName := "image"
		if i > 0 {
			fieldName = fmt.Sprintf("image[%d]", i)
		}
		part, err := createImageStudioEditFilePart(writer, fieldName, image)
		if err != nil {
			return nil, "", fmt.Errorf("create image edit part: %w", err)
		}
		if _, err := part.Write(image.Data); err != nil {
			return nil, "", fmt.Errorf("write image edit part: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finalize image edit body: %w", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func createImageStudioEditFilePart(writer *multipart.Writer, fieldName string, image ImageStudioReferenceImage) (io.Writer, error) {
	contentType := strings.TrimSpace(image.ContentType)
	if contentType == "" && len(image.Data) > 0 {
		contentType = http.DetectContentType(image.Data)
	}
	if mediaType, _, ok := strings.Cut(contentType, ";"); ok {
		contentType = strings.TrimSpace(mediaType)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") && len(image.Data) > 0 {
		if detected := http.DetectContentType(image.Data); strings.HasPrefix(strings.ToLower(detected), "image/") {
			contentType = detected
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", multipart.FileContentDisposition(fieldName, firstNonEmptyTrimmed(image.FileName, "reference.png")))
	header.Set("Content-Type", contentType)
	return writer.CreatePart(header)
}

func ensureImageStudioAPIKeyContext(apiKey *APIKey, userID int64) {
	if apiKey.User == nil {
		apiKey.User = &User{ID: userID}
	}
	if apiKey.UserID == 0 {
		apiKey.UserID = userID
	}
}

func newImageStudioGatewayGinContext(endpoint string, contentType string, body []byte, userAgent string, ipAddress string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.TrimSpace(userAgent) != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if strings.TrimSpace(ipAddress) != "" {
		req.Header.Set("X-Forwarded-For", ipAddress)
	}
	ginCtx.Request = req
	return ginCtx, recorder
}

func extractImageStudioBytesFromGatewayBody(body []byte) ([]byte, string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, "", infraerrors.ServiceUnavailable("IMAGE_GENERATION_EMPTY_RESULT", "image provider returned no image")
	}
	if decoded, ok, err := decodeImageStudioBase64(gjson.GetBytes(body, "data.0.b64_json").String()); ok || err != nil {
		if err != nil {
			return nil, "", err
		}
		outputFormat, _ := extractImageStudioOutputMetadataFromGatewayBody(body)
		return decoded, openAIImageOutputMIMEType(outputFormat), nil
	}
	for _, item := range collectOpenAIImageInlineAssets(body, "") {
		if decoded, ok, err := decodeImageStudioBase64(item.B64JSON); ok || err != nil {
			if err != nil {
				return nil, "", err
			}
			return decoded, firstNonEmptyTrimmed(item.MimeType, "image/png"), nil
		}
		if strings.HasPrefix(strings.ToLower(item.DownloadURL), "data:image/") {
			decoded, mimeType, err := decodeImageStudioDataURL(item.DownloadURL)
			if err != nil {
				return nil, "", err
			}
			return decoded, mimeType, nil
		}
	}
	return nil, "", infraerrors.ServiceUnavailable("IMAGE_GENERATION_EMPTY_RESULT", "image provider returned no inline image")
}

func extractImageStudioOutputMetadataFromGatewayBody(body []byte) (string, string) {
	outputFormat := strings.TrimSpace(gjson.GetBytes(body, "output_format").String())
	background := strings.TrimSpace(gjson.GetBytes(body, "background").String())
	if outputFormat == "" {
		outputFormat = strings.TrimSpace(gjson.GetBytes(body, "data.0.output_format").String())
	}
	if background == "" {
		background = strings.TrimSpace(gjson.GetBytes(body, "data.0.background").String())
	}
	return outputFormat, background
}

func decodeImageStudioDataURL(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	header, encoded, ok := strings.Cut(raw, ",")
	if !ok || strings.TrimSpace(encoded) == "" {
		return nil, "", fmt.Errorf("invalid image data url")
	}
	mimeType := strings.TrimPrefix(strings.ToLower(header), "data:")
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = mimeType[:idx]
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	decoded, ok, err := decodeImageStudioBase64(encoded)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", fmt.Errorf("invalid image data url")
	}
	return decoded, mimeType, nil
}

func decodeImageStudioBase64(raw string) ([]byte, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		if _, encoded, ok := strings.Cut(raw, ","); ok {
			raw = strings.TrimSpace(encoded)
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, true, nil
	}
	normalized := normalizeOpenAIImageBase64(raw)
	if normalized == "" {
		return nil, false, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return nil, false, fmt.Errorf("decode generated image: %w", err)
	}
	return decoded, true, nil
}

func imageStudioUsagePayloadHash(parsed *OpenAIImagesRequest, body []byte) string {
	if parsed != nil && parsed.Multipart {
		return HashUsageRequestPayload([]byte(parsed.StickySessionSeed()))
	}
	return HashUsageRequestPayload(body)
}

func imageStudioUpstreamEndpoint(inboundEndpoint string, platform string) string {
	inboundEndpoint = strings.TrimSpace(inboundEndpoint)
	if inboundEndpoint == "" {
		inboundEndpoint = openAIImagesGenerationsEndpoint
	}
	if strings.TrimSpace(platform) == PlatformOpenAI {
		return inboundEndpoint
	}
	return inboundEndpoint
}

var _ = io.EOF
