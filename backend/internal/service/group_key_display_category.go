package service

import (
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func validateGroupKeyDisplayCategory(category string) error {
	if err := domain.ValidateGroupKeyDisplayCategory(category); err != nil {
		return infraerrors.BadRequest("INVALID_GROUP_KEY_DISPLAY_CATEGORY", err.Error())
	}
	return nil
}
