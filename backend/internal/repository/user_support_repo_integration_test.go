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

func TestUserSupportSubAdminStaffAndGiftWallet(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	staff := supportUser(t, service.RoleSubAdmin, 0)
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

func TestUserSupportOrdinaryUserDeniedEvenWithExplicitAccess(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	ordinary := supportUser(t, service.RoleUser, 0)
	target := supportUser(t, service.RoleUser, 9)
	r := NewUserSupportRepository(integrationDB)
	// A legacy or erroneously inserted access row never promotes a normal user.
	_, err := integrationDB.Exec("INSERT INTO user_support_access(user_id,enabled,updated_by) VALUES($1,TRUE,$2)", ordinary.ID, admin.ID)
	require.NoError(t, err)
	allowed, err := r.CanAccess(ctx, ordinary.ID)
	require.NoError(t, err)
	require.False(t, allowed)
	_, err = r.LookupAndGrant(ctx, ordinary.ID, target.Email)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	_, err = r.GetAccess(ctx, admin.ID, ordinary.ID)
	require.ErrorIs(t, err, service.ErrUserSupportUserNotFound)
	for _, enabled := range []bool{true, false} {
		require.ErrorIs(t, r.SetAccess(ctx, admin.ID, ordinary.ID, enabled), service.ErrUserSupportUserNotFound)
	}
	var balance float64
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", target.ID).Scan(&balance))
	require.Equal(t, float64(9), balance)
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM support_trial_grants WHERE user_id=$1", target.ID).Scan(&count))
	require.Zero(t, count)
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
	group := mustCreateGroup(t, testEntClient(t), &service.Group{Name: "支持分组-" + uuid.NewString()})
	deletedGroup := mustCreateGroup(t, testEntClient(t), &service.Group{Name: "历史分组-" + uuid.NewString()})
	_, err := integrationDB.Exec("UPDATE groups SET deleted_at=NOW() WHERE id=$1", deletedGroup.ID)
	require.NoError(t, err)
	stamp := time.Now().UTC()
	for i := 0; i < 12; i++ {
		var groupID any = group.ID
		if i == 10 {
			groupID = nil
		} else if i == 11 {
			groupID = deletedGroup.ID
		}
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,output_tokens,actual_cost,created_at,group_id) VALUES($1,$2,$3,$4,'mapped-secret.example','按量vet/gemini-4-argon',11,12,0.3,$5,$6)`, u.ID, key.ID, account.ID, uuid.NewString(), stamp, groupID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO ops_error_logs(user_id,created_at,error_phase,error_type,status_code,upstream_status_code,group_id,model,requested_model,error_message,is_count_tokens) VALUES($1,$2,'upstream','api_error',502,429,$3,'mapped-secret.example','gemini-4-argon','http://secret.example sk-secret',false)`, u.ID, stamp, groupID)
		require.NoError(t, err)
	}
	otherKey := mustCreateApiKey(t, testEntClient(t), &service.APIKey{UserID: other.ID, Key: "sk-" + uuid.NewString(), Name: "other"})
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,group_id,created_at) VALUES($1,$2,$3,$4,'gemini-4-argon','gemini-4-argon',$5,$6)`, other.ID, otherKey.ID, account.ID, uuid.NewString(), group.ID, stamp.Add(time.Hour))
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO ops_error_logs(user_id,error_phase,error_type,status_code,model,error_message) VALUES($1,'upstream','api_error',502,'secret.example','sk-other')`, other.ID)
	require.NoError(t, err)
	usage, err := r.ListUsage(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, usage, 10)
	usageJSON, err := json.Marshal(usage)
	require.NoError(t, err)
	var publicUsage []map[string]any
	require.NoError(t, json.Unmarshal(usageJSON, &publicUsage))
	require.Equal(t, deletedGroup.Name, publicUsage[0]["group_name"])
	require.Equal(t, "", publicUsage[1]["group_name"])
	require.Equal(t, group.Name, publicUsage[2]["group_name"])
	require.True(t, usage[0].CreatedAt.Before(stamp.Add(time.Hour)), "another user's newer usage must not appear")
	errors, err := r.ListErrors(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, errors, 10)
	errorJSON, err := json.Marshal(errors)
	require.NoError(t, err)
	var publicErrors []map[string]any
	require.NoError(t, json.Unmarshal(errorJSON, &publicErrors))
	require.Equal(t, deletedGroup.Name, publicErrors[0]["group_name"])
	require.Equal(t, "", publicErrors[1]["group_name"])
	require.Equal(t, group.Name, publicErrors[2]["group_name"])
	require.Equal(t, float64(502), publicErrors[0]["status_code"], "retain the recorded status instead of the provider status")
	require.Equal(t, "upstream", publicErrors[0]["error_kind"])
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
	require.NotContains(t, string(encoded), "upstream_model")
	require.NotContains(t, string(encoded), "upstream_status_code")
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

