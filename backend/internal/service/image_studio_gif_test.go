//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func imageStudioAnimatedGIFFixture(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 32, 16), palette)
	second := image.NewPaletted(first.Bounds(), palette)
	first.SetColorIndex(0, 0, 1)
	second.SetColorIndex(1, 0, 2)
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{7, 23}, LoopCount: 5, Disposal: []byte{gif.DisposalBackground, gif.DisposalPrevious}}))
	return buf.Bytes()
}

func TestImageStudioGIFSniffsInlineBytesDespiteMissingOrWrongMetadata(t *testing.T) {
	data := imageStudioAnimatedGIFFixture(t)
	encoded := base64.StdEncoding.EncodeToString(data)
	for _, body := range []string{
		`{"data":[{"b64_json":"` + encoded + `"}]}`,
		`{"output_format":"png","data":[{"b64_json":"` + encoded + `","mime_type":"image/png"}]}`,
		`{"data":[{"b64_json":"data:image/jpeg;base64,` + encoded + `"}]}`,
		`{"data":[{"url":"data:image/png;base64,` + encoded + `"}]}`,
	} {
		got, mimeType, err := extractImageStudioBytesFromGatewayBody([]byte(body))
		require.NoError(t, err)
		require.Equal(t, data, got)
		require.Equal(t, "image/gif", mimeType)
	}
}

func TestImageStudioGIFPreservesEveryFrameWithoutCanvasNormalization(t *testing.T) {
	data := imageStudioAnimatedGIFFixture(t)
	for _, format := range []string{"gif", "png", "jpeg", "webp", ""} {
		got, mimeType, actualFormat, err := normalizeImageStudioOutputImage(data, "image/png", format, "opaque", "1:1", "3840x2160")
		require.NoError(t, err)
		require.Equal(t, data, got)
		require.Equal(t, "image/gif", mimeType)
		require.Equal(t, "gif", actualFormat)
		animation, err := gif.DecodeAll(bytes.NewReader(got))
		require.NoError(t, err)
		require.Len(t, animation.Image, 2)
		require.Equal(t, []int{7, 23}, animation.Delay)
		require.Equal(t, 5, animation.LoopCount)
	}
}

func TestImageStudioGIFMappedModelOptionsAndGenerationUseAutoSize(t *testing.T) {
	const publicModel = "绘图/loop"
	svc := NewImageStudioService(nil, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{publicModel: "gpt-image-2.6-sunburst", "gpt-image-2": "gpt-image-2"}}}}}, nil)
	options, err := svc.GetOptions(context.Background(), 7)
	require.NoError(t, err)
	encoded, err := json.Marshal(options)
	require.NoError(t, err)
	require.Contains(t, gjson.GetBytes(encoded, `groups.0.models.#(model=="绘图/loop").output_formats`).Raw, `"gif"`)
	require.NotContains(t, gjson.GetBytes(encoded, `groups.0.models.#(model=="gpt-image-2").output_formats`).Raw, `"gif"`)
	_, prepared, err := svc.prepareGenerateInput(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: publicModel, Prompt: "a looping lantern", Quality: "4K", AspectRatio: "16:9", OutputFormat: "gif"})
	require.NoError(t, err)
	require.Equal(t, "auto", prepared.Size)
	require.NotEqual(t, "4K", prepared.Quality)
	body, err := buildImageStudioGenerateBody(prepared)
	require.NoError(t, err)
	require.Equal(t, "auto", gjson.GetBytes(body, "size").String())
	require.Equal(t, "gif", gjson.GetBytes(body, "output_format").String())
	for _, field := range []string{"response_format", "stream", "partial_images", "output_compression", "prompt_rewriter", "frame_count", "frame_duration_ms"} {
		require.False(t, gjson.GetBytes(body, field).Exists(), field)
	}
	_, _, err = svc.prepareGenerateInput(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "gpt-image-2", Prompt: "loop", OutputFormat: "gif"})
	require.Error(t, err)
	_, _, err = svc.prepareEditInput(context.Background(), ImageStudioEditInput{UserID: 7, Model: publicModel, Prompt: "loop", OutputFormat: "gif", ReferenceImages: []ImageStudioReferenceImage{{Data: []byte("image"), ContentType: "image/png"}}})
	require.ErrorContains(t, err, "generation")
}

