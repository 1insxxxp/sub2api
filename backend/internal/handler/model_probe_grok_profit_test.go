//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelProbeGrokMediaKeepsTokenProfitGateSuppressed(t *testing.T) {
	for _, model := range []string{"grok-imagine-image", "grok-imagine-video", "grok-video-1.5"} {
		t.Run(model, func(t *testing.T) {
			group := &service.Group{ID: 7, Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true,
				AllowImageGeneration: true, RateMultiplier: 1, ProfitControlEnabled: true, ProfitMinMargin: 0.5}
			account := probeGatewayAccount(71, 7, service.PlatformGrok, model, model)
			rate := 1.0 // Above the text threshold; dedicated media has separate pricing.
			account.RateMultiplier = &rate
			repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
			slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
			h := probeGatewayFixture(repo, nil, nil, slots)
			selection, err := h.SelectForProbe(context.Background(), group, model)
			require.NoError(t, err, "the native Grok media endpoint does not apply the token profit gate")
			require.Equal(t, account.ID, selection.AccountID)
			selection.Release()
			slots.mu.Lock()
			releases := slots.accountReleaseCalls
			slots.mu.Unlock()
			require.Equal(t, 1, releases)
		})
	}
}
