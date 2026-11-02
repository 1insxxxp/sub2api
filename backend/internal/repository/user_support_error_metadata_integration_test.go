//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserSupportErrorMetadataIsBoundedAndPublic(t *testing.T) {
	u := supportUser(t, service.RoleUser, 0)
	group := mustCreateGroup(t, testEntClient(t), &service.Group{Name: "客户分组-" + uuid.NewString()})
	cases := []struct {
		name, phase, kind, wantKind string
		status, wantStatus          any
	}{
		{"stream error", "request", "cyber_policy", "request", 200, float64(200)},
		{"missing status", "request", "cyber_policy", "request", nil, nil},
		{"invalid high status", "request", "cyber_policy", "request", 700, nil},
		{"invalid low status", "request", "cyber_policy", "request", 99, nil},
		{"valid low status", "request", "cyber_policy", "request", 100, float64(100)},
		{"valid high status", "network", "network_error", "network", 599, float64(599)},
		{"untrusted phase", "https://secret.example/sk-secret", "sk-secret", "unknown", 503, float64(503)},
	}
	stamp := time.Now().UTC()
	expected := make(map[int64]int, len(cases))
	for i, tc := range cases {
		var id int64
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO ops_error_logs
		(user_id,group_id,created_at,error_phase,error_type,status_code,upstream_status_code,requested_model,error_message)
		VALUES($1,$2,$3,$4,$5,$6,429,'gemini-4-argon','http://secret.example sk-secret') RETURNING id`,
			u.ID, group.ID, stamp.Add(time.Duration(i)*time.Second), tc.phase, tc.kind, tc.status).Scan(&id), tc.name)
		expected[id] = i
	}
	rows, err := NewUserSupportRepository(integrationDB).ListErrors(context.Background(), u.ID)
	require.NoError(t, err)
	require.Len(t, rows, len(cases))
	encoded, err := json.Marshal(rows)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	var publicRows []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &publicRows))
	for i, row := range rows {
		index, known := expected[row.ID]
		require.True(t, known, "only this user's fixture records may be returned")
		tc := cases[index]
		require.Equal(t, group.Name, publicRows[i]["group_name"], tc.name)
		require.Equal(t, tc.wantStatus, publicRows[i]["status_code"], tc.name)
		require.Equal(t, tc.wantKind, publicRows[i]["error_kind"], tc.name)
		if i > 0 {
			require.Greater(t, rows[i-1].ID, row.ID)
		}
	}
}

func TestUserSupportErrorsKeepRequestScopedOriginalReason(t *testing.T) {
	u := supportUser(t, service.RoleUser, 0)
	_, err := integrationDB.Exec(`INSERT INTO ops_error_logs
	(user_id,error_phase,error_type,status_code,requested_model,error_message,upstream_error_message)
	VALUES($1,'request','content_filter',400,'gemini-3.8-flash',$2,$3)`,
		u.ID, "Gemini rejected prompt: PROHIBITED_CONTENT", "Provider temporarily unavailable")
	require.NoError(t, err)
	rows, err := NewUserSupportRepository(integrationDB).ListErrors(context.Background(), u.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Contains(t, rows[0].OriginalMessage, "PROHIBITED_CONTENT")
	require.NotContains(t, rows[0].OriginalMessage, "temporarily unavailable", "a provider diagnostic must not replace this request's rejection reason")
}
