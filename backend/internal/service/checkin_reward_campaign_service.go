package service

import (
	"context"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/checkinrewardcampaign"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	"entgo.io/ent/dialect"
)

var (
	ErrInvalidCheckinDate = infraerrors.BadRequest(
		"INVALID_CHECKIN_DATE",
		"check-in date must use YYYY-MM-DD format",
	)
	ErrCheckinCampaignDataIntegrity = infraerrors.InternalServer(
		"CHECKIN_CAMPAIGN_DATA_INTEGRITY",
		"check-in reward campaign data is inconsistent",
	)
	ErrCheckinCampaignIncompatibleWithConfig = infraerrors.Conflict(
		"CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG",
		"check-in configuration is incompatible with an enabled reward campaign",
	)
	ErrCheckinCampaignTransactionUnavailable = infraerrors.InternalServer(
		"CHECKIN_CAMPAIGN_TRANSACTION_UNAVAILABLE",
		"check-in campaign configuration transaction is unavailable",
	)
)

const (
	CheckinRewardCampaignLifecycleDraft    = "draft"
	CheckinRewardCampaignLifecycleUpcoming = "upcoming"
	CheckinRewardCampaignLifecycleActive   = "active"
	CheckinRewardCampaignLifecycleEnded    = "ended"
	CheckinRewardCampaignLifecycleDisabled = "disabled"
)

