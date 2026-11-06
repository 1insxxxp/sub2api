//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"mime"
	"mime/multipart"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newImageStudioGIFEditTestService(model, upstream, accountType string) *ImageStudioService {
	cfg := defaultImageStudioSettings()
	cfg.Enabled, cfg.MaxReferenceImageMB = true, 1
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: cfg})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{{ID: 12, Platform: PlatformOpenAI, Type: accountType, Credentials: map[string]any{"model_mapping": map[string]any{model: upstream}}}}}, nil)
	return svc
}

func TestImageStudioGIFEditCapabilityUsesFinalAccountModel(t *testing.T) {
	for _, tc := range []struct {
		model, upstream, accountType string
		generationGIF, editGIF       bool
	}{
		{"gpt-image-2.6", "gpt-image-2.6", AccountTypeAPIKey, true, true},
		{"绘图/loop", "gpt-image-2.6", AccountTypeAPIKey, true, true},
		{"绘图/loop", "gpt-image-2.6-sunburst", AccountTypeAPIKey, true, false},
		{"绘图/loop", "gpt-image-2.6-flare", AccountTypeAPIKey, true, false},
		{"gpt-image-2.6", "gpt-image-1.5", AccountTypeAPIKey, false, false},
		{"gpt-image-2.6", "gpt-image-2.6", AccountTypeOAuth, false, false},
	} {
		t.Run(tc.model+"-"+tc.upstream+"-"+tc.accountType, func(t *testing.T) {
			svc := newImageStudioGIFEditTestService(tc.model, tc.upstream, tc.accountType)
			options, err := svc.GetOptions(context.Background(), 7)
			require.NoError(t, err)
			encoded, err := json.Marshal(options)
			require.NoError(t, err)
			path := fmt.Sprintf(`groups.0.models.#(model==%q)`, tc.model)
			require.Equal(t, tc.generationGIF, gjson.GetBytes(encoded, path+`.output_formats.#(=="gif")`).Exists())
			require.Equal(t, tc.editGIF, gjson.GetBytes(encoded, path+`.edit_output_formats.#(=="gif")`).Exists())
		})
	}
}

func TestImageStudioGIFEditNormalizesAutomaticOutputAndValidatesReferences(t *testing.T) {
	const model = "绘图/loop"
	svc := newImageStudioGIFEditTestService(model, "gpt-image-2.6", AccountTypeAPIKey)
	ref := ImageStudioReferenceImage{Data: imageStudioTestPNG(t, 8, 8, color.White), FileName: "reference.png", ContentType: "image/png"}
	input := ImageStudioEditInput{UserID: 7, Model: model, Prompt: "animate this reference", OutputFormat: "gif", AspectRatio: "16:9", Quality: "4K", ReferenceImages: []ImageStudioReferenceImage{ref}}
	_, prepared, err := svc.prepareEditInput(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "auto", prepared.Size)
	require.Equal(t, "auto", prepared.AspectRatio)
	require.Empty(t, prepared.Quality)
	require.Empty(t, prepared.BillingTier)
	require.Equal(t, input.ReferenceImages, prepared.ReferenceImages)
	for _, tc := range []struct {
		name   string
		mutate func(*ImageStudioEditInput)
		reason string
	}{
		{"missing", func(in *ImageStudioEditInput) { in.ReferenceImages = nil }, "reference image is required"},
		{"empty", func(in *ImageStudioEditInput) {
			in.ReferenceImages = []ImageStudioReferenceImage{{ContentType: "image/png"}}
		}, "reference image is empty"},
		{"oversized", func(in *ImageStudioEditInput) {
			in.ReferenceImages = []ImageStudioReferenceImage{{ContentType: "image/png", Data: make([]byte, (1<<20)+1)}}
		}, "reference image exceeds"},
		{"non-image", func(in *ImageStudioEditInput) {
			in.ReferenceImages = []ImageStudioReferenceImage{{ContentType: "text/plain", Data: []byte("text")}}
		}, "reference file must be an image"},
		{"transparent", func(in *ImageStudioEditInput) { in.Background = "transparent" }, "transparent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := input
			tc.mutate(&invalid)
			_, _, err := svc.prepareEditInput(context.Background(), invalid)
			require.ErrorContains(t, err, tc.reason)
		})
	}
}

