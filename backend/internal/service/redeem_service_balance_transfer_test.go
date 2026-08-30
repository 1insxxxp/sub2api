package service

import (
	"context"
	"math"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type balanceTransferUserRepo struct {
	*userRepoStub
	balance     float64
	adjustCalls []float64
}

func (r *balanceTransferUserRepo) AdjustBalance(_ context.Context, id int64, delta float64) (BalanceChange, error) {
	r.adjustCalls = append(r.adjustCalls, delta)
	if r.user == nil || r.user.ID != id {
		return BalanceChange{}, ErrUserNotFound
	}
	old := r.balance
	next := old + delta
	if next < 0 {
		return BalanceChange{Old: old, New: next}, ErrBalanceNegative
	}
	r.balance = next
	r.user.Balance = next
	return BalanceChange{Old: old, New: next}, nil
}

type balanceTransferRedeemRepo struct {
	*redeemRejectRepo
	codes               map[int64]*RedeemCode
	nextID              int64
	created             []*RedeemCode
	deleted             []int64
	generated           []RedeemCode
	history             []RedeemCode
	atomicDeleteErr     error
	requireAtomicDelete bool
}

func (r *balanceTransferRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	if r.nextID == 0 {
		r.nextID = 100
	}
	code.ID = r.nextID
	r.nextID++
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	clone := *code
	r.created = append(r.created, &clone)
	if r.codes == nil {
		r.codes = make(map[int64]*RedeemCode)
	}
	stored := clone
	r.codes[clone.ID] = &stored
	return nil
}

func (r *balanceTransferRedeemRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	if r.codes != nil {
		if code, ok := r.codes[id]; ok {
			clone := *code
			return &clone, nil
		}
	}
	return nil, ErrRedeemCodeNotFound
}

func (r *balanceTransferRedeemRepo) Delete(_ context.Context, id int64) error {
	if r.requireAtomicDelete {
		panic("Delete called instead of atomic balance transfer delete")
	}
	r.deleted = append(r.deleted, id)
	delete(r.codes, id)
	return nil
}

func (r *balanceTransferRedeemRepo) DeleteBalanceTransferByCreator(_ context.Context, userID, codeID int64) (*RedeemCode, error) {
	if r.atomicDeleteErr != nil {
		return nil, r.atomicDeleteErr
	}
	code, err := r.GetByID(context.Background(), codeID)
	if err != nil {
		return nil, ErrBalanceTransferRedeemCodeNotFound
	}
	if code.Source != RedeemCodeSourceUserBalanceTransfer ||
		code.Type != RedeemTypeBalance ||
		code.CreatedBy == nil ||
		*code.CreatedBy != userID {
		return nil, ErrBalanceTransferRedeemCodeNotFound
	}
	r.deleted = append(r.deleted, codeID)
	delete(r.codes, codeID)
	return code, nil
}

func (r *balanceTransferRedeemRepo) DeleteBalanceTransfersByCreator(_ context.Context, userID int64, codeIDs []int64) ([]RedeemCode, error) {
	if r.atomicDeleteErr != nil {
		return nil, r.atomicDeleteErr
	}
	if len(codeIDs) == 0 {
		return []RedeemCode{}, nil
	}

	seen := make(map[int64]struct{}, len(codeIDs))
	codes := make([]RedeemCode, 0, len(codeIDs))
	for _, codeID := range codeIDs {
		if _, ok := seen[codeID]; ok {
			continue
		}
		seen[codeID] = struct{}{}

		code, err := r.GetByID(context.Background(), codeID)
		if err != nil {
			return nil, ErrBalanceTransferRedeemCodeNotFound
		}
		if code.Source != RedeemCodeSourceUserBalanceTransfer ||
			code.Type != RedeemTypeBalance ||
			code.CreatedBy == nil ||
			*code.CreatedBy != userID {
			return nil, ErrBalanceTransferRedeemCodeNotFound
		}
		codes = append(codes, *code)
	}

	for _, code := range codes {
		r.deleted = append(r.deleted, code.ID)
		delete(r.codes, code.ID)
	}
	return codes, nil
}

func (r *balanceTransferRedeemRepo) ListByCreator(_ context.Context, userID int64, limit int) ([]RedeemCode, error) {
	if limit <= 0 {
		limit = 25
	}
	out := make([]RedeemCode, 0)
	for _, code := range r.generated {
		if code.CreatedBy == nil || *code.CreatedBy != userID {
			continue
		}
		out = append(out, code)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *balanceTransferRedeemRepo) ListByCreatorPaginated(_ context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	out := make([]RedeemCode, 0)
	for _, code := range r.generated {
		if code.CreatedBy == nil || *code.CreatedBy != userID {
			continue
		}
		if code.Source != "" && code.Source != RedeemCodeSourceUserBalanceTransfer {
			continue
		}
		out = append(out, code)
	}
	return paginateRedeemCodeTestSlice(out, params), redeemCodeTestPaginationResult(len(out), params), nil
}

func (r *balanceTransferRedeemRepo) ListByUserPaginated(_ context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	out := make([]RedeemCode, 0)
	for _, code := range r.history {
		if code.UsedBy == nil || *code.UsedBy != userID {
			continue
		}
		if codeType != "" && code.Type != codeType {
			continue
		}
		out = append(out, code)
	}
	return paginateRedeemCodeTestSlice(out, params), redeemCodeTestPaginationResult(len(out), params), nil
}

func paginateRedeemCodeTestSlice(codes []RedeemCode, params pagination.PaginationParams) []RedeemCode {
	offset := params.Offset()
	if offset >= len(codes) {
		return []RedeemCode{}
	}
	end := offset + params.Limit()
	if end > len(codes) {
		end = len(codes)
	}
	return codes[offset:end]
}

func redeemCodeTestPaginationResult(total int, params pagination.PaginationParams) *pagination.PaginationResult {
	pageSize := params.Limit()
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	return &pagination.PaginationResult{
		Total:    int64(total),
		Page:     params.Page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

func TestRedeemServiceBalanceTransferRejectsUnauthorizedUser(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Balance: 50}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, 7, GenerateBalanceTransferCodeInput{Amount: 10})

	require.Nil(t, got)
	require.Error(t, err)
	require.True(t, infraerrors.IsForbidden(err))
	require.Equal(t, "BALANCE_TRANSFER_REDEEM_NOT_ALLOWED", infraerrors.Reason(err))
	require.Empty(t, redeemRepo.created)
	require.Empty(t, userRepo.adjustCalls)
	require.Equal(t, 50.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferAllowsSubAdminRole(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Role: RoleSubAdmin, Balance: 50}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{Amount: 10, Count: 1})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, []float64{-10}, userRepo.adjustCalls)
	require.Equal(t, 40.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferAllowsRegularUserWithExplicitPermission(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Role: RoleUser, Balance: 50, BalanceRedeemCodeEnabled: true}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCodes(ctx, 7, GenerateBalanceTransferCodeInput{Amount: 10, Count: 1})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, redeemRepo.created, 1)
	require.Equal(t, []float64{-10}, userRepo.adjustCalls)
	require.Equal(t, 40.0, userRepo.balance)
}

func TestRedeemServiceConvertBalanceToRedeemCodesReturnsUpdatedBalance(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Role: RoleSubAdmin, Balance: 50}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	result, err := svc.ConvertBalanceToRedeemCodes(ctx, 7, ConvertBalanceToRedeemCodesInput{
		Value: 10,
		Count: 2,
	})

	require.NoError(t, err)
	require.Len(t, result.Codes, 2)
	require.Equal(t, 20.0, result.TotalValue)
	require.Equal(t, 30.0, result.NewBalance)
}

