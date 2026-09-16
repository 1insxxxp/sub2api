package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

func ValidateGroupTag(tag string) error {
	switch tag {
	case "", "chat", "image", "airp":
		return nil
	default:
		return infraerrors.BadRequest("INVALID_GROUP_TAG", "group tag must be chat, image, airp, or empty")
	}
}