type imageStudioGIFEditGateway struct {
	imageStudioGIFGateway
	contentType, endpoint string
}

func (s *imageStudioGIFEditGateway) ParseOpenAIImagesRequest(c *gin.Context, body []byte) (*OpenAIImagesRequest, error) {
	// Keep the boundary too, for verifying the exact forwarded multipart body.
	s.contentType, s.endpoint = c.GetHeader("Content-Type"), c.Request.URL.Path
	return s.imageStudioGIFGateway.ParseOpenAIImagesRequest(c, body)
}

func TestImageStudioGIFEditPreservesReferencesAndAnimationThroughStorage(t *testing.T) {
	const model = "绘图/loop"
	svc := newImageStudioGIFEditTestService(model, "gpt-image-2.6", AccountTypeAPIKey)
	data := imageStudioAnimatedGIFFixture(t)
	gateway := &imageStudioGIFEditGateway{imageStudioGIFGateway: imageStudioGIFGateway{data: data, channelModel: "channel-loop", accounts: []Account{
		imageStudioGIFAccount(1, "gpt-image-2.6-sunburst"), imageStudioGIFAccount(2, "gpt-image-2.6"), imageStudioGIFAccount(3, "gpt-image-2.6-flare"),
		imageStudioGIFAccount(4, "gpt-image-1.5"), imageStudioGIFAccount(5, "gpt-image-2.6"),
	}}}
	gateway.accounts[1].Type = AccountTypeOAuth
	svc.SetExecutor(imageStudioGIFExecutor(gateway))
	storage := &imageStudioStorageStub{}
	svc.SetStorageFactory(func(context.Context, *ImageStudioSettings) (ImageStorage, error) { return storage, nil })
	ref := ImageStudioReferenceImage{Data: imageStudioTestPNG(t, 8, 8, color.White), FileName: "reference.png", ContentType: "image/png"}
	input := ImageStudioEditInput{UserID: 7, Model: model, Prompt: "keep the reference subject and animate it", OutputFormat: "gif", Quality: "4K", AspectRatio: "16:9", ReferenceImages: []ImageStudioReferenceImage{ref, ref}}
	record, err := svc.Edit(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, gateway.selected)
	require.Equal(t, gateway.selected, gateway.released)
	require.Equal(t, []int64{5}, gateway.forwarded)
	require.Equal(t, openAIImagesEditsEndpoint, gateway.endpoint)
	_, params, err := mime.ParseMediaType(gateway.contentType)
	require.NoError(t, err)
	form, err := multipart.NewReader(bytes.NewReader(gateway.forwardBody), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Equal(t, []string{"gif"}, form.Value["output_format"])
	require.Equal(t, []string{"auto"}, form.Value["size"])
	require.Equal(t, []string{input.Prompt}, form.Value["prompt"])
	for _, key := range []string{"quality", "response_format", "stream", "partial_images", "frame_count"} {
		require.NotContains(t, form.Value, key)
	}
	for _, key := range []string{"image", "image[1]"} {
		require.Len(t, form.File[key], 1)
		f, err := form.File[key][0].Open()
		require.NoError(t, err)
		got, err := io.ReadAll(f)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		require.Equal(t, ref.Data, got)
	}
	require.Equal(t, data, storage.putData)
	require.Equal(t, "gif", record.OutputFormat)
	require.Equal(t, "image/gif", record.MimeType)
	require.Equal(t, "32x16", record.Size)
	require.Equal(t, ImageStudioModeEdit, record.Mode)
	require.Equal(t, 2, record.SourceImageCount)
	require.Equal(t, 1, gateway.recordCalls)
}

func TestImageStudioGIFEditCapabilityFollowsChannelMapping(t *testing.T) {
	const alias = "绘图/loop"
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{9}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {alias: "channel-loop"}}}
	for _, upstream := range []string{"gpt-image-2.6", "gpt-image-2.6-flare"} {
		t.Run(upstream, func(t *testing.T) {
			channels := newTestChannelService(makeStandardRepo(channel, map[int64]string{9: PlatformOpenAI}))
			svc := newImageStudioGIFEditTestService(alias, "gpt-image-2.6", AccountTypeAPIKey)
			account := imageStudioGIFAccount(1, upstream)
			// The direct public mapping deliberately contradicts the channel route.
			account.Credentials["model_mapping"].(map[string]any)[alias] = "gpt-image-2.6-sunburst"
			svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{account}}, channels)
			options, err := svc.GetOptions(context.Background(), 7)
			require.NoError(t, err)
			encoded, err := json.Marshal(options)
			require.NoError(t, err)
			require.Equal(t, upstream == "gpt-image-2.6", gjson.GetBytes(encoded, `groups.0.models.#(model=="绘图/loop").edit_output_formats.#(=="gif")`).Exists())
			_, _, err = svc.prepareEditInput(context.Background(), ImageStudioEditInput{UserID: 7, Model: alias, Prompt: "animate", OutputFormat: "gif", ReferenceImages: []ImageStudioReferenceImage{{Data: imageStudioTestPNG(t, 8, 8, color.White), ContentType: "image/png"}}})
			if upstream == "gpt-image-2.6" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "GIF edit route")
			}
		})
	}
}

