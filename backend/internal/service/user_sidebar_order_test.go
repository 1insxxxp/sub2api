package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUserSidebarOrder(t *testing.T) {
	got, err := NormalizeUserSidebarOrder([]string{" /model-plaza ", "/keys", "/keys", "", "/custom/help"})
	require.NoError(t, err)
	require.Equal(t, []string{"/model-plaza", "/keys", "/custom/help"}, got)
}

func TestNormalizeUserSidebarOrderRejectsUnsafeIdentifiers(t *testing.T) {
	_, err := NormalizeUserSidebarOrder([]string{"model-plaza"})
	require.Error(t, err)
}

func TestParseUserSidebarOrderFallsBackToEmpty(t *testing.T) {
	require.Empty(t, ParseUserSidebarOrder("not-json"))
	require.Equal(t, []string{"/keys"}, ParseUserSidebarOrder(`["/keys"]`))
}
