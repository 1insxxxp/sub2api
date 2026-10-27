//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserErrorRequestsExcludeCanceledWithOwnership(t *testing.T) {
	repo := &stubOpsRepoForUserErr{}
	svc := &OpsService{opsRepo: repo}
	filter := &OpsErrorLogFilter{IncludeCanceledRequests: true, CanceledOnly: true}
	_, err := svc.ListUserErrorRequests(context.Background(), 42, filter)
	require.NoError(t, err)
	encoded, err := json.Marshal(repo.gotFilter)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Equal(t, false, fields["IncludeCanceledRequests"])
	require.EqualValues(t, 42, *repo.gotFilter.UserID)
	require.False(t, repo.gotFilter.IncludeRecoveredUpstream)
	require.False(t, repo.gotFilter.CanceledOnly)
	require.True(t, repo.gotFilter.ExcludeCanceledRequests)
	require.True(t, filter.IncludeCanceledRequests, "caller filter remains immutable")
}

func TestUserCanceledRequestKeepsWireStatusAndRedactsEarlierProviderFailure(t *testing.T) {
	var source OpsErrorLog
	require.NoError(t, json.Unmarshal([]byte(`{"id":18,"phase":"upstream","type":"upstream_error","status_code":503,"downstream_status_code":200,"client_canceled":true,"message":"Recovered upstream error 503: internal route provider.invalid"}`), &source))
	result := ToUserErrorRequest(&source)
	require.Equal(t, "client_cancel", result.Category)
	require.Equal(t, 200, result.StatusCode, "HTTP headers already sent must not be relabeled as an invented error status")
	require.NotContains(t, result.Message, "provider.invalid")
	require.Contains(t, result.Message, "cancel")
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"client_canceled":true`)
}
