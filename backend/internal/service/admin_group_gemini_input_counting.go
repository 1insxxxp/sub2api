package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

func validateGroupGeminiInputCounting(platform string, enabled bool) error {
	if enabled && platform != PlatformGemini && platform != PlatformComposite {
		return infraerrors.BadRequest("INVALID_GEMINI_INPUT_COUNTING_CONFIG", "Gemini input counting can only be enabled for Gemini or composite groups")
	}
	return nil
}