func TestImageStudioGIFBaseModelOptionsAndGeneration(t *testing.T) {
	for _, publicModel := range []string{"gpt-image-2.6", "绘图/base-loop"} {
		t.Run(publicModel, func(t *testing.T) {
			svc := NewImageStudioService(nil, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
			svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
			svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{publicModel: "gpt-image-2.6"}}}}}, nil)
			options, err := svc.GetOptions(context.Background(), 7)
			require.NoError(t, err)
			encoded, err := json.Marshal(options)
			require.NoError(t, err)
			require.Contains(t, gjson.GetBytes(encoded, fmt.Sprintf(`groups.0.models.#(model==%q).output_formats`, publicModel)).Raw, `"gif"`)
			_, prepared, err := svc.prepareGenerateInput(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: publicModel, Prompt: "a looping lantern", OutputFormat: "gif", Quality: "4K", AspectRatio: "16:9"})
			require.NoError(t, err)
			body, err := buildImageStudioGenerateBody(prepared)
			require.NoError(t, err)
			require.Equal(t, "gif", gjson.GetBytes(body, "output_format").String())
			require.Equal(t, "auto", gjson.GetBytes(body, "size").String())
			require.False(t, gjson.GetBytes(body, "response_format").Exists())
		})
	}
}

func TestImageStudioGIFBaseModelExecutorPreservesAnimation(t *testing.T) {
	gateway := &imageStudioGIFGateway{data: imageStudioAnimatedGIFFixture(t), channelModel: "channel-loop", accounts: []Account{
		imageStudioGIFAccount(1, "gpt-image-2"), imageStudioGIFAccount(2, "gpt-image-2.6"), imageStudioGIFAccount(3, "gpt-image-2.6"),
	}}
	gateway.accounts[1].Type = AccountTypeOAuth
	result, err := imageStudioGIFExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", Prompt: "loop", OutputFormat: "gif", Size: "auto"})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, gateway.selected)
	require.Equal(t, gateway.selected, gateway.released)
	require.Equal(t, []int64{3}, gateway.forwarded)
	require.Equal(t, gateway.data, result.ImageBytes)
	require.Equal(t, "image/gif", result.MimeType)
	require.Equal(t, "gif", gjson.GetBytes(gateway.forwardBody, "output_format").String())
	require.False(t, gjson.GetBytes(gateway.forwardBody, "response_format").Exists())
}

func TestImageStudioGIFRejectsTransparentBackgroundAndEditBody(t *testing.T) {
	require.NoError(t, ValidateImageStudioOutputOptions("gif", "auto"))
	require.Error(t, ValidateImageStudioOutputOptions("gif", "transparent"))
	_, _, err := buildImageStudioEditBody(ImageStudioEditInput{OutputFormat: "gif"})
	require.ErrorContains(t, err, "generation")
}

func TestImageStudioGIFStorageUsesActualFormatAndDimensions(t *testing.T) {
	data := imageStudioAnimatedGIFFixture(t)
	repo := &imageStudioRepoStub{}
	storage := &imageStudioStorageStub{}
	svc := NewImageStudioService(repo, nil)
	svc.SetStorageFactory(func(context.Context, *ImageStudioSettings) (ImageStorage, error) { return storage, nil })
	record, err := svc.storeExecutionResult(context.Background(), defaultImageStudioSettings(), ImageStudioModeGeneration, 7, "gpt-image-2.6-flare", "loop", "1:1", "1024x1024", "png", "auto", &ImageStudioExecutionResult{ImageBytes: data, MimeType: "image/png", OutputFormat: "png"})
	require.NoError(t, err)
	require.Equal(t, data, storage.putData)
	require.Equal(t, "image/gif", record.MimeType)
	require.Equal(t, "gif", record.OutputFormat)
	require.Equal(t, "32x16", record.Size)
	require.True(t, strings.HasSuffix(record.StorageObjectKey, ".gif"))
	require.True(t, strings.HasSuffix(imageStudioDownloadFileName(record), ".gif"))
	require.Equal(t, "image/gif", openAIImageOutputMIMEType("gif"))
}

func TestImageStudioGIFDoesNotMislabelStillResponse(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			var b bytes.Buffer
			img := image.NewRGBA(image.Rect(0, 0, 32, 16))
			if format == "png" {
				require.NoError(t, png.Encode(&b, img))
			} else {
				require.NoError(t, jpeg.Encode(&b, img, nil))
			}
			data := b.Bytes()
			got, mimeType, actual, err := normalizeImageStudioOutputImage(data, "image/gif", "gif", "auto", "auto", "auto")
			require.NoError(t, err)
			require.Equal(t, data, got)
			require.Equal(t, "image/"+format, mimeType)
			require.Equal(t, format, actual)
		})
	}
}