func TestRedeemServiceBalanceTransferRejectsInsufficientBalance(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Role: RoleSubAdmin, Balance: 3, BalanceRedeemCodeEnabled: true}},
		balance:      3,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, 7, GenerateBalanceTransferCodeInput{Amount: 10})

	require.Nil(t, got)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Empty(t, redeemRepo.created)
	require.Equal(t, []float64{-10}, userRepo.adjustCalls)
	require.Equal(t, 3.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferCreatesCodeAndDeductsBalance(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Role: RoleSubAdmin, Balance: 50, BalanceRedeemCodeEnabled: true}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, userID, GenerateBalanceTransferCodeInput{
		Amount:        12.5,
		ExpiresInDays: 14,
		Notes:         "for teammate",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Code, 32)
	require.Equal(t, RedeemTypeBalance, got.Type)
	require.Equal(t, 12.5, got.Value)
	require.Equal(t, StatusUnused, got.Status)
	require.NotNil(t, got.CreatedBy)
	require.Equal(t, userID, *got.CreatedBy)
	require.Equal(t, RedeemCodeSourceUserBalanceTransfer, got.Source)
	require.Equal(t, "for teammate", got.Notes)
	require.Equal(t, 14, got.ValidityDays)
	require.NotNil(t, got.ExpiresAt)
	require.WithinDuration(t, time.Now().UTC().AddDate(0, 0, 14), *got.ExpiresAt, 3*time.Second)
	require.Equal(t, []float64{-12.5}, userRepo.adjustCalls)
	require.Equal(t, 37.5, userRepo.balance)
	require.Len(t, redeemRepo.created, 1)
}

