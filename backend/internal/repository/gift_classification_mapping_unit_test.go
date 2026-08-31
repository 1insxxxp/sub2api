//go:build unit

package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestUserEntityToService_MapsGiftBalancesForDirectAndAPIKeyLoads(t *testing.T) {
	user := &dbent.User{
		ID:                41,
		GiftBalance:       12.5,
		FrozenGiftBalance: 3.25,
	}

	direct := userEntityToService(user)
	require.Equal(t, 12.5, direct.GiftBalance)
	require.Equal(t, 3.25, direct.FrozenGiftBalance)

	apiKey := apiKeyEntityToService(&dbent.APIKey{
		Edges: dbent.APIKeyEdges{User: user},
	})
	require.NotNil(t, apiKey.User)
	require.Equal(t, 12.5, apiKey.User.GiftBalance)
	require.Equal(t, 3.25, apiKey.User.FrozenGiftBalance)
}
