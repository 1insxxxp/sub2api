package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelProbeExecutionSpy struct {
	modelProbeGatewayStub
	executions int
	groupID    int64
	model      string
	ctx        context.Context
}

func (s *modelProbeExecutionSpy) ExecuteForProbe(ctx context.Context, group *Group, model string) (ModelProbeExecution, error) {
	s.ctx = ctx
	s.executions++
	s.groupID, s.model = group.ID, model
	return ModelProbeExecution{Status: "success"}, nil
}

func TestModelProbeExecutionBindsActorAccountExclusions(t *testing.T) {
	svc, _, _, _, _ := modelProbeFixture()
	svc.users = &modelProbeUserStub{user: &User{ID: 4, Status: StatusActive, GroupAccountExclusions: GroupAccountExclusions{7: {8}}}}
	gateway := &modelProbeExecutionSpy{modelProbeGatewayStub: modelProbeGatewayStub{models: []string{"public-model"}}}
	svc.gateway = gateway
	_, err := svc.Run(context.Background(), 4, 7, "public-model")
	require.NoError(t, err)
	groupID := int64(7)
	require.False(t, IsGroupAccountAllowedForUser(gateway.ctx, &groupID, 8))
	require.True(t, IsGroupAccountAllowedForUser(gateway.ctx, &groupID, 9))
}

func TestModelProbeExecutionRejectsMissingOrInactiveActor(t *testing.T) {
	for _, user := range []*User{nil, {ID: 5, Status: StatusActive}, {ID: 4, Status: StatusDisabled}} {
		svc, _, _, gateway, _ := modelProbeFixture()
		svc.users = &modelProbeUserStub{user: user}
		_, err := svc.Run(context.Background(), 4, 7, "public-model")
		require.Error(t, err)
		require.Zero(t, gateway.selected)
	}
}

func TestModelProbeUsesGatewayExecution(t *testing.T) {
	svc, _, _, _, legacy := modelProbeFixture()
	legacy.result = ModelProbeExecution{Status: "failed"}
	gateway := &modelProbeExecutionSpy{modelProbeGatewayStub: modelProbeGatewayStub{models: []string{"public-model"}}}
	svc.gateway = gateway
	result, err := svc.Run(context.Background(), 4, 7, "public-model")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status, "probe must use the complete gateway execution, including account failover")
	require.Equal(t, 1, gateway.executions)
	require.Equal(t, int64(7), gateway.groupID)
	require.Equal(t, "public-model", gateway.model)
	require.Zero(t, gateway.selected, "service must not terminate at a single native account test")
	require.Zero(t, legacy.account)
}