// CheckinRewardCampaign is the service-facing representation of a scheduled
// base-reward campaign. LifecycleStatus is populated by lifecycle operations.
type CheckinRewardCampaign struct {
	ID               int64                `json:"id"`
	Name             string               `json:"name"`
	Status           string               `json:"status"`
	LifecycleStatus  string               `json:"lifecycle_status"`
	StartDate        string               `json:"start_date"`
	EndDate          string               `json:"end_date"`
	RewardTiers      []CheckinRewardTier  `json:"reward_tiers"`
	ProbabilityTotal float64              `json:"probability_total"`
	Preview          CheckinRewardPreview `json:"preview"`
	CreatedBy        *int64               `json:"created_by,omitempty"`
	UpdatedBy        *int64               `json:"updated_by,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type EffectiveCheckinConfig struct {
	Config   *CheckinConfig
	Campaign *CheckinRewardCampaign
}

func (s *CheckinService) resolveEffectiveCheckinConfig(
	ctx context.Context,
	client *dbent.Client,
	checkinDate string,
	baseline *CheckinConfig,
) (*EffectiveCheckinConfig, error) {
	day, err := s.parseCheckinDate(checkinDate)
	if err != nil {
		return nil, err
	}
	if client == nil || baseline == nil {
		return nil, ErrCheckinCampaignDataIntegrity
	}

	rows, err := client.CheckinRewardCampaign.Query().
		Where(
			checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled),
			checkinrewardcampaign.StartDateLTE(day),
			checkinrewardcampaign.EndDateGTE(day),
		).
		Limit(2).
		All(ctx)
	if err != nil {
		return nil, infraerrors.InternalServer(
			"CHECKIN_CAMPAIGN_RESOLVE_FAILED",
			"failed to resolve the effective check-in reward campaign",
		).WithCause(err)
	}
	if len(rows) > 1 {
		return nil, ErrCheckinCampaignDataIntegrity.WithMetadata(map[string]string{
			"checkin_date": checkinDate,
		})
	}

	merged := cloneCheckinConfig(*baseline)
	if len(rows) == 1 {
		merged.Tiers = append([]CheckinRewardTier(nil), rows[0].RewardTiers...)
	}
	normalized, err := normalizeCheckinConfig(merged)
	if err != nil {
		if len(rows) == 1 {
			return nil, ErrCheckinCampaignDataIntegrity.WithMetadata(checkinCampaignMetadata(rows[0], s.beijingLocation)).WithCause(err)
		}
		return nil, err
	}

	result := &EffectiveCheckinConfig{Config: normalized}
	if len(rows) == 1 {
		result.Campaign = checkinRewardCampaignFromEntity(rows[0], normalized.Tiers, day, s.beijingLocation)
	}
	return result, nil
}

func (s *CheckinService) parseCheckinDate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, s.beijingLocation)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, ErrInvalidCheckinDate.WithMetadata(map[string]string{"checkin_date": value})
	}
	return parsed, nil
}

func cloneCheckinConfig(cfg CheckinConfig) CheckinConfig {
	cloned := cfg
	cloned.Tiers = append([]CheckinRewardTier(nil), cfg.Tiers...)
	cloned.StreakRules = append([]CheckinStreakRule(nil), cfg.StreakRules...)
	return cloned
}

func checkinRewardCampaignFromEntity(
	entity *dbent.CheckinRewardCampaign,
	tiers []CheckinRewardTier,
	currentDay time.Time,
	loc *time.Location,
) *CheckinRewardCampaign {
	return &CheckinRewardCampaign{
		ID:               entity.ID,
		Name:             entity.Name,
		Status:           entity.Status,
		LifecycleStatus:  deriveCheckinRewardCampaignLifecycle(entity.Status, entity.StartDate, entity.EndDate, currentDay),
		StartDate:        formatCheckinCampaignDate(entity.StartDate, loc),
		EndDate:          formatCheckinCampaignDate(entity.EndDate, loc),
		RewardTiers:      append([]CheckinRewardTier(nil), tiers...),
		ProbabilityTotal: checkinProbabilityTotal(tiers),
		Preview:          checkinRewardPreview(tiers),
		CreatedBy:        cloneOptionalInt64(entity.CreatedBy),
		UpdatedBy:        cloneOptionalInt64(entity.UpdatedBy),
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func deriveCheckinRewardCampaignLifecycle(status string, startDate, endDate, currentDay time.Time) string {
	switch status {
	case domain.CheckinRewardCampaignStatusDraft:
		return CheckinRewardCampaignLifecycleDraft
	case domain.CheckinRewardCampaignStatusDisabled:
		return CheckinRewardCampaignLifecycleDisabled
	case domain.CheckinRewardCampaignStatusEnabled:
		day := currentDay.Format("2006-01-02")
		if day < startDate.Format("2006-01-02") {
			return CheckinRewardCampaignLifecycleUpcoming
		}
		if day > endDate.Format("2006-01-02") {
			return CheckinRewardCampaignLifecycleEnded
		}
		return CheckinRewardCampaignLifecycleActive
	default:
		return status
	}
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func formatCheckinCampaignDate(value time.Time, loc *time.Location) string {
	if loc != nil {
		value = value.In(loc)
	}
	return value.Format("2006-01-02")
}

func checkinCampaignMetadata(entity *dbent.CheckinRewardCampaign, loc *time.Location) map[string]string {
	return map[string]string{
		"campaign_id":         strconv.FormatInt(entity.ID, 10),
		"campaign_name":       entity.Name,
		"campaign_start_date": formatCheckinCampaignDate(entity.StartDate, loc),
		"campaign_end_date":   formatCheckinCampaignDate(entity.EndDate, loc),
	}
}

func (s *CheckinService) withCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(client *dbent.Client, repo SettingRepository) error,
) error {
	if s == nil || s.entClient == nil || s.settingRepo == nil || fn == nil {
		return ErrCheckinCampaignTransactionUnavailable
	}
	if s.entClient.Driver().Dialect() == dialect.Postgres {
		txRepo, ok := s.settingRepo.(CheckinCampaignConfigTransactionRepository)
		if !ok {
			return ErrCheckinCampaignTransactionUnavailable
		}
		return txRepo.WithCheckinCampaignConfigTx(ctx, fn)
	}

	s.checkinCampaignMu.Lock()
	defer s.checkinCampaignMu.Unlock()
	return fn(s.entClient, s.settingRepo)
}

func (s *CheckinService) validateConfigAgainstEnabledCampaigns(ctx context.Context, client *dbent.Client, baseline *CheckinConfig) error {
	if client == nil {
		return infraerrors.InternalServer(
			"CHECKIN_CAMPAIGN_COMPATIBILITY_CHECK_FAILED",
			"check-in campaign storage is not configured",
		)
	}
	today, _ := s.currentBeijingDay()
	day, err := s.parseCheckinDate(today)
	if err != nil {
		return err
	}
	campaigns, err := client.CheckinRewardCampaign.Query().
		Where(
			checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled),
			checkinrewardcampaign.EndDateGTE(day),
		).
		All(ctx)
	if err != nil {
		return infraerrors.InternalServer(
			"CHECKIN_CAMPAIGN_COMPATIBILITY_CHECK_FAILED",
			"failed to validate enabled check-in reward campaigns",
		).WithCause(err)
	}

	for _, campaign := range campaigns {
		normalizedTiers, normalizeTiersErr := normalizeCheckinRewardTiers(campaign.RewardTiers)
		if normalizeTiersErr != nil {
			return ErrCheckinCampaignDataIntegrity.
				WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation)).
				WithCause(normalizeTiersErr)
		}
		merged := cloneCheckinConfig(*baseline)
		merged.Tiers = normalizedTiers
		if _, normalizeErr := normalizeCheckinConfig(merged); normalizeErr != nil {
			return ErrCheckinCampaignIncompatibleWithConfig.
				WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation)).
				WithCause(normalizeErr)
		}
	}
	return nil
}
