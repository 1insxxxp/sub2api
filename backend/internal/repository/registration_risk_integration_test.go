//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPostgresConcurrentRegistrationRisk(t *testing.T) {
	client := testEntClient(t)
	ctx := context.Background()
	settings := NewSettingRepository(client)
	keys := []string{"account_risk_registration_mode", "account_risk_challenge_count"}
	prior, err := settings.GetMultiple(ctx, keys)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, key := range keys {
			if value, ok := prior[key]; ok {
				_, err := integrationDB.Exec(`UPDATE settings SET value=$2 WHERE key=$1`, key, value)
				require.NoError(t, err)
			} else {
				_, err := integrationDB.Exec(`DELETE FROM settings WHERE key=$1`, key)
				require.NoError(t, err)
			}
		}
	})
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{keys[0]: "enforce", keys[1]: "1"}))
	suffix := time.Now().UnixNano()
	ip := fmt.Sprintf("2001:db8::%x:%x", uint16(suffix>>16), uint16(suffix))
	t.Cleanup(func() {
		_, err := integrationDB.Exec("DELETE FROM users WHERE registration_ip=$1", ip)
		require.NoError(t, err)
	})
	verifyConcurrentRegistration(t, NewUserRepository(client, integrationDB), client, ip, fmt.Sprintf("risk-%d-", suffix))
}
