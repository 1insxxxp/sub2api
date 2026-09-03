package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/lotteryactivity"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptgrant"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptwallet"
	"github.com/Wei-Shaw/sub2api/ent/lotterydraw"
	"github.com/Wei-Shaw/sub2api/ent/lotteryprize"
	"github.com/Wei-Shaw/sub2api/ent/lotteryprizeitem"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type lotteryRepository struct {
	client *dbent.Client
}

var _ service.LotteryRepository = (*lotteryRepository)(nil)

func NewLotteryRepository(client *dbent.Client) service.LotteryRepository {
	return &lotteryRepository{client: client}
}

func (r *lotteryRepository) GetActivity(ctx context.Context) (*service.LotteryActivity, error) {
	activity, err := clientFromContext(ctx, r.client).LotteryActivity.Query().Order(lotteryactivity.ByID()).First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrLotteryActivityNotFound
		}
		return nil, err
	}
	return lotteryActivityToService(activity), nil
}

func (r *lotteryRepository) GetActiveActivity(ctx context.Context, now time.Time, forUpdate bool) (*service.LotteryActivity, error) {
	query := clientFromContext(ctx, r.client).LotteryActivity.Query().Where(
		lotteryactivity.StatusEQ(service.LotteryActivityStatusActive),
		lotteryactivity.Or(lotteryactivity.StartsAtIsNil(), lotteryactivity.StartsAtLTE(now)),
		lotteryactivity.Or(lotteryactivity.EndsAtIsNil(), lotteryactivity.EndsAtGTE(now)),
	).Order(lotteryactivity.ByID())
	if forUpdate {
		query = query.ForUpdate()
	}
	activity, err := query.First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrLotteryActivityNotFound
		}
		return nil, err
	}
	return lotteryActivityToService(activity), nil
}

func (r *lotteryRepository) ListPrizes(ctx context.Context, activityID int64, includeInventory bool) ([]service.LotteryPrize, error) {
	prizes, err := clientFromContext(ctx, r.client).LotteryPrize.Query().
		Where(lotteryprize.ActivityIDEQ(activityID)).
		Order(lotteryprize.BySortOrder(), lotteryprize.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]service.LotteryPrize, 0, len(prizes))
	for _, prize := range prizes {
		item := lotteryPrizeToService(prize)
		if includeInventory && prize.Type == service.LotteryPrizeTypeProduct {
			count, err := clientFromContext(ctx, r.client).LotteryPrizeItem.Query().
				Where(
					lotteryprizeitem.PrizeIDEQ(prize.ID),
					lotteryprizeitem.StatusEQ(service.LotteryPrizeItemStatusAvailable),
				).
				Count(ctx)
			if err != nil {
				return nil, err
			}
			item.AvailableItemCount = count
		}
		result = append(result, *item)
	}
	return result, nil
}

func (r *lotteryRepository) SaveActivity(ctx context.Context, activityID int64, input service.LotteryActivityInput, createdBy *int64) (*service.LotteryActivity, error) {
	client := clientFromContext(ctx, r.client)
	var (
		activity *dbent.LotteryActivity
		err      error
	)
	if activityID > 0 {
		activity, err = client.LotteryActivity.Get(ctx, activityID)
	} else {
		activity, err = client.LotteryActivity.Query().Order(lotteryactivity.ByID()).First(ctx)
	}
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}
	if dbent.IsNotFound(err) {
		builder := client.LotteryActivity.Create().
			SetName(strings.TrimSpace(input.Name)).
			SetDescription(input.Description).
			SetStatus(input.Status).
			SetAttemptMode(input.AttemptMode).
			SetAttemptLimit(input.AttemptLimit).
			SetNillableStartsAt(input.StartsAt).
			SetNillableEndsAt(input.EndsAt).
			SetNillableCreatedBy(createdBy)
		activity, err = builder.Save(ctx)
		if err != nil {
			return nil, err
		}
		return lotteryActivityToService(activity), nil
	}

	update := client.LotteryActivity.UpdateOneID(activity.ID).
		SetName(strings.TrimSpace(input.Name)).
		SetDescription(input.Description).
		SetStatus(input.Status).
		SetAttemptMode(input.AttemptMode).
		SetAttemptLimit(input.AttemptLimit)
	if input.StartsAt != nil {
		update.SetStartsAt(*input.StartsAt)
	} else {
		update.ClearStartsAt()
	}
	if input.EndsAt != nil {
		update.SetEndsAt(*input.EndsAt)
	} else {
		update.ClearEndsAt()
	}
	activity, err = update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return lotteryActivityToService(activity), nil
}

