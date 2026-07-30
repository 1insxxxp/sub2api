//go:build !unit

package service

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type userRepoStub struct {
	user          *User
	getErr        error
	createErr     error
	deleteErr     error
	exists        bool
	existsErr     error
	nextID        int64
	created       []*User
	updated       []*User
	deletedIDs    []int64
	usersByEmail  map[string]*User
	getByEmailErr error
}

func (s *userRepoStub) Create(ctx context.Context, user *User) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.nextID != 0 && user.ID == 0 {
		user.ID = s.nextID
	}
	s.created = append(s.created, user)
	if s.usersByEmail == nil {
		s.usersByEmail = make(map[string]*User)
	}
	s.usersByEmail[user.Email] = user
	s.user = user
	return nil
}

func (s *userRepoStub) CreateWithEmailAliasGuard(ctx context.Context, user *User) error {
	return s.Create(ctx, user)
}

func (s *userRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	return s.user, nil
}

func (s *userRepoStub) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return s.GetByID(ctx, id)
}

func (s *userRepoStub) GetByEmail(ctx context.Context, email string) (*User, error) {
	if s.getByEmailErr != nil {
		return nil, s.getByEmailErr
	}
	if s.usersByEmail != nil {
		if user, ok := s.usersByEmail[email]; ok {
			return user, nil
		}
	}
	if s.user != nil && s.user.Email == email {
		return s.user, nil
	}
	return nil, ErrUserNotFound
}

func (s *userRepoStub) GetFirstAdmin(ctx context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}

func (s *userRepoStub) Update(ctx context.Context, user *User, _ UserUpdateFields) error {
	s.updated = append(s.updated, user)
	if s.usersByEmail == nil {
		s.usersByEmail = make(map[string]*User)
	}
	s.usersByEmail[user.Email] = user
	s.user = user
	return nil
}

func (s *userRepoStub) Delete(ctx context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteErr
}

func (s *userRepoStub) GetUserAvatar(ctx context.Context, userID int64) (*UserAvatar, error) {
	panic("unexpected GetUserAvatar call")
}

func (s *userRepoStub) UpsertUserAvatar(ctx context.Context, userID int64, input UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertUserAvatar call")
}

func (s *userRepoStub) DeleteUserAvatar(ctx context.Context, userID int64) error {
	panic("unexpected DeleteUserAvatar call")
}

func (s *userRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *userRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *userRepoStub) GetLatestUsedAtByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserIDs call")
}

func (s *userRepoStub) GetLatestUsedAtByUserID(ctx context.Context, userID int64) (*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserID call")
}

func (s *userRepoStub) UpdateUserLastActiveAt(ctx context.Context, userID int64, activeAt time.Time) error {
	panic("unexpected UpdateUserLastActiveAt call")
}

func (s *userRepoStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *userRepoStub) AdjustBalance(ctx context.Context, id int64, delta float64) (BalanceChange, error) {
	panic("unexpected AdjustBalance call")
}

func (s *userRepoStub) SetBalance(ctx context.Context, id int64, value float64) (BalanceChange, error) {
	panic("unexpected SetBalance call")
}

func (s *userRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected DeductBalance call")
}

func (s *userRepoStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *userRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}

func (s *userRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}

func (s *userRepoStub) BatchUpdateLimits(context.Context, []int64, *int, *int) (int, error) {
	return 0, nil
}

func (s *userRepoStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.exists, nil
}

func (s *userRepoStub) ExistsByEmailAlias(ctx context.Context, email string) (bool, error) {
	return s.ExistsByEmail(ctx, email)
}

func (s *userRepoStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *userRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *userRepoStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *userRepoStub) ListUserAuthIdentities(ctx context.Context, userID int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected ListUserAuthIdentities call")
}

func (s *userRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected UnbindUserAuthProvider call")
}

func (s *userRepoStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *userRepoStub) EnableTotp(ctx context.Context, userID int64) error {
	panic("unexpected EnableTotp call")
}

func (s *userRepoStub) DisableTotp(ctx context.Context, userID int64) error {
	panic("unexpected DisableTotp call")
}

func newCheckinServiceTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:redeem_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

type batchRedeemRepo struct {
	*redeemRejectRepo
	codes map[string]*RedeemCode
}

