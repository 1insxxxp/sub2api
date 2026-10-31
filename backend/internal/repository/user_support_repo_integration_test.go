//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func supportUser(t *testing.T, role string, balance float64) *service.User {
	t.Helper()
	u := mustCreateUser(t, testEntClient(t), &service.User{Email: "support-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Role: role, Balance: balance, Status: service.StatusActive})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM support_trial_grants WHERE user_id=$1 OR operator_id=$1", u.ID)
		_, _ = integrationDB.Exec("DELETE FROM user_support_access WHERE user_id=$1 OR updated_by=$1", u.ID)
	})
	return u
}

func TestUserSupportGrantConcurrentAndDurable(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	secondAdmin := supportUser(t, service.RoleAdmin, 0)
	target := supportUser(t, service.RoleUser, 7)
	r := NewUserSupportRepository(integrationDB)
	const workers = 8
	var wg sync.WaitGroup
	results := make(chan *service.UserSupportGrantResult, workers)
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		actor := admin.ID
		if i%2 == 1 {
			actor = secondAdmin.ID
		}
		wg.Add(1)
		go func(actor int64) {
			defer wg.Done()
			got, err := r.LookupAndGrant(ctx, actor, target.Email)
			results <- got
			errors <- err
		}(actor)
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var grants int
	for got := range results {
		require.NotNil(t, got)
		if got.Trial.Status == "granted" {
			grants++
		}
	}
	require.Equal(t, 1, grants)
	var balance, gift, recharged float64
	require.NoError(t, integrationDB.QueryRow("SELECT balance,gift_balance,total_recharged FROM users WHERE id=$1", target.ID).Scan(&balance, &gift, &recharged))
	require.Equal(t, float64(22), balance)
	require.Equal(t, float64(15), gift)
	require.Zero(t, recharged)
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM redeem_codes WHERE used_by=$1 AND source='support_trial' AND threshold_exempt=true", target.ID).Scan(&count))
	require.Equal(t, 1, count)
	_, err := integrationDB.Exec("DELETE FROM redeem_codes WHERE used_by=$1 AND source='support_trial'", target.ID)
	require.NoError(t, err)
	got, err := r.LookupAndGrant(ctx, admin.ID, strconv.FormatInt(target.ID, 10))
	require.NoError(t, err)
	require.Equal(t, "already_received", got.Trial.Status)
	require.Equal(t, float64(22), got.User.Balance)
}

func TestUserSupportOrdinaryStaffAndGiftWallet(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	staff := supportUser(t, service.RoleUser, 0)
	r := NewUserSupportRepository(integrationDB)
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, true))
	allowed, err := r.CanAccess(ctx, staff.ID)
	require.NoError(t, err)
	require.True(t, allowed)
	_, err = r.GetAccess(ctx, staff.ID, admin.ID)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	mainAccess, err := r.GetAccess(ctx, admin.ID, admin.ID)
	require.NoError(t, err)
	require.True(t, mainAccess)
	require.ErrorIs(t, r.SetAccess(ctx, admin.ID, admin.ID, false), service.ErrUserSupportForbidden)
	for _, initial := range []float64{-20, -5, 10} {
		u := supportUser(t, service.RoleUser, initial)
		got, err := r.LookupAndGrant(ctx, staff.ID, u.Email)
		require.NoError(t, err)
		require.Equal(t, initial+15, got.User.Balance)
		var gift float64
		require.NoError(t, integrationDB.QueryRow("SELECT gift_balance FROM users WHERE id=$1", u.ID).Scan(&gift))
		wantGift := initial + 15
		if wantGift < 0 {
			wantGift = 0
		}
		if wantGift > 15 {
			wantGift = 15
		}
		require.Equal(t, wantGift, gift)
	}
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, false))
	allowed, err = r.CanAccess(ctx, staff.ID)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestUserSupportLegacyRecognitionAndOrdinaryRecharge(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	r := NewUserSupportRepository(integrationDB)
	for _, kind := range []string{"admin_balance", "balance"} {
		t.Run(kind, func(t *testing.T) {
			u := supportUser(t, service.RoleUser, 19)
			_, err := integrationDB.Exec(`INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,created_at) VALUES($1,$2,15,'used',$3,NOW(),NOW())`, uuid.NewString()[:30], kind, u.ID)
			require.NoError(t, err)
			got, err := r.LookupAndGrant(ctx, admin.ID, u.Email)
			require.NoError(t, err)
			if kind == "admin_balance" {
				require.Equal(t, "already_received", got.Trial.Status)
				require.True(t, got.Trial.Legacy)
				require.Equal(t, float64(19), got.User.Balance)
			} else {
				require.Equal(t, "granted", got.Trial.Status)
				require.Equal(t, float64(34), got.User.Balance)
			}
		})
	}
}