func TestImageStudioGIFRecoveryKeepsAutomaticAnimationRequest(t *testing.T) {
	attempts := imageStudioGenerationRecoveryAttempts(ImageStudioGenerateInput{OutputFormat: "gif", AspectRatio: "16:9", Quality: "4K", Size: "3840x2160"})
	require.Len(t, attempts, 2)
	for _, attempt := range attempts {
		require.Equal(t, "gif", attempt.OutputFormat)
		require.Equal(t, "auto", attempt.Size)
		require.Empty(t, attempt.Quality)
	}
}

type imageStudioGIFGateway struct {
	imageStudioGatewayStub
	accounts                      []Account
	selected, released, forwarded []int64
	channelModel                  string
	data                          []byte
	url                           string
	fetchURLs                     []string
	fetchErr                      error
	failures                      int
}

func (s *imageStudioGIFGateway) ParseOpenAIImagesRequest(c *gin.Context, body []byte) (*OpenAIImagesRequest, error) {
	return (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body)
}
func (s *imageStudioGIFGateway) ResolveChannelMappingAndRestrict(_ context.Context, _ *int64, model string) (ChannelMappingResult, bool) {
	return ChannelMappingResult{MappedModel: firstNonEmptyTrimmed(s.channelModel, model)}, false
}
func (s *imageStudioGIFGateway) SelectAccountWithSchedulerForImages(_ context.Context, _ *int64, _, _ string, excluded map[int64]struct{}, capability OpenAIImagesCapability) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	for i := range s.accounts {
		account := &s.accounts[i]
		if _, skip := excluded[account.ID]; skip {
			continue
		}
		s.selected = append(s.selected, account.ID)
		return &AccountSelectionResult{Account: account, ReleaseFunc: func() { s.released = append(s.released, account.ID) }}, OpenAIAccountScheduleDecision{}, nil
	}
	return nil, OpenAIAccountScheduleDecision{}, ErrNoAvailableAccounts
}
func (s *imageStudioGIFGateway) ForwardImages(_ context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, mapped string) (*OpenAIForwardResult, error) {
	s.forwarded = append(s.forwarded, account.ID)
	s.forwardBody = body
	if s.failures > 0 {
		s.failures--
		return nil, &UpstreamFailoverError{StatusCode: 503}
	}
	item := gin.H{"b64_json": base64.StdEncoding.EncodeToString(s.data)}
	if s.url != "" {
		item = gin.H{"url": s.url}
	}
	c.JSON(http.StatusOK, gin.H{"output_format": "gif", "data": []gin.H{item}})
	return &OpenAIForwardResult{Model: parsed.Model, UpstreamModel: account.GetMappedModel(mapped), ImageCount: 1, ImageSize: parsed.SizeTier}, nil
}
func (s *imageStudioGIFGateway) fetchOpenAIImageURLBase64(_ context.Context, _ *Account, url string) (string, error) {
	s.fetchURLs = append(s.fetchURLs, url)
	return base64.StdEncoding.EncodeToString(s.data), s.fetchErr
}
func imageStudioGIFExecutor(gateway ImageStudioGateway) *ImageStudioGatewayExecutor {
	id := int64(9)
	key := &APIKey{ID: 22, UserID: 7, Status: StatusAPIKeyActive, GroupID: &id, User: &User{ID: 7, Balance: 10}, Group: &Group{ID: id, Platform: PlatformOpenAI, AllowImageGeneration: true}}
	return NewImageStudioGatewayExecutor(&imageStudioAPIKeyProviderStub{apiKey: key}, nil, nil, gateway)
}
func imageStudioGIFAccount(id int64, model string) Account {
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"channel-loop": model, "绘图/loop": model}}}
}

func TestImageStudioGIFMixedPoolSkipsUnsupportedWithoutConsumingForwardBudget(t *testing.T) {
	gateway := &imageStudioGIFGateway{data: imageStudioAnimatedGIFFixture(t), channelModel: "channel-loop", accounts: []Account{
		imageStudioGIFAccount(1, "gpt-image-2"), imageStudioGIFAccount(2, "gpt-image-1.5"), imageStudioGIFAccount(3, "gpt-image-2.6-flare"), imageStudioGIFAccount(4, "gpt-image-2.6-sunburst"),
	}}
	gateway.accounts[2].Type = AccountTypeOAuth
	result, err := imageStudioGIFExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", Prompt: "loop", OutputFormat: "gif", Size: "auto"})
	require.NoError(t, err)
	require.Equal(t, gateway.data, result.ImageBytes)
	require.Equal(t, []int64{1, 2, 3, 4}, gateway.selected)
	require.Equal(t, []int64{1, 2, 3, 4}, gateway.released)
	require.Equal(t, []int64{4}, gateway.forwarded)
	require.Equal(t, "gif", gjson.GetBytes(gateway.forwardBody, "output_format").String())
	require.False(t, gjson.GetBytes(gateway.forwardBody, "stream").Exists())
	require.False(t, gjson.GetBytes(gateway.forwardBody, "response_format").Exists())
}

