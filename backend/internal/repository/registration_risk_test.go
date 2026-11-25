package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	entuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestConcurrentRegistrationRiskRollsBackRewardsAndUser(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	_, err := client.ExecContext(ctx, `CREATE TABLE account_risk_events(user_id BIGINT,event_type TEXT,browser_hash TEXT,ip_address TEXT,user_agent TEXT,created_at TIMESTAMP);
 CREATE TABLE account_registration_cooldowns(ip_address TEXT PRIMARY KEY,expires_at TIMESTAMP);
 CREATE TABLE account_registration_captcha(ip_address TEXT,browser_hash TEXT,scope TEXT,expires_at TIMESTAMP, PRIMARY KEY(ip_address,browser_hash,scope));`)
	require.NoError(t, err)
	require.NoError(t, NewSettingRepository(client).Set(ctx, "account_risk_registration_mode", "enforce"))
	require.NoError(t, NewSettingRepository(client).Set(ctx, "account_risk_challenge_count", "1"))
	verifyConcurrentRegistration(t, repo, client, "192.0.2.10", "")
}

func verifyConcurrentRegistration(t *testing.T, repo service.UserRepository, client *dbent.Client, ip, prefix string) {
	t.Helper()
	ctx := context.Background()
	// Release all callers together so the check uses committed registrations,
	// including those completed by another repository transaction.
	const workers = 6
	var wg sync.WaitGroup
	start := make(chan struct{})
	outcomes := make(chan error, workers)
	for n := range workers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			ctx := service.WithRegistrationRiskSource(ctx, &service.RegistrationRiskSource{IP: ip, BrowserHash: fmt.Sprintf("%064x", n+1)})
			u := &service.User{Email: fmt.Sprintf("%suser%d@example.org", prefix, n), PasswordHash: "test", Role: service.RoleUser, Status: service.StatusActive, Balance: 10}
			outcomes <- repo.CreateWithEmailAliasGuard(ctx, u)
		}(n)
	}
	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("registrations did not finish")
	}
	close(outcomes)
	successful, challenged := 0, 0
	for err := range outcomes {
		switch {
		case err == nil:
			successful++
		case errors.Is(err, service.ErrRegistrationCaptchaRequired):
			challenged++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successful)
	require.Equal(t, workers-1, challenged)
	users, err := client.User.Query().Where(entuser.RegistrationIPEQ(ip)).All(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, 10.0, users[0].Balance)
}
