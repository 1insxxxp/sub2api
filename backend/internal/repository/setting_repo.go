package repository

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

const checkinCampaignConfigAdvisoryLockKey int64 = 0x43484b43414d50

type settingRepository struct {
	client *ent.Client
}

func NewSettingRepository(client *ent.Client) service.SettingRepository {
	return &settingRepository{client: client}
}

var _ service.CheckinCampaignConfigTransactionRepository = (*settingRepository)(nil)
var _ service.CheckinCampaignConfigReadTransactionRepository = (*settingRepository)(nil)

func (r *settingRepository) WithCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(client *ent.Client, repo service.SettingRepository) error,
) error {
	if r == nil || r.client == nil || r.client.Driver().Dialect() != dialect.Postgres {
		return fmt.Errorf("check-in campaign config transaction requires PostgreSQL")
	}
	if fn == nil {
		return fmt.Errorf("check-in campaign config transaction callback is nil")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin check-in campaign config transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var rows entsql.Rows
	if err := tx.Client().Driver().Query(
		ctx,
		"SELECT pg_advisory_xact_lock($1)",
		[]any{checkinCampaignConfigAdvisoryLockKey},
		&rows,
	); err != nil {
		return fmt.Errorf("acquire check-in campaign config advisory lock: %w", err)
	}
	_ = rows.Close()

	txClient := tx.Client()
	txRepo := &settingRepository{client: txClient}
	if err := fn(txClient, txRepo); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit check-in campaign config transaction: %w", err)
	}
	return nil
}

func (r *settingRepository) WithCheckinCampaignConfigReadTx(
	ctx context.Context,
	fn func(client *ent.Client, repo service.SettingRepository) error,
) error {
	if r == nil || r.client == nil || r.client.Driver().Dialect() != dialect.Postgres {
		return fmt.Errorf("check-in campaign config read transaction requires PostgreSQL")
	}
	if fn == nil {
		return fmt.Errorf("check-in campaign config read transaction callback is nil")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin check-in campaign config read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var rows entsql.Rows
	if err := tx.Client().Driver().Query(
		ctx,
		"SELECT pg_advisory_xact_lock_shared($1)",
		[]any{checkinCampaignConfigAdvisoryLockKey},
		&rows,
	); err != nil {
		return fmt.Errorf("acquire shared check-in campaign config advisory lock: %w", err)
	}
	_ = rows.Close()

	txClient := tx.Client()
	txRepo := &settingRepository{client: txClient}
	if err := fn(txClient, txRepo); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit check-in campaign config read transaction: %w", err)
	}
	return nil
}

func (r *settingRepository) Get(ctx context.Context, key string) (*service.Setting, error) {
	m, err := r.client.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrSettingNotFound
		}
		return nil, err
	}
	return &service.Setting{
		ID:        m.ID,
		Key:       m.Key,
		Value:     m.Value,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (r *settingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *settingRepository) Set(ctx context.Context, key, value string) error {
	now := time.Now()
	return r.client.Setting.
		Create().
		SetKey(key).
		SetValue(value).
		SetUpdatedAt(now).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	settings, err := r.client.Setting.Query().Where(setting.KeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	return setMultiple(ctx, r.client, settings)
}

func setMultiple(ctx context.Context, client *ent.Client, settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}

	now := time.Now()
	builders := make([]*ent.SettingCreate, 0, len(settings))
	for key, value := range settings {
		builders = append(builders, client.Setting.Create().SetKey(key).SetValue(value).SetUpdatedAt(now))
	}
	return client.Setting.
		CreateBulk(builders...).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) SetMultipleWithAffiliateQualificationReconcile(
	ctx context.Context,
	settings map[string]string,
	defaultQualificationAmount float64,
	reconcileUpdates map[string]string,
) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := tx.Setting.Query().
		Where(setting.KeyEQ(service.SettingKeyAffiliateQualificationAmount)).
		ForUpdate().
		Only(ctx)
	oldAmount := defaultQualificationAmount
	if err == nil {
		oldAmount, err = strconv.ParseFloat(stored.Value, 64)
		if err != nil {
			oldAmount = math.NaN()
		}
	} else if !ent.IsNotFound(err) {
		return err
	}

	newAmount, err := strconv.ParseFloat(settings[service.SettingKeyAffiliateQualificationAmount], 64)
	if err != nil {
		return err
	}
	toWrite := settings
	if oldAmount != newAmount {
		toWrite = make(map[string]string, len(settings)+len(reconcileUpdates))
		for key, value := range settings {
			toWrite[key] = value
		}
		for key, value := range reconcileUpdates {
			toWrite[key] = value
		}
	}
	if err := setMultiple(ctx, tx.Client(), toWrite); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *settingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	settings, err := r.client.Setting.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx)
	return err
}
