//go:build unit

package service

import "testing"

func TestModelProbeDedicatedEndpointsAreExplicitlyUnsupported(t *testing.T) {
	for _, model := range []string{"text-embedding-3-small", "models/gemini-embedding-001", "omni-moderation-latest", "gpt-4o-mini-tts", "gpt-realtime-1", "gpt-audio", "whisper-1", "grok-voice-latest", "xai/grok-tts", "grok-web-search", "veo-3.1-generate-preview", "imagen-4.0-generate-001", "dall-e-3", "nai-diffusion-4-5-curated", "sora-2"} {
		if modelProbeSupportsDefaultMode(model) {
			t.Errorf("dedicated endpoint %q must not be tested as text", model)
		}
	}
	for _, model := range []string{"gemini-2.5-flash", "gemini-2.5-flash-image", "gpt-image-1", "grok-imagine-video", "grok-imagine-image", "claude-sonnet-4-6", "custom-model", "gpt-5.6-sol"} {
		if !modelProbeSupportsDefaultMode(model) {
			t.Errorf("native default probe %q should remain available", model)
		}
	}
}
