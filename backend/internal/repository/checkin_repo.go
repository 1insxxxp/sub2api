package repository

import (
	"context"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinblacklist"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinstatussnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type checkinRepository struct {
	client *dbent.Client
}

func NewCheckinRepository(client *dbent.Client) service.CheckinRepository {
	return &checkinRepository{client: client}
}

func (r *checkinRepository) GetStatus(ctx context.Context, userID int64, checkinDate string) (*service.CheckinStatus, error) {
	client := clientFromContext(ctx, r.client)
	status := &service.CheckinStatus{
		UserID:      userID,
		CheckinDate: checkinDate,
	}

	blacklisted, err := client.UserCheckinBlacklist.Query().
		Where(usercheckinblacklist.UserIDEQ(userID), usercheckinblacklist.RemovedAtIsNil()).
		Exist(ctx)
	if err != nil {
		return nil, err
	}
	status.Blacklisted = blacklisted

	snapshot, err := client.UserCheckinStatusSnapshot.Query().
		Where(usercheckinstatussnapshot.UserIDEQ(userID)).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}
	if snapshot != nil {
		status.CurrentStreak = snapshot.CurrentStreak
		status.LastCheckinDate = snapshot.LastCheckinDate
		status.LifetimeCheckinDays = snapshot.LifetimeCheckinDays
	}

	record, err := client.UserCheckin.Query().
		Where(usercheckin.UserIDEQ(userID), usercheckin.CheckinDateEQ(checkinDate)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return status, nil
		}
		return nil, err
	}
	applyCheckinRecord(status, record)
	return status, nil
}