func (r *batchRedeemRepo) Create(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Create call")
}

func (r *batchRedeemRepo) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *batchRedeemRepo) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	stored, ok := r.codes[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	clone := *stored
	return &clone, nil
}

func (r *batchRedeemRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	for _, stored := range r.codes {
		if stored.ID == id {
			clone := *stored
			return &clone, nil
		}
	}
	return nil, ErrRedeemCodeNotFound
}

func (r *batchRedeemRepo) Update(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *batchRedeemRepo) BatchUpdate(ctx context.Context, ids []int64, fields RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (r *batchRedeemRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *batchRedeemRepo) Use(_ context.Context, id, userID int64) error {
	for _, stored := range r.codes {
		if stored.ID != id {
			continue
		}
		if stored.Status != StatusUnused {
			return ErrRedeemCodeUsed
		}
		stored.Status = StatusUsed
		stored.UsedBy = &userID
		return nil
	}
	return ErrRedeemCodeNotFound
}

func (r *batchRedeemRepo) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *batchRedeemRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *batchRedeemRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *batchRedeemRepo) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *batchRedeemRepo) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

type batchRedeemUserRepo struct {
	*userRepoStub
	balance float64
}

func (r *batchRedeemUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.balance += amount
	return nil
}

type affiliateTierServiceRepoStub struct {
	AffiliateRepository
	AffiliateQualificationRepository
	mu                    sync.Mutex
	qualifiedCount        int
	inviteeSummary        *AffiliateSummary
	inviterSummary        *AffiliateSummary
	accruedAmount         float64
	reconcileInviteeCalls int
	reconcileInviteeErr   error
}

func (r *affiliateTierServiceRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if r.inviteeSummary != nil && r.inviteeSummary.UserID == userID {
		return r.inviteeSummary, nil
	}
	if r.inviterSummary != nil && r.inviterSummary.UserID == userID {
		return r.inviterSummary, nil
	}
	return &AffiliateSummary{UserID: userID}, nil
}

func (r *affiliateTierServiceRepoStub) AccrueQuota(_ context.Context, _, _ int64, amount float64, _ int, _ *int64) (bool, error) {
	r.accruedAmount = amount
	return true, nil
}

func (r *affiliateTierServiceRepoStub) CountQualifiedInvitees(context.Context, int64, float64) (int, error) {
	return r.qualifiedCount, nil
}

func (r *affiliateTierServiceRepoStub) TryWithAffiliateQualificationReconcileLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if fn == nil {
		return true, nil
	}
	return true, fn(ctx)
}

func (r *affiliateTierServiceRepoStub) ListAffiliateQualificationDirtyEvents(context.Context, int) ([]AffiliateQualificationDirtyEvent, error) {
	return nil, nil
}

func (r *affiliateTierServiceRepoStub) ReadReconcilePendingSnapshot(context.Context) (AffiliateReconcilePendingSnapshot, error) {
	return AffiliateReconcilePendingSnapshot{}, nil
}

func (r *affiliateTierServiceRepoStub) ReconcileInviteeQualification(context.Context, int64, float64) (*AffiliateQualification, error) {
	r.mu.Lock()
	r.reconcileInviteeCalls++
	r.mu.Unlock()
	return nil, r.reconcileInviteeErr
}

type affiliateTierServiceSettingRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func newAffiliateTierServiceSettingRepo() *affiliateTierServiceSettingRepo {
	return &affiliateTierServiceSettingRepo{values: map[string]string{
		SettingKeyAffiliateRebateRate:          "8",
		SettingKeyAffiliateQualificationAmount: "50",
		SettingKeyAffiliateBronzeInvitees:      "3",
		SettingKeyAffiliateBronzeRate:          "10",
		SettingKeyAffiliateSilverInvitees:      "10",
		SettingKeyAffiliateSilverRate:          "12",
		SettingKeyAffiliateGoldInvitees:        "30",
		SettingKeyAffiliateGoldRate:            "15",
	}}
}

func (r *affiliateTierServiceSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *affiliateTierServiceSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *affiliateTierServiceSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *affiliateTierServiceSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *affiliateTierServiceSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *affiliateTierServiceSettingRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *affiliateTierServiceSettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}