func TestRedeemServiceBalanceTransferBatchCreatesCodesAndDeductsTotal(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Role: RoleSubAdmin, Balance: 50, BalanceRedeemCodeEnabled: true}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{
		Amount:           5,
		Count:            3,
		ExpiresInDays:    14,
		Notes:            "team drop",
		SingleUsePerUser: true,
	})

	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, []float64{-15}, userRepo.adjustCalls)
	require.Equal(t, 35.0, userRepo.balance)
	require.Len(t, redeemRepo.created, 3)
	require.NotNil(t, got[0].BatchID)
	require.NotEmpty(t, *got[0].BatchID)
	for _, code := range got {
		require.Equal(t, RedeemTypeBalance, code.Type)
		require.Equal(t, 5.0, code.Value)
		require.Equal(t, StatusUnused, code.Status)
		require.NotNil(t, code.CreatedBy)
		require.Equal(t, userID, *code.CreatedBy)
		require.Equal(t, RedeemCodeSourceUserBalanceTransfer, code.Source)
		require.Equal(t, "team drop", code.Notes)
		require.NotNil(t, code.BatchID)
		require.Equal(t, *got[0].BatchID, *code.BatchID)
	}
}

func TestRedeemServiceBalanceTransferBatchRejectsInvalidCount(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 500, BalanceRedeemCodeEnabled: true}},
		balance:      500,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{Amount: 1, Count: 0})
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "BALANCE_TRANSFER_COUNT_INVALID", infraerrors.Reason(err))

	got, err = svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{Amount: 1, Count: 101})
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "BALANCE_TRANSFER_COUNT_INVALID", infraerrors.Reason(err))
	require.Empty(t, userRepo.adjustCalls)
	require.Empty(t, redeemRepo.created)
}

func TestRedeemServiceBalanceTransferDeleteUnusedCodeRefundsCreator(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	createdBy := userID
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 35, BalanceRedeemCodeEnabled: true}},
		balance:      35,
	}
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[int64]*RedeemCode{
			88: {
				ID:        88,
				Code:      "DELETE-ME",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUnused,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
		},
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.DeleteGeneratedBalanceTransferCode(ctx, userID, 88)

	require.NoError(t, err)
	require.NotNil(t, deleted)
	require.Equal(t, int64(88), deleted.ID)
	require.Equal(t, []float64{12.5}, userRepo.adjustCalls)
	require.Equal(t, 47.5, userRepo.balance)
	require.Equal(t, []int64{88}, redeemRepo.deleted)
	require.Empty(t, redeemRepo.codes)
}

func TestRedeemServiceBalanceTransferBatchDeleteUnusedCodesRefundsCreatorOnce(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	createdBy := userID
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 35, BalanceRedeemCodeEnabled: true}},
		balance:      35,
	}
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[int64]*RedeemCode{
			88: {
				ID:        88,
				Code:      "DELETE-ME-A",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUnused,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
			89: {
				ID:        89,
				Code:      "DELETE-ME-B",
				Type:      RedeemTypeBalance,
				Value:     5,
				Status:    StatusUnused,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
		},
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.DeleteGeneratedBalanceTransferCodes(ctx, userID, []int64{88, 89})

	require.NoError(t, err)
	require.Len(t, deleted, 2)
	require.Equal(t, []float64{17.5}, userRepo.adjustCalls)
	require.Equal(t, 52.5, userRepo.balance)
	require.Equal(t, []int64{88, 89}, redeemRepo.deleted)
	require.Empty(t, redeemRepo.codes)
}

func TestRedeemServiceBalanceTransferBatchDeleteRefundsOnlyUnusedCodes(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	createdBy := userID
	usedBy := int64(9)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 35, BalanceRedeemCodeEnabled: true}},
		balance:      35,
	}
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[int64]*RedeemCode{
			88: {
				ID:        88,
				Code:      "DELETE-ME",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUnused,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
			89: {
				ID:        89,
				Code:      "ALREADY-USED",
				Type:      RedeemTypeBalance,
				Value:     5,
				Status:    StatusUsed,
				UsedBy:    &usedBy,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
		},
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.DeleteGeneratedBalanceTransferCodes(ctx, userID, []int64{88, 89})

	require.NoError(t, err)
	require.Len(t, deleted, 2)
	require.Equal(t, []float64{12.5}, userRepo.adjustCalls)
	require.Equal(t, 47.5, userRepo.balance)
	require.Equal(t, []int64{88, 89}, redeemRepo.deleted)
	require.Empty(t, redeemRepo.codes)
}

func TestRedeemServiceBalanceTransferDeleteDoesNotRefundWhenAtomicDeleteLosesRace(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	createdBy := userID
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 35, BalanceRedeemCodeEnabled: true}},
		balance:      35,
	}
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo:    &redeemRejectRepo{},
		atomicDeleteErr:     ErrBalanceTransferRedeemCodeUsed,
		requireAtomicDelete: true,
		codes: map[int64]*RedeemCode{
			88: {
				ID:        88,
				Code:      "DELETE-ME",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUnused,
				CreatedBy: &createdBy,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
		},
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.DeleteGeneratedBalanceTransferCode(ctx, userID, 88)

	require.Nil(t, deleted)
	require.ErrorIs(t, err, ErrBalanceTransferRedeemCodeUsed)
	require.Empty(t, userRepo.adjustCalls)
	require.Equal(t, 35.0, userRepo.balance)
	require.Empty(t, redeemRepo.deleted)
}