func (r *checkinRepository) Checkin(ctx context.Context, input service.CheckinCreateInput) (*service.CheckinResult, error) {
	var out *service.CheckinResult
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		blacklisted, err := txClient.UserCheckinBlacklist.Query().
			Where(usercheckinblacklist.UserIDEQ(input.UserID), usercheckinblacklist.RemovedAtIsNil()).
			Exist(txCtx)
		if err != nil {
			return err
		}
		if blacklisted {
			return service.ErrCheckinBlacklisted
		}

		existing, err := txClient.UserCheckin.Query().
			Where(usercheckin.UserIDEQ(input.UserID), usercheckin.CheckinDateEQ(input.CheckinDate)).
			Only(txCtx)
		if err != nil && !dbent.IsNotFound(err) {
			return err
		}
		if existing != nil {
			status, err := r.statusFromRecord(txCtx, txClient, input.UserID, input.CheckinDate, existing)
			if err != nil {
				return err
			}
			out = &service.CheckinResult{CheckinStatus: *status}
			return nil
		}

		userEntity, err := txClient.User.Query().
			Where(user.IDEQ(input.UserID)).
			ForUpdate().
			Only(txCtx)
		if isForUpdateUnsupported(err) {
			userEntity, err = txClient.User.Query().
				Where(user.IDEQ(input.UserID)).
				Only(txCtx)
		}
		if err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}

		snapshot, err := txClient.UserCheckinStatusSnapshot.Query().
			Where(usercheckinstatussnapshot.UserIDEQ(input.UserID)).
			ForUpdate().
			Only(txCtx)
		if isForUpdateUnsupported(err) {
			snapshot, err = txClient.UserCheckinStatusSnapshot.Query().
				Where(usercheckinstatussnapshot.UserIDEQ(input.UserID)).
				Only(txCtx)
		}
		if err != nil && !dbent.IsNotFound(err) {
			return err
		}

		streakDay, lifetimeDays := nextCheckinCounters(input.CheckinDate, snapshot)
		bonus := 0.0
		if input.StreakEnabled {
			bonus = service.CheckinBonusForStreak(streakDay, input.StreakRules)
		}
		total := input.BaseRewardAmount + bonus
		balanceBefore := userEntity.Balance
		balanceAfter := balanceBefore + total

		record, err := txClient.UserCheckin.Create().
			SetUserID(input.UserID).
			SetCheckinDate(input.CheckinDate).
			SetRewardAmount(total).
			SetBalanceBefore(balanceBefore).
			SetBalanceAfter(balanceAfter).
			SetStreakDay(streakDay).
			SetBaseRewardAmount(input.BaseRewardAmount).
			SetBonusRewardAmount(bonus).
			SetTotalRewardAmount(total).
			Save(txCtx)
		if err != nil {
			return err
		}

		if err := txClient.User.UpdateOneID(input.UserID).
			AddBalance(total).
			Exec(txCtx); err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}

		if snapshot == nil {
			_, err = txClient.UserCheckinStatusSnapshot.Create().
				SetUserID(input.UserID).
				SetCurrentStreak(streakDay).
				SetLastCheckinDate(input.CheckinDate).
				SetLifetimeCheckinDays(lifetimeDays).
				Save(txCtx)
		} else {
			_, err = txClient.UserCheckinStatusSnapshot.UpdateOneID(snapshot.ID).
				SetCurrentStreak(streakDay).
				SetLastCheckinDate(input.CheckinDate).
				SetLifetimeCheckinDays(lifetimeDays).
				Save(txCtx)
		}
		if err != nil {
			return err
		}

		status := &service.CheckinStatus{
			UserID:              input.UserID,
			CheckinDate:         input.CheckinDate,
			CurrentStreak:       streakDay,
			LastCheckinDate:     input.CheckinDate,
			LifetimeCheckinDays: lifetimeDays,
		}
		applyCheckinRecord(status, record)
		out = &service.CheckinResult{CheckinStatus: *status}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *checkinRepository) statusFromRecord(ctx context.Context, client *dbent.Client, userID int64, checkinDate string, record *dbent.UserCheckin) (*service.CheckinStatus, error) {
	status := &service.CheckinStatus{
		UserID:      userID,
		CheckinDate: checkinDate,
	}
	snapshot, err := client.UserCheckinStatusSnapshot.Query().
		Where(usercheckinstatussnapshot.UserIDEQ(userID)).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}
	if snapshot != nil {
		status.CurrentStreak = snapshot.CurrentStreak
		status.LastCheckinDate = snapshot.LastCheckinDate
		status.LifetimeCheckinDays = snapshot.LifetimeCheckinDays
	}
	applyCheckinRecord(status, record)
	return status, nil
}

func (r *checkinRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func applyCheckinRecord(status *service.CheckinStatus, record *dbent.UserCheckin) {
	if status == nil || record == nil {
		return
	}
	status.CheckedInToday = true
	status.CurrentStreak = record.StreakDay
	status.BaseRewardAmount = record.BaseRewardAmount
	status.BonusRewardAmount = record.BonusRewardAmount
	status.TotalRewardAmount = record.TotalRewardAmount
	status.BalanceBefore = record.BalanceBefore
	status.BalanceAfter = record.BalanceAfter
	if status.LifetimeCheckinDays == 0 {
		status.LifetimeCheckinDays = 1
	}
}

func nextCheckinCounters(checkinDate string, snapshot *dbent.UserCheckinStatusSnapshot) (streakDay int, lifetimeDays int) {
	if snapshot == nil {
		return 1, 1
	}
	lifetimeDays = snapshot.LifetimeCheckinDays + 1
	if isPreviousCheckinDate(snapshot.LastCheckinDate, checkinDate) {
		return snapshot.CurrentStreak + 1, lifetimeDays
	}
	return 1, lifetimeDays
}

func isPreviousCheckinDate(previous, current string) bool {
	prevDate, err := time.Parse(time.DateOnly, previous)
	if err != nil {
		return false
	}
	currentDate, err := time.Parse(time.DateOnly, current)
	if err != nil {
		return false
	}
	return prevDate.AddDate(0, 0, 1).Equal(currentDate)
}

func isForUpdateUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SELECT .. FOR UPDATE/SHARE not supported")
}
