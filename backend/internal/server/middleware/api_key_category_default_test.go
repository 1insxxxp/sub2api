package middleware

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultAuthenticationDefersEntryPermissionToSourceResolver(t *testing.T) {
	entryID := int64(91)
	key := &service.APIKey{GroupID: &entryID, User: &service.User{ID: 3, RestrictPublicGroups: true}, Group: &service.Group{ID: entryID, IsCategoryDefault: true, Platform: service.PlatformComposite}}
	require.True(t, validateAPIKeyGroupAllowed(key), "entry permission is derived from the currently authorized published source")
	key.Group.IsCategoryDefault = false
	require.False(t, validateAPIKeyGroupAllowed(key), "ordinary group authorization must retain its existing direct-binding check")
}
