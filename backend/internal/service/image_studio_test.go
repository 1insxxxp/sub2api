package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioAspectRatioResolution(t *testing.T) {
	tests := []struct {
		name        string
		ratio       string
		wantSize    string
		wantBilling string
	}{
		{name: "square", ratio: "1:1", wantSize: "1024x1024", wantBilling: ImageBillingSize1K},
		{name: "landscape wide", ratio: "16:9", wantSize: "1536x864", wantBilling: ImageBillingSize2K},
		{name: "portrait wide", ratio: "9:16", wantSize: "864x1536", wantBilling: ImageBillingSize2K},
		{name: "landscape classic", ratio: "4:3", wantSize: "1024x768", wantBilling: ImageBillingSize1K},
		{name: "portrait classic", ratio: "3:4", wantSize: "768x1024", wantBilling: ImageBillingSize1K},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSize, gotBilling, err := ResolveImageStudioAspectRatio(tt.ratio)
			require.NoError(t, err)
			require.Equal(t, tt.wantSize, gotSize)
			require.Equal(t, tt.wantBilling, gotBilling)
		})
	}
}

func TestImageStudioAspectRatioResolutionRejectsUnknownRatio(t *testing.T) {
	_, _, err := ResolveImageStudioAspectRatio("2:1")
	require.Error(t, err)
}

func TestValidateImageStudioModel(t *testing.T) {
	require.NoError(t, ValidateImageStudioModel("gpt-image-1", []string{"gpt-image-1", "gpt-image-2"}))
	require.Error(t, ValidateImageStudioModel("", []string{"gpt-image-1"}))
	require.Error(t, ValidateImageStudioModel("unknown-image-model", []string{"gpt-image-1"}))
}

func TestNormalizeImageStudioPrompt(t *testing.T) {
	prompt, err := NormalizeImageStudioPrompt("  a neon blue API portal  ")
	require.NoError(t, err)
	require.Equal(t, "a neon blue API portal", prompt)

	_, err = NormalizeImageStudioPrompt("   ")
	require.Error(t, err)
}