func (r *lotteryRepository) CreatePrize(ctx context.Context, activityID int64, input service.LotteryPrizeInput) (*service.LotteryPrize, error) {
	prize, err := clientFromContext(ctx, r.client).LotteryPrize.Create().
		SetActivityID(activityID).
		SetName(strings.TrimSpace(input.Name)).
		SetDescription(input.Description).
		SetType(input.Type).
		SetWeight(input.Weight).
		SetNillableBalanceAmount(input.BalanceAmount).
		SetEnabled(input.Enabled).
		SetSortOrder(input.SortOrder).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return lotteryPrizeToService(prize), nil
}

func (r *lotteryRepository) UpdatePrize(ctx context.Context, input service.LotteryPrizeInput) (*service.LotteryPrize, error) {
	client := clientFromContext(ctx, r.client)
	update := client.LotteryPrize.UpdateOneID(input.ID).
		SetName(strings.TrimSpace(input.Name)).
		SetDescription(input.Description).
		SetType(input.Type).
		SetWeight(input.Weight).
		SetEnabled(input.Enabled).
		SetSortOrder(input.SortOrder)
	if input.BalanceAmount != nil {
		update.SetBalanceAmount(*input.BalanceAmount)
	} else {
		update.ClearBalanceAmount()
	}
	prize, err := update.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrLotteryPrizeInvalid
		}
		return nil, err
	}
	return lotteryPrizeToService(prize), nil
}

func (r *lotteryRepository) DeletePrize(ctx context.Context, prizeID int64) error {
	deleted, err := clientFromContext(ctx, r.client).LotteryPrize.Delete().Where(lotteryprize.IDEQ(prizeID)).Exec(ctx)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return service.ErrLotteryPrizeInvalid
	}
	return nil
}

func (r *lotteryRepository) AppendPrizeItems(ctx context.Context, prizeID int64, contents []string) (int, error) {
	client := clientFromContext(ctx, r.client)
	builders := make([]*dbent.LotteryPrizeItemCreate, 0, len(contents))
	for _, raw := range contents {
		content := strings.TrimSpace(raw)
		if content == "" {
			continue
		}
		builders = append(builders, client.LotteryPrizeItem.Create().
			SetPrizeID(prizeID).
			SetContent(content).
			SetStatus(service.LotteryPrizeItemStatusAvailable))
	}
	if len(builders) == 0 {
		return 0, nil
	}
	items, err := client.LotteryPrizeItem.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func (r *lotteryRepository) ListPrizeItems(ctx context.Context, prizeID int64, includeContent bool) ([]service.LotteryPrizeItem, error) {
	items, err := clientFromContext(ctx, r.client).LotteryPrizeItem.Query().
		Where(lotteryprizeitem.PrizeIDEQ(prizeID)).
		Order(lotteryprizeitem.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]service.LotteryPrizeItem, 0, len(items))
	for _, item := range items {
		mapped := lotteryPrizeItemToService(item)
		if !includeContent {
			mapped.Content = ""
		}
		result = append(result, *mapped)
	}
	return result, nil
}

func (r *lotteryRepository) DeleteAvailablePrizeItems(ctx context.Context, prizeID int64, itemIDs []int64) (int, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}
	return clientFromContext(ctx, r.client).LotteryPrizeItem.Delete().Where(
		lotteryprizeitem.PrizeIDEQ(prizeID),
		lotteryprizeitem.IDIn(itemIDs...),
		lotteryprizeitem.StatusEQ(service.LotteryPrizeItemStatusAvailable),
	).Exec(ctx)
}

func (r *lotteryRepository) CountUserDraws(ctx context.Context, activityID, userID int64, since *time.Time) (int, error) {
	query := clientFromContext(ctx, r.client).LotteryDraw.Query().Where(
		lotterydraw.ActivityIDEQ(activityID),
		lotterydraw.UserIDEQ(userID),
		lotterydraw.AttemptSourceEQ(service.LotteryAttemptSourceActivity),
	)
	if since != nil {
		query = query.Where(lotterydraw.CreatedAtGTE(*since))
	}
	return query.Count(ctx)
}

