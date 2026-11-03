package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

var ErrCategoryDefaultManagedOnly = infraerrors.Conflict("CATEGORY_DEFAULT_MANAGED_ONLY", "category default groups must be managed through their publication API")