func TestImageStudioGIFEditExecutorRejectsUnsupportedRoutesBeforeForwarding(t *testing.T) {
	for _, upstream := range []string{"gpt-image-1.5", "gpt-image-2.6-sunburst", "gpt-image-2.6-flare"} {
		t.Run(upstream, func(t *testing.T) {
			gateway := &imageStudioGIFGateway{channelModel: "channel-loop", accounts: []Account{imageStudioGIFAccount(1, upstream)}}
			_, err := imageStudioGIFExecutor(gateway).Edit(context.Background(), ImageStudioEditInput{UserID: 7, Model: "绘图/loop", Prompt: "animate", OutputFormat: "gif", ReferenceImages: []ImageStudioReferenceImage{{Data: imageStudioTestPNG(t, 8, 8, color.White), ContentType: "image/png"}}})
			require.ErrorContains(t, err, "GIF edit route")
			require.Equal(t, []int64{1}, gateway.released)
			require.Empty(t, gateway.forwarded)
			require.Zero(t, gateway.recordCalls)
		})
	}
}

func TestImageStudioGIFEditExecutorUsesFinalChannelRoute(t *testing.T) {
	for _, tc := range []struct {
		public, upstream string
		supported        bool
	}{
		{"gpt-image-2.6", "gpt-image-2.6-flare", false},
		{"gpt-image-2.6-sunburst", "gpt-image-2.6", true},
	} {
		t.Run(tc.public+"-"+tc.upstream, func(t *testing.T) {
			account := imageStudioGIFAccount(1, tc.upstream)
			account.Credentials["model_mapping"].(map[string]any)["绘图/loop"] = tc.public
			gateway := &imageStudioGIFGateway{data: imageStudioAnimatedGIFFixture(t), channelModel: "channel-loop", accounts: []Account{account}}
			result, err := imageStudioGIFExecutor(gateway).Edit(context.Background(), ImageStudioEditInput{UserID: 7, Model: "绘图/loop", Prompt: "animate", OutputFormat: "gif", ReferenceImages: []ImageStudioReferenceImage{{Data: imageStudioTestPNG(t, 8, 8, color.White), ContentType: "image/png"}}})
			require.Equal(t, []int64{1}, gateway.selected)
			require.Equal(t, []int64{1}, gateway.released)
			if tc.supported {
				require.NoError(t, err)
				require.Equal(t, []int64{1}, gateway.forwarded)
				require.Equal(t, gateway.data, result.ImageBytes)
			} else {
				require.ErrorContains(t, err, "GIF edit route")
				require.Empty(t, gateway.forwarded)
				require.Zero(t, gateway.recordCalls)
			}
		})
	}
}
