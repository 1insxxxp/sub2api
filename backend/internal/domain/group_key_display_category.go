package domain

import "errors"

const (
	KeyDisplayCategoryAutomatic = ""
	KeyDisplayCategoryAnthropic = "anthropic"
	KeyDisplayCategoryOpenAI    = "openai"
	KeyDisplayCategoryGemini    = "gemini"
	KeyDisplayCategoryGrok      = "grok"
	KeyDisplayCategoryDomestic  = "domestic"
	KeyDisplayCategoryImage     = "image"
	KeyDisplayCategoryOther     = "other"
)

// ValidateGroupKeyDisplayCategory validates display metadata independently of platform.
func ValidateGroupKeyDisplayCategory(category string) error {
	switch category {
	case KeyDisplayCategoryAutomatic, KeyDisplayCategoryAnthropic, KeyDisplayCategoryOpenAI, KeyDisplayCategoryGemini, KeyDisplayCategoryGrok, KeyDisplayCategoryDomestic, KeyDisplayCategoryImage, KeyDisplayCategoryOther:
		return nil
	default:
		return errors.New("key_display_category must be empty, anthropic, openai, gemini, grok, domestic, image, or other")
	}
}
