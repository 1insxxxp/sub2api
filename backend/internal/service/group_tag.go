package service

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var groupTagColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func ValidateGroupTag(tag string) error {
	if !utf8.ValidString(tag) || utf8.RuneCountInString(strings.TrimSpace(tag)) > 20 {
		return infraerrors.BadRequest("INVALID_GROUP_TAG", "group tag must be valid UTF-8 with at most 20 Unicode codepoints")
	}
	for _, r := range tag {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return infraerrors.BadRequest("INVALID_GROUP_TAG", "group tag must not contain control characters or line breaks")
		}
	}
	return nil
}

func normalizeGroupTag(tag, color string) (string, string, error) {
	// Validate before trimming so trailing newlines cannot bypass validation.
	if err := ValidateGroupTag(tag); err != nil {
		return "", "", err
	}
	tag, color = strings.TrimSpace(tag), strings.TrimSpace(color)
	if color != "" && !groupTagColorPattern.MatchString(color) {
		return "", "", infraerrors.BadRequest("INVALID_GROUP_TAG_COLOR", "group tag color must be #RRGGBB or empty")
	}
	if tag == "" {
		color = ""
	}
	return tag, color, nil
}

func updateGroupTag(group *Group, tag, color *string) error {
	if tag == nil && color == nil {
		return nil
	}
	nextTag, nextColor := group.Tag, group.TagColor
	if tag != nil {
		nextTag = *tag
	}
	if color != nil {
		nextColor = *color
	}
	nextTag, nextColor, err := normalizeGroupTag(nextTag, nextColor)
	if err != nil {
		return err
	}
	group.Tag, group.TagColor = nextTag, nextColor
	return nil
}