func (r *lotteryRepository) GetDrawByAttemptKey(ctx context.Context, attemptKey string) (*service.LotteryDraw, error) {
	draw, err := clientFromContext(ctx, r.client).LotteryDraw.Query().Where(lotterydraw.AttemptKeyEQ(attemptKey)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrLotteryDrawNotFound
		}
		return nil, err
	}
	return lotteryDrawToService(draw), nil
}

func (r *lotteryRepository) ClaimAvailableProductItem(ctx context.Context, prizeID, userID int64, now time.Time) (*service.LotteryPrizeItem, error) {
	client := clientFromContext(ctx, r.client)
	item, err := client.LotteryPrizeItem.Query().Where(
		lotteryprizeitem.PrizeIDEQ(prizeID),
		lotteryprizeitem.StatusEQ(service.LotteryPrizeItemStatusAvailable),
	).Order(lotteryprizeitem.ByID()).ForUpdate().First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrLotteryProductUnavailable
		}
		return nil, err
	}
	claimed, err := client.LotteryPrizeItem.UpdateOneID(item.ID).
		SetStatus(service.LotteryPrizeItemStatusClaimed).
		SetClaimedBy(userID).
		SetClaimedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return lotteryPrizeItemToService(claimed), nil
}

func (r *lotteryRepository) CreateDraw(ctx context.Context, draw service.LotteryDraw) (*service.LotteryDraw, error) {
	builder := clientFromContext(ctx, r.client).LotteryDraw.Create().
		SetNillableActivityID(draw.ActivityID).
		SetNillablePrizeID(draw.PrizeID).
		SetUserID(draw.UserID).
		SetPrizeName(draw.PrizeName).
		SetPrizeType(draw.PrizeType).
		SetNillableBalanceAmount(draw.BalanceAmount).
		SetNillableProductContent(draw.ProductContent).
		SetAttemptKey(draw.AttemptKey).
		SetAttemptSource(draw.AttemptSource)
	if !draw.CreatedAt.IsZero() {
		builder.SetCreatedAt(draw.CreatedAt)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return lotteryDrawToService(created), nil
}

func (r *lotteryRepository) ListUserDraws(ctx context.Context, userID int64, offset, limit int) ([]service.LotteryDraw, int, error) {
	client := clientFromContext(ctx, r.client)
	query := client.LotteryDraw.Query().Where(lotterydraw.UserIDEQ(userID))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	draws, err := query.Order(
		lotterydraw.ByCreatedAt(entsql.OrderDesc()),
		lotterydraw.ByID(entsql.OrderDesc()),
	).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]service.LotteryDraw, 0, len(draws))
	for _, draw := range draws {
		result = append(result, *lotteryDrawToService(draw))
	}
	return result, total, nil
}

func (r *lotteryRepository) ListAdminDraws(ctx context.Context, offset, limit int) ([]service.LotteryAdminDraw, int, error) {
	client := clientFromContext(ctx, r.client)
	query := client.LotteryDraw.Query()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	draws, err := query.Order(
		lotterydraw.ByCreatedAt(entsql.OrderDesc()),
		lotterydraw.ByID(entsql.OrderDesc()),
	).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]int64, 0, len(draws))
	seen := make(map[int64]struct{}, len(draws))
	for _, draw := range draws {
		if _, ok := seen[draw.UserID]; ok {
			continue
		}
		seen[draw.UserID] = struct{}{}
		userIDs = append(userIDs, draw.UserID)
	}
	usersByID := make(map[int64]*dbent.User, len(userIDs))
	if len(userIDs) > 0 {
		users, err := client.User.Query().Where(user.IDIn(userIDs...)).All(mixins.SkipSoftDelete(ctx))
		if err != nil {
			return nil, 0, err
		}
		for _, item := range users {
			usersByID[item.ID] = item
		}
	}

	result := make([]service.LotteryAdminDraw, 0, len(draws))
	for _, draw := range draws {
		var resolvedUser *service.User
		if entity := usersByID[draw.UserID]; entity != nil {
			resolvedUser = &service.User{
				ID: entity.ID, Email: entity.Email, Username: entity.Username, DeletedAt: entity.DeletedAt,
			}
		}
		result = append(result, service.NewLotteryAdminDraw(*lotteryDrawToService(draw), resolvedUser))
	}
	return result, total, nil
}