func TestUserSupportReadsLeaveTrialEligibleUntilExplicitGrant(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	target := supportUser(t, service.RoleUser, 7)
	r := NewUserSupportRepository(integrationDB)
	for i := 0; i < 3; i++ {
		suggestions, err := r.Search(ctx, admin.ID, target.Email)
		require.NoError(t, err)
		require.Equal(t, []service.UserSupportSuggestion{{ID: target.ID, Email: target.Email}}, suggestions)
		got, err := r.Lookup(ctx, admin.ID, target.Email)
		require.NoError(t, err)
		require.Equal(t, float64(7), got.User.Balance)
		require.Equal(t, "eligible", got.Trial.Status)
	}
	var balance, gift float64
	var claims, codes int
	require.NoError(t, integrationDB.QueryRow(`SELECT balance,gift_balance FROM users WHERE id=$1`, target.ID).Scan(&balance, &gift))
	require.Equal(t, float64(7), balance)
	require.Zero(t, gift)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM support_trial_grants WHERE user_id=$1`, target.ID).Scan(&claims))
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM redeem_codes WHERE used_by=$1`, target.ID).Scan(&codes))
	require.Zero(t, claims)
	require.Zero(t, codes)

	granted, err := r.LookupAndGrant(ctx, admin.ID, strconv.FormatInt(target.ID, 10))
	require.NoError(t, err)
	require.Equal(t, "granted", granted.Trial.Status)
	require.Equal(t, float64(22), granted.User.Balance)
	lookedUp, err := r.Lookup(ctx, admin.ID, target.Email)
	require.NoError(t, err)
	require.Equal(t, "already_received", lookedUp.Trial.Status)
	require.Equal(t, float64(22), lookedUp.User.Balance)
	replayed, err := r.LookupAndGrant(ctx, admin.ID, strconv.FormatInt(target.ID, 10))
	require.NoError(t, err)
	require.Equal(t, "already_received", replayed.Trial.Status)
	require.Equal(t, float64(22), replayed.User.Balance)
}

func TestUserSupportReadOnlyLegacyRecognitionDoesNotMaterializeClaim(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	target := supportUser(t, service.RoleUser, 19)
	_, err := integrationDB.Exec(`INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,created_at) VALUES($1,'admin_balance',15,'used',$2,NOW(),NOW())`, uuid.NewString()[:30], target.ID)
	require.NoError(t, err)
	r := NewUserSupportRepository(integrationDB)
	got, err := r.Lookup(ctx, admin.ID, target.Email)
	require.NoError(t, err)
	require.Equal(t, "already_received", got.Trial.Status)
	require.True(t, got.Trial.Legacy)
	require.Equal(t, float64(19), got.User.Balance)
	var claims int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM support_trial_grants WHERE user_id=$1`, target.ID).Scan(&claims))
	require.Zero(t, claims, "recognizing historical credit during lookup must remain read-only")
}

func TestUserSupportSuggestionsAreLiteralBoundedActiveAndFreshlyAuthorized(t *testing.T) {
	ctx := context.Background()
	admin := supportUser(t, service.RoleAdmin, 0)
	staff := supportUser(t, service.RoleSubAdmin, 0)
	r := NewUserSupportRepository(integrationDB)
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, true))
	prefix := "suggest-" + uuid.NewString()
	var visibleIDs []int64
	for i := 0; i < 11; i++ {
		u := supportUser(t, service.RoleUser, 0)
		_, err := integrationDB.Exec(`UPDATE users SET email=$1 WHERE id=$2`, fmt.Sprintf("%s-%%_-%02d@example.com", prefix, i), u.ID)
		require.NoError(t, err)
		if i == 9 {
			_, err = integrationDB.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, u.ID)
			require.NoError(t, err)
		} else if i == 10 {
			_, err = integrationDB.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=$1`, u.ID)
			require.NoError(t, err)
		} else {
			visibleIDs = append(visibleIDs, u.ID)
		}
	}
	decoy := supportUser(t, service.RoleUser, 0)
	_, err := integrationDB.Exec(`UPDATE users SET email=$1 WHERE id=$2`, prefix+"-XY-00@example.com", decoy.ID)
	require.NoError(t, err)
	rows, err := r.Search(ctx, staff.ID, strings.ToUpper(prefix)+"-%_")
	require.NoError(t, err)
	require.Len(t, rows, 8)
	for _, row := range rows {
		require.Contains(t, visibleIDs, row.ID)
		require.Contains(t, row.Email, "-%_-")
	}
	rows, err = r.Search(ctx, staff.ID, strconv.FormatInt(decoy.ID, 10))
	require.NoError(t, err)
	require.Contains(t, rows, service.UserSupportSuggestion{ID: decoy.ID, Email: prefix + "-XY-00@example.com"})
	require.Equal(t, decoy.ID, rows[0].ID, "an ID match must precede email substring matches that could fill the eight suggestions")
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, false))
	_, err = r.Search(ctx, staff.ID, prefix)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
	_, err = r.Lookup(ctx, staff.ID, decoy.Email)
	require.ErrorIs(t, err, service.ErrUserSupportForbidden)
}

func TestUserSupportExplicitGrantRechecksRevocationAfterWalletWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin := supportUser(t, service.RoleAdmin, 0)
	staff := supportUser(t, service.RoleSubAdmin, 0)
	target := supportUser(t, service.RoleUser, 7)
	r := NewUserSupportRepository(integrationDB)
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, true))
	lock, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback() }()
	var lockedID, blockerPID int64
	require.NoError(t, lock.QueryRowContext(ctx, `SELECT id,pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE`, target.ID).Scan(&lockedID, &blockerPID))
	finished := make(chan error, 1)
	go func() {
		_, err := r.LookupAndGrant(ctx, staff.ID, strconv.FormatInt(target.ID, 10))
		finished <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, blockerPID).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "the explicit grant must be waiting for the target wallet before revocation")
	require.NoError(t, r.SetAccess(ctx, admin.ID, staff.ID, false))
	require.NoError(t, lock.Commit())
	require.ErrorIs(t, <-finished, service.ErrUserSupportForbidden)
	var balance float64
	var claims int
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, target.ID).Scan(&balance))
	require.Equal(t, float64(7), balance)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM support_trial_grants WHERE user_id=$1`, target.ID).Scan(&claims))
	require.Zero(t, claims)
}