func TestRedeemServiceBalanceTransferDeleteUsedCodeWithoutRefundAndRejectsNonOwnerCode(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	otherID := int64(8)
	usedBy := int64(9)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 35, BalanceRedeemCodeEnabled: true}},
		balance:      35,
	}
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[int64]*RedeemCode{
			88: {
				ID:        88,
				Code:      "USED",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUsed,
				UsedBy:    &usedBy,
				CreatedBy: &userID,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
			89: {
				ID:        89,
				Code:      "NOT-MINE",
				Type:      RedeemTypeBalance,
				Value:     12.5,
				Status:    StatusUnused,
				CreatedBy: &otherID,
				Source:    RedeemCodeSourceUserBalanceTransfer,
			},
		},
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.DeleteGeneratedBalanceTransferCode(ctx, userID, 88)
	require.NoError(t, err)
	require.NotNil(t, deleted)
	require.Equal(t, int64(88), deleted.ID)

	deleted, err = svc.DeleteGeneratedBalanceTransferCode(ctx, userID, 89)
	require.Nil(t, deleted)
	require.Error(t, err)
	require.Equal(t, "BALANCE_TRANSFER_REDEEM_CODE_NOT_FOUND", infraerrors.Reason(err))
	require.Empty(t, userRepo.adjustCalls)
	require.Equal(t, []int64{88}, redeemRepo.deleted)
	require.Equal(t, 35.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferListsOnlyCreatorCodes(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	otherID := int64(8)
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		generated: []RedeemCode{
			{ID: 1, Code: "OWN", CreatedBy: &userID, Source: RedeemCodeSourceUserBalanceTransfer},
			{ID: 2, Code: "OTHER", CreatedBy: &otherID, Source: RedeemCodeSourceUserBalanceTransfer},
		},
	}
	svc := NewRedeemService(redeemRepo, &balanceTransferUserRepo{userRepoStub: &userRepoStub{}}, nil, nil, nil, nil, nil, nil)

	codes, err := svc.ListGeneratedBalanceTransferCodes(ctx, userID, 25)

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, "OWN", codes[0].Code)
}

func TestRedeemServiceUserHistoryPaginated(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	otherID := int64(8)
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		history: []RedeemCode{
			{ID: 1, Code: "OWN-1", Type: RedeemTypeBalance, UsedBy: &userID},
			{ID: 2, Code: "OTHER", Type: RedeemTypeBalance, UsedBy: &otherID},
			{ID: 3, Code: "OWN-2", Type: RedeemTypeConcurrency, UsedBy: &userID},
			{ID: 4, Code: "OWN-3", Type: RedeemTypeBalance, UsedBy: &userID},
		},
	}
	svc := NewRedeemService(redeemRepo, &balanceTransferUserRepo{userRepoStub: &userRepoStub{}}, nil, nil, nil, nil, nil, nil)

	codes, page, err := svc.GetUserHistoryPaginated(ctx, userID, pagination.PaginationParams{Page: 2, PageSize: 2})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, "OWN-3", codes[0].Code)
	require.Equal(t, int64(3), page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 2, page.PageSize)
	require.Equal(t, 2, page.Pages)
}

func TestRedeemServiceGeneratedBalanceTransferCodesPaginated(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	otherID := int64(8)
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		generated: []RedeemCode{
			{ID: 1, Code: "OWN-1", CreatedBy: &userID, Source: RedeemCodeSourceUserBalanceTransfer},
			{ID: 2, Code: "OTHER", CreatedBy: &otherID, Source: RedeemCodeSourceUserBalanceTransfer},
			{ID: 3, Code: "OWN-2", CreatedBy: &userID, Source: RedeemCodeSourceUserBalanceTransfer},
			{ID: 4, Code: "OWN-3", CreatedBy: &userID, Source: RedeemCodeSourceUserBalanceTransfer},
		},
	}
	svc := NewRedeemService(redeemRepo, &balanceTransferUserRepo{userRepoStub: &userRepoStub{}}, nil, nil, nil, nil, nil, nil)

	codes, page, err := svc.ListGeneratedBalanceTransferCodesPaginated(ctx, userID, pagination.PaginationParams{Page: 2, PageSize: 2})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, "OWN-3", codes[0].Code)
	require.Equal(t, int64(3), page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 2, page.PageSize)
	require.Equal(t, 2, page.Pages)
}
