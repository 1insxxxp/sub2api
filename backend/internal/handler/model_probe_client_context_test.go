//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type probeClientGroups struct {
	service.GroupRepository
	groups map[int64]*service.Group
	read   []int64
}

func (r *probeClientGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	r.read = append(r.read, id)
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

func TestModelProbeClaudeNativeClientRetainsSelectedGroup(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "no fallback", true: "configured fallback"}[fallback], func(t *testing.T) {
			group := &service.Group{ID: 7, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true, ClaudeCodeOnly: true}
			other := &service.Group{ID: 8, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true}
			if fallback {
				group.FallbackGroupID = &other.ID
			}
			model := "claude-probe-public"
			account := probeGatewayAccount(71, 7, service.PlatformAnthropic, model, "claude-native")
			outside := probeGatewayAccount(81, 8, service.PlatformAnthropic, model, "different-native")
			repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}, 8: {outside}}}
			groups := &probeClientGroups{groups: map[int64]*service.Group{7: group, 8: other}}
			slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
			concurrency := service.NewConcurrencyService(slots)
			cfg := &config.Config{RunMode: config.RunModeStandard}
			gateway := service.NewGatewayService(repo, groups, nil, nil, nil, nil, nil, nil, cfg,
				nil, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h := &GatewayHandler{gatewayService: gateway, concurrencyHelper: NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0)}
			selection, err := h.SelectForProbe(context.Background(), group, model)
			require.NoError(t, err, "the native account test uses Claude Code client characteristics")
			require.Equal(t, account.ID, selection.AccountID, "the probe must test the selected group rather than its non-Claude fallback")
			for _, id := range repo.queries {
				require.Equal(t, group.ID, id)
			}
			for _, id := range groups.read {
				require.Equal(t, group.ID, id, "only the selected group may be refreshed")
			}
			selection.Release()
		})
	}
}
