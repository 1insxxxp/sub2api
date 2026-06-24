package service

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const imageStudioMaxNormalizedDimension = 8192

func normalizeImageStudioOutputImage(data []byte, mimeType string, outputFormat string, background string, aspectRatio string, size string) ([]byte, string, string, error) {
	normalizedFormat := NormalizeImageStudioOutputFormat(outputFormat)
	if normalizedFormat == "webp" {
		return data, mimeType, outputFormat, nil
	}
	targetWidth, targetHeight, ok := imageStudioTargetCanvasDimensions(aspectRatio, size)
	if !ok || targetWidth <= 0 || targetHeight <= 0 || len(data) == 0 {
		return data, mimeType, outputFormat, nil
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return data, mimeType, outputFormat, nil
	}
	if cfg.Width == targetWidth && cfg.Height == targetHeight {
		return data, mimeType, outputFormat, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mimeType, outputFormat, nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	if NormalizeImageStudioBackground(background) != "transparent" || normalizedFormat == "jpeg" {
		stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	}
	sourceRect := imageStudioCenterCropRect(src.Bounds(), targetWidth, targetHeight)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sourceRect, stddraw.Over, nil)

	var buf bytes.Buffer
	switch normalizedFormat {
	case "jpeg":
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 92}); err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), "image/jpeg", "jpeg", nil
	default:
		if err := png.Encode(&buf, dst); err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), "image/png", "png", nil
	}
}

func imageStudioTargetCanvasDimensions(aspectRatio string, size string) (int, int, bool) {
	if width, height, ok := parseImageStudioSize(size); ok {
		return width, height, true
	}
	ratioWidth, ratioHeight, ok := parseImageStudioRatio(aspectRatio)
	if !ok {
		return 0, 0, false
	}
	const base = 1024
	if ratioWidth >= ratioHeight {
		return base, int(math.Round(float64(base) * ratioHeight / ratioWidth)), true
	}
	return int(math.Round(float64(base) * ratioWidth / ratioHeight)), base, true
}

func parseImageStudioSize(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 || width > imageStudioMaxNormalizedDimension || height > imageStudioMaxNormalizedDimension {
		return 0, 0, false
	}
	return width, height, true
}

func parseImageStudioRatio(aspectRatio string) (float64, float64, bool) {
	parts := strings.Split(strings.TrimSpace(aspectRatio), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 || !isFiniteImageStudioRatio(width) || !isFiniteImageStudioRatio(height) {
		return 0, 0, false
	}
	return width, height, true
}

func isFiniteImageStudioRatio(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}

func imageStudioCenterCropRect(bounds image.Rectangle, targetWidth int, targetHeight int) image.Rectangle {
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 || targetWidth <= 0 || targetHeight <= 0 {
		return bounds
	}
	sourceAspect := float64(srcWidth) / float64(srcHeight)
	targetAspect := float64(targetWidth) / float64(targetHeight)
	if math.Abs(sourceAspect-targetAspect) < 0.0001 {
		return bounds
	}
	if sourceAspect > targetAspect {
		cropWidth := int(math.Round(float64(srcHeight) * targetAspect))
		if cropWidth < 1 {
			cropWidth = 1
		}
		left := bounds.Min.X + (srcWidth-cropWidth)/2
		return image.Rect(left, bounds.Min.Y, left+cropWidth, bounds.Max.Y)
	}
	cropHeight := int(math.Round(float64(srcWidth) / targetAspect))
	if cropHeight < 1 {
		cropHeight = 1
	}
	top := bounds.Min.Y + (srcHeight-cropHeight)/2
	return image.Rect(bounds.Min.X, top, bounds.Max.X, top+cropHeight)
}
