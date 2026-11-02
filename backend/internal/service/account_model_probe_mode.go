package service

import "strings"

// The account dialog exposes these dedicated protocols as separate modes (or
// does not support them). A default text probe cannot establish their health.
// Classify the final mapped ID, so a public alias does not conceal its endpoint.
func modelProbeSupportsDefaultMode(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndexByte(model, '/'); slash >= 0 {
		model = model[slash+1:]
	}
	for _, prefix := range []string{"text-embedding-", "gemini-embedding-", "embedding-", "text-moderation-", "omni-moderation-", "tts-", "whisper-", "grok-voice", "grok-tts", "grok-stt", "grok-web-search", "dall-e-", "imagen-", "veo-", "nai-diffusion-", "sora-"} {
		if strings.HasPrefix(model, prefix) {
			return false
		}
	}
	for _, family := range []string{"realtime", "-transcribe", "-tts", "-stt", "-audio"} {
		if strings.Contains(model, family) {
			return false
		}
	}
	return true
}