func (r *lotteryRepository) GrantLotteryAttempts(ctx context.Context, input service.LotteryAttemptGrantInput) (service.LotteryAttemptGrantResult, error) {
	if r == nil || r.client == nil {
		return service.LotteryAttemptGrantResult{}, service.ErrLotteryConfigurationDenied
	}
	if dbent.TxFromContext(ctx) != nil {
		return r.grantLotteryAttemptsWithClient(ctx, clientFromContext(ctx, r.client), input)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return service.LotteryAttemptGrantResult{}, fmt.Errorf("begin lottery attempt grant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	result, err := r.grantLotteryAttemptsWithClient(txCtx, tx.Client(), input)
	if err != nil {
		return service.LotteryAttemptGrantResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return service.LotteryAttemptGrantResult{}, fmt.Errorf("commit lottery attempt grant transaction: %w", err)
	}
	return result, nil
}

func (r *lotteryRepository) grantLotteryAttemptsWithClient(ctx context.Context, client *dbent.Client, input service.LotteryAttemptGrantInput) (service.LotteryAttemptGrantResult, error) {
	// A request key makes retries safe: once any rows for the key exist, return
	// the original aggregate instead of crediting the wallets again. The whole
	// batch is transactional, so a failed first attempt leaves no rows and can
	// be retried with the same key.
	existing, found, err := existingLotteryAttemptGrantResult(ctx, client, input)
	if err != nil {
		return service.LotteryAttemptGrantResult{}, fmt.Errorf("check lottery attempt grant request key: %w", err)
	}
	if found {
		return existing, nil
	}

	userIDs, err := resolveLotteryAttemptGrantUsers(ctx, client, input)
	if err != nil {
		return service.LotteryAttemptGrantResult{}, err
	}
	if !input.All && len(userIDs) > service.LotteryAttemptGrantMaxUsers {
		return service.LotteryAttemptGrantResult{}, service.ErrLotteryAttemptGrantTooLarge
	}
	result := service.LotteryAttemptGrantResult{}
	for _, userID := range userIDs {
		grant, err := client.LotteryAttemptGrant.Create().
			SetUserID(userID).
			SetRequestKey(input.RequestKey).
			SetTargetAll(input.All).
			SetAmount(input.Amount).
			SetDescription(strings.TrimSpace(input.Description)).
			SetCreatedBy(input.CreatedBy).
			Save(ctx)
		if err != nil {
			return service.LotteryAttemptGrantResult{}, fmt.Errorf("create lottery attempt grant for user %d: %w", userID, err)
		}

		if err := client.LotteryAttemptWallet.Create().
			SetUserID(userID).
			SetBalance(0).
			OnConflict(entsql.ConflictColumns(lotteryattemptwallet.FieldUserID)).
			DoNothing().Exec(ctx); err != nil {
			return service.LotteryAttemptGrantResult{}, fmt.Errorf("ensure lottery attempt wallet for user %d: %w", userID, err)
		}
		wallet, err := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			return service.LotteryAttemptGrantResult{}, fmt.Errorf("load lottery attempt wallet for user %d: %w", userID, err)
		}
		wallet, err = client.LotteryAttemptWallet.UpdateOneID(wallet.ID).AddBalance(input.Amount).Save(ctx)
		if err != nil {
			return service.LotteryAttemptGrantResult{}, fmt.Errorf("credit lottery attempt wallet for user %d: %w", userID, err)
		}
		if _, err := client.LotteryAttemptLedger.Create().
			SetUserID(userID).
			SetDelta(input.Amount).
			SetBalanceAfter(wallet.Balance).
			SetSourceType(service.LotteryAttemptLedgerSourceAdminGrant).
			SetSourceID(grant.ID).
			SetDescription(strings.TrimSpace(input.Description)).
			Save(ctx); err != nil {
			return service.LotteryAttemptGrantResult{}, fmt.Errorf("create lottery attempt grant ledger for user %d: %w", userID, err)
		}
		result.Affected++
		result.TotalGranted += input.Amount
	}
	return result, nil
}

func existingLotteryAttemptGrantResult(ctx context.Context, client *dbent.Client, input service.LotteryAttemptGrantInput) (service.LotteryAttemptGrantResult, bool, error) {
	grants, err := client.LotteryAttemptGrant.Query().
		Where(lotteryattemptgrant.RequestKeyEQ(input.RequestKey)).
		All(ctx)
	if err != nil {
		return service.LotteryAttemptGrantResult{}, false, err
	}
	if len(grants) == 0 {
		return service.LotteryAttemptGrantResult{}, false, nil
	}
	result := service.LotteryAttemptGrantResult{Affected: len(grants)}
	existingUserIDs := make(map[int64]struct{}, len(grants))
	for _, grant := range grants {
		if grant.Amount != input.Amount || grant.Description != strings.TrimSpace(input.Description) || grant.CreatedBy != input.CreatedBy || grant.TargetAll != input.All {
			return service.LotteryAttemptGrantResult{}, false, service.ErrLotteryAttemptGrantConflict
		}
		existingUserIDs[grant.UserID] = struct{}{}
		result.TotalGranted += grant.Amount
	}
	if !input.All {
		if len(existingUserIDs) != len(input.UserIDs) {
			return service.LotteryAttemptGrantResult{}, false, service.ErrLotteryAttemptGrantConflict
		}
		for _, userID := range input.UserIDs {
			if _, ok := existingUserIDs[userID]; !ok {
				return service.LotteryAttemptGrantResult{}, false, service.ErrLotteryAttemptGrantConflict
			}
		}
	}
	return result, true, nil
}

func resolveLotteryAttemptGrantUsers(ctx context.Context, client *dbent.Client, input service.LotteryAttemptGrantInput) ([]int64, error) {
	if input.All {
		return client.User.Query().Where(user.DeletedAtIsNil()).Order(user.ByID()).IDs(ctx)
	}
	if len(input.UserIDs) == 0 {
		return nil, service.ErrLotteryAttemptGrantInvalid
	}
	resolved, err := client.User.Query().Where(user.IDIn(input.UserIDs...), user.DeletedAtIsNil()).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve lottery attempt grant users: %w", err)
	}
	if len(resolved) != len(input.UserIDs) {
		return nil, service.ErrUserNotFound
	}
	return input.UserIDs, nil
}

