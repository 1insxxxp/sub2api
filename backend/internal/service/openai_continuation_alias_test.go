//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type continuationAliasTestCache struct {
	stubGatewayCache
	mu      sync.Mutex
	aliases map[string]OpenAIContinuationAlias
}

func (c *continuationAliasTestCache) SetOpenAIContinuationAlias(_ context.Context, groupID, userID int64, public string, b OpenAIContinuationAlias, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.aliases == nil {
		c.aliases = map[string]OpenAIContinuationAlias{}
	}
	c.aliases[openAIContinuationAliasKey(groupID, userID, public)] = b
	return nil
}
func (c *continuationAliasTestCache) GetOpenAIContinuationAlias(_ context.Context, groupID, userID int64, public string) (OpenAIContinuationAlias, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.aliases[openAIContinuationAliasKey(groupID, userID, public)]
	return v, ok, nil
}

func TestOpenAIContinuationAliasPersistsAndIsTenantScoped(t *testing.T) {
	ctx := context.Background()
	cache := &continuationAliasTestCache{}
	svc := &OpenAIGatewayService{cache: cache}
	binding := OpenAIContinuationAlias{ResponseID: "resp_final", AccountID: 73}
	require.NoError(t, svc.bindContinuationAlias(ctx, 7, 11, "resp_public", binding))
	otherInstance := &OpenAIGatewayService{cache: cache}
	actual, err := otherInstance.ResolveOpenAIContinuationAlias(ctx, 7, 11, "resp_public")
	require.NoError(t, err)
	require.Equal(t, "resp_final", actual)
	for _, scope := range [][2]int64{{8, 11}, {7, 12}} {
		actual, err = otherInstance.ResolveOpenAIContinuationAlias(ctx, scope[0], scope[1], "resp_public")
		require.NoError(t, err)
		require.Equal(t, "resp_public", actual)
	}
}

func TestOpenAIContinuationAliasLocalStoreExpires(t *testing.T) {
	svc := &OpenAIGatewayService{}
	store := svc.getOpenAIWSStateStore().(*defaultOpenAIWSStateStore)
	require.NoError(t, store.bindContinuationAlias(context.Background(), 7, 11, "resp_public", OpenAIContinuationAlias{ResponseID: "resp_final", AccountID: 73}, time.Millisecond))
	time.Sleep(4 * time.Millisecond)
	actual, err := svc.ResolveOpenAIContinuationAlias(context.Background(), 7, 11, "resp_public")
	require.NoError(t, err)
	require.Equal(t, "resp_public", actual)
}
