package domain

import "errors"

const (
	KeyDisplayCategoryAutomatic = ""
	KeyDisplayCategoryAnthropic = "anthropic"
	KeyDisplayCategoryOpenAI    = "openai"
	KeyDisplayCategoryDomestic  = "domestic"
	KeyDisplayCategoryOther     = "other"
)

// ValidateGroupKeyDisplayCategory validates display metadata independently of platform.
func ValidateGroupKeyDisplayCategory(category string) error {
	switch category {
	case KeyDisplayCategoryAutomatic, KeyDisplayCategoryAnthropic, KeyDisplayCategoryOpenAI, KeyDisplayCategoryDomestic, KeyDisplayCategoryOther:
		return nil
	default:
		return errors.New("key_display_category must be empty, anthropic, openai, domestic, or other")
	}
}