func lotteryActivityToService(activity *dbent.LotteryActivity) *service.LotteryActivity {
	return &service.LotteryActivity{
		ID: activity.ID, Name: activity.Name, Description: activity.Description,
		Status: activity.Status, AttemptMode: activity.AttemptMode, AttemptLimit: activity.AttemptLimit,
		StartsAt: activity.StartsAt, EndsAt: activity.EndsAt, CreatedBy: activity.CreatedBy,
		CreatedAt: activity.CreatedAt, UpdatedAt: activity.UpdatedAt,
	}
}

func lotteryPrizeToService(prize *dbent.LotteryPrize) *service.LotteryPrize {
	return &service.LotteryPrize{
		ID: prize.ID, ActivityID: prize.ActivityID, Name: prize.Name, Description: prize.Description,
		Type: prize.Type, Weight: prize.Weight, BalanceAmount: prize.BalanceAmount,
		Enabled: prize.Enabled, SortOrder: prize.SortOrder, CreatedAt: prize.CreatedAt, UpdatedAt: prize.UpdatedAt,
	}
}

func lotteryPrizeItemToService(item *dbent.LotteryPrizeItem) *service.LotteryPrizeItem {
	return &service.LotteryPrizeItem{
		ID: item.ID, PrizeID: item.PrizeID, Content: item.Content, Status: item.Status,
		ClaimedBy: item.ClaimedBy, ClaimedAt: item.ClaimedAt, CreatedAt: item.CreatedAt,
	}
}

func lotteryDrawToService(draw *dbent.LotteryDraw) *service.LotteryDraw {
	return &service.LotteryDraw{
		ID: draw.ID, ActivityID: draw.ActivityID, PrizeID: draw.PrizeID, UserID: draw.UserID,
		PrizeName: draw.PrizeName, PrizeType: draw.PrizeType, BalanceAmount: draw.BalanceAmount,
		ProductContent: draw.ProductContent, AttemptKey: draw.AttemptKey,
		AttemptSource: draw.AttemptSource, CreatedAt: draw.CreatedAt,
	}
}