func TestUserSupportFreshPermissionAndRollback(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	staff := supportUser(t, service.RoleSubAdmin, 0)
	u := supportUser(t, service.RoleUser, 4)
	r := NewUserSupportRepository(integrationDB)
	_, err := r.LookupAndGrant(ctx, staff.ID, u.Email)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, true))
	ok, err := r.CanAccess(ctx, staff.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, false))
	_, err = r.LookupAndGrant(ctx, staff.ID, u.Email)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	require.ErrorIs(t, r.SetAccess(ctx, staff.ID, staff.ID, true), service.ErrUserSupportForbidden)
	// A ledger failure after wallet credit must roll back both the credit and claim.
	name := "support_fail_" + strconv.FormatInt(u.ID, 10)
	_, err = integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger AS $$ BEGIN IF NEW.used_by=%d AND NEW.source='support_trial' THEN RAISE EXCEPTION 'test ledger failure'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql; CREATE TRIGGER %s BEFORE INSERT ON redeem_codes FOR EACH ROW EXECUTE FUNCTION %s()`, name, u.ID, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DROP TRIGGER IF EXISTS " + name + " ON redeem_codes; DROP FUNCTION IF EXISTS " + name + "()")
	})
	_, err = r.LookupAndGrant(ctx, admin.ID, u.Email)
	require.Error(t, err)
	var balance float64
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", u.ID).Scan(&balance))
	require.Equal(t, float64(4), balance)
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM support_trial_grants WHERE user_id=$1", u.ID).Scan(&count))
	require.Zero(t, count)
}

func TestUserSupportRecordsAreBoundedOwnedAndSafe(t *testing.T) {
	ctx := context.Background()
	u := supportUser(t, service.RoleUser, 0)
	other := supportUser(t, service.RoleUser, 0)
	r := NewUserSupportRepository(integrationDB)
	key := mustCreateApiKey(t, testEntClient(t), &service.APIKey{UserID: u.ID, Key: "sk-" + uuid.NewString(), Name: "secret.example"})
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "http://secret.example", Type: service.AccountTypeAPIKey})
	stamp := time.Now().UTC()
	for i := 0; i < 12; i++ {
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,output_tokens,actual_cost,created_at) VALUES($1,$2,$3,$4,'mapped-secret.example','按量vet/gemini-4-argon',11,12,0.3,$5)`, u.ID, key.ID, account.ID, uuid.NewString(), stamp)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO ops_error_logs(user_id,created_at,error_phase,error_type,status_code,model,requested_model,error_message,is_count_tokens) VALUES($1,$2,'upstream','api_error',502,'mapped-secret.example','gemini-4-argon','http://secret.example sk-secret',false)`, u.ID, stamp)
		require.NoError(t, err)
	}
	_, err := integrationDB.Exec(`INSERT INTO ops_error_logs(user_id,error_phase,error_type,status_code,model,error_message) VALUES($1,'upstream','api_error',502,'secret.example','sk-other')`, other.ID)
	require.NoError(t, err)
	usage, err := r.ListUsage(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, usage, 10)
	errors, err := r.ListErrors(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, errors, 10)
	for i := 1; i < 10; i++ {
		require.Greater(t, usage[i-1].ID, usage[i].ID)
		require.Greater(t, errors[i-1].ID, errors[i].ID)
	}
	encoded, err := json.Marshal(struct {
		Usage  any
		Errors any
	}{usage, errors})
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "upstream")
	require.NotContains(t, string(encoded), "account")
	require.Equal(t, "gemini-4-argon", usage[0].Model)
	require.Equal(t, "service_unavailable", errors[0].Category)
	_, err = r.ListErrors(ctx, 0)
	require.ErrorIs(t, err, service.ErrUserSupportUserNotFound)
}

func TestUserSupportAmbiguousLegacyEmailNeverCreditsEitherUser(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	first := supportUser(t, service.RoleUser, 3)
	var secondID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users(email,password_hash,role,status,balance) VALUES($1,'hash','user','active',7) RETURNING id`, " "+strings.ToUpper(first.Email)+" ").Scan(&secondID))
	r := NewUserSupportRepository(integrationDB)
	_, err := r.LookupAndGrant(ctx, admin.ID, first.Email)
	require.ErrorIs(t, err, service.ErrUserSupportInvalidAccount)
	var firstBalance, secondBalance float64
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", first.ID).Scan(&firstBalance))
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", secondID).Scan(&secondBalance))
	require.Equal(t, float64(3), firstBalance)
	require.Equal(t, float64(7), secondBalance)
	var claims int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM support_trial_grants WHERE user_id IN ($1,$2)", first.ID, secondID).Scan(&claims))
	require.Zero(t, claims)
	// An explicit ID still selects exactly one account after the ambiguity.
	got, err := r.LookupAndGrant(ctx, admin.ID, strconv.FormatInt(secondID, 10))
	require.NoError(t, err)
	require.Equal(t, secondID, got.User.ID)
	t.Cleanup(func() { _, _ = integrationDB.Exec("DELETE FROM support_trial_grants WHERE user_id=$1", secondID) })
}
