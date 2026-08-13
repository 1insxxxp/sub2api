package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceAdmin_IncludesDefaultReasoningEffort(t *testing.T) {
	t.Parallel()

	group := &service.Group{
		ID:                     42,
		Name:                   "ccmax",
		DefaultReasoningEffort: "high",
	}

	got := GroupFromServiceAdmin(group)

	require.NotNil(t, got)
	require.Equal(t, "high", got.DefaultReasoningEffort)
}

func TestGroupFromServiceAdmin_ExplicitlyIncludesSystemCustomRoutingFlag(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		enabled := enabled
		t.Run(map[bool]string{false: "ordinary", true: "system_custom"}[enabled], func(t *testing.T) {
			t.Parallel()

			got := GroupFromServiceAdmin(&service.Group{
				ID:                         42,
				Name:                       "tavern monthly card",
				SystemCustomRoutingEnabled: enabled,
			})

			require.NotNil(t, got)
			require.Equal(t, enabled, got.SystemCustomRoutingEnabled)

			raw, err := json.Marshal(got)
			require.NoError(t, err)
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &body))
			require.JSONEq(t, map[bool]string{false: "false", true: "true"}[enabled], string(body["system_custom_routing_enabled"]))
		})
	}
}