func TestImageStudioGIFExecutorRejectsUnsupportedAndEditInputs(t *testing.T) {
	gateway := &imageStudioGIFGateway{accounts: []Account{imageStudioGIFAccount(1, "gpt-image-2")}}
	executor := imageStudioGIFExecutor(gateway)
	_, err := executor.Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", OutputFormat: "gif", Prompt: "loop"})
	require.Error(t, err)
	require.Empty(t, gateway.forwarded)
	_, err = executor.Edit(context.Background(), ImageStudioEditInput{UserID: 7, Model: "gpt-image-2.6-flare", OutputFormat: "gif"})
	require.ErrorContains(t, err, "generation")
	_, err = executor.Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "gpt-image-2.6-flare", OutputFormat: "gif", Background: "transparent"})
	require.Error(t, err)
	require.Empty(t, gateway.forwarded)
}

func TestImageStudioGIFURLResponseUsesSafeFetcher(t *testing.T) {
	for _, reject := range []bool{false, true} {
		gateway := &imageStudioGIFGateway{data: imageStudioAnimatedGIFFixture(t), url: "https://cdn.example.com/loop.gif", accounts: []Account{imageStudioGIFAccount(1, "gpt-image-2.6-flare")}}
		if reject {
			gateway.fetchErr = fmt.Errorf("image url host is not allowed")
		}
		result, err := imageStudioGIFExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", Prompt: "loop", OutputFormat: "gif"})
		require.Equal(t, []string{gateway.url}, gateway.fetchURLs)
		if reject {
			require.ErrorContains(t, err, "not allowed")
		} else {
			require.NoError(t, err)
			require.Equal(t, "image/gif", result.MimeType)
			require.Equal(t, gateway.data, result.ImageBytes)
		}
	}
}

func TestImageStudioGIFChannelAndAccountMappingCapability(t *testing.T) {
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{9}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"绘图/loop": "channel-loop"}}}
	channels := newTestChannelService(makeStandardRepo(channel, map[int64]string{9: PlatformOpenAI}))
	svc := NewImageStudioService(nil, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{imageStudioGIFAccount(1, "gpt-image-2.6-flare")}}, channels)
	options, err := svc.GetOptions(context.Background(), 7)
	require.NoError(t, err)
	encoded, err := json.Marshal(options)
	require.NoError(t, err)
	require.Contains(t, gjson.GetBytes(encoded, `groups.0.models.#(model=="绘图/loop").output_formats`).Raw, `"gif"`)
}

func TestImageStudioGIFMixedPoolStillLimitsRealUpstreamAttempts(t *testing.T) {
	gateway := &imageStudioGIFGateway{channelModel: "channel-loop", failures: 10}
	for i := int64(1); i <= 3; i++ {
		gateway.accounts = append(gateway.accounts, imageStudioGIFAccount(i, "gpt-image-2"))
	}
	for i := int64(4); i <= 8; i++ {
		gateway.accounts = append(gateway.accounts, imageStudioGIFAccount(i, "gpt-image-2.6-flare"))
	}
	_, err := imageStudioGIFExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", Prompt: "loop", OutputFormat: "gif"})
	require.Error(t, err)
	require.Equal(t, []int64{4, 5, 6}, gateway.forwarded)
	require.Equal(t, gateway.selected, gateway.released)
}

func TestImageStudioGIFDownloadKeepsAnimationBytes(t *testing.T) {
	data := imageStudioAnimatedGIFFixture(t)
	record := &ImageStudioImageRecord{ID: 20, UserID: 7, StorageDriver: "local", StorageObjectKey: "loop.gif", MimeType: "image/gif", OutputFormat: "gif", Bytes: int64(len(data))}
	repo := &imageStudioRepoStub{getByIDRecord: record}
	storage := &imageStudioStorageStub{openFile: &ImageStudioStoredFile{Reader: bytes.NewReader(data), ContentType: "image/gif", Size: int64(len(data)), Close: func() error { return nil }}}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetStorageFactory(func(context.Context, *ImageStudioSettings) (ImageStorage, error) { return storage, nil })
	file, err := svc.Download(context.Background(), 7, 20)
	require.NoError(t, err)
	defer file.Close()
	got, err := io.ReadAll(file.Reader)
	require.NoError(t, err)
	require.Equal(t, data, got)
	require.Equal(t, "image/gif", file.ContentType)
	require.True(t, strings.HasSuffix(file.Name, ".gif"))
}
