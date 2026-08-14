package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/checkinrewardcampaign"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"

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
	ErrCheckinRewardCampaignNotFound = infraerrors.NotFound(
		"CHECKIN_REWARD_CAMPAIGN_NOT_FOUND",
		"check-in reward campaign was not found",
	)
	ErrCheckinRewardCampaignInvalidName = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_NAME",
		"check-in reward campaign name must be non-empty and contain at most 120 characters",
	)
	ErrCheckinRewardCampaignInvalidDate = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_DATE",
		"campaign dates must use YYYY-MM-DD format",
	)
	ErrCheckinRewardCampaignInvalidDateRange = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_DATE_RANGE",
		"campaign start date must not be after its end date",
	)
	ErrCheckinRewardCampaignInvalidStateTransition = infraerrors.Conflict(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION",
		"check-in reward campaign cannot perform this state transition",
	)
	ErrCheckinRewardCampaignOverlap = infraerrors.Conflict(
		"CHECKIN_REWARD_CAMPAIGN_OVERLAP",
		"check-in reward campaign overlaps an enabled campaign",
	)
	ErrCheckinRewardCampaignHistoryProtected = infraerrors.Conflict(
		"CHECKIN_REWARD_CAMPAIGN_HISTORY_PROTECTED",
		"ended check-in reward campaigns are read-only",
	)
	ErrCheckinRewardCampaignReferenced = infraerrors.Conflict(
		"CHECKIN_REWARD_CAMPAIGN_REFERENCED",
		"check-in reward campaign is referenced by check-in history",
	)
	ErrCheckinRewardCampaignInvalidLifecycleFilter = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_LIFECYCLE_FILTER",
		"invalid check-in reward campaign lifecycle filter",
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

type CreateCheckinRewardCampaignInput struct {
	Name        string              `json:"name"`
	StartDate   string              `json:"start_date"`
	EndDate     string              `json:"end_date"`
	RewardTiers []CheckinRewardTier `json:"reward_tiers"`
	AdminID     int64               `json:"-"`
}

type UpdateCheckinRewardCampaignInput struct {
	Name        string              `json:"name"`
	StartDate   string              `json:"start_date"`
	EndDate     string              `json:"end_date"`
	RewardTiers []CheckinRewardTier `json:"reward_tiers"`
	AdminID     int64               `json:"-"`
}

type EffectiveCheckinConfig struct {
	Config   *CheckinConfig
	Campaign *CheckinRewardCampaign
}

func (s *CheckinService) ListRewardCampaigns(ctx context.Context, lifecycle string) ([]CheckinRewardCampaign, error) {
	filter := strings.TrimSpace(strings.ToLower(lifecycle))
	if filter == "all" {
		filter = ""
	}
	if filter != "" && !validCheckinRewardCampaignLifecycle(filter) {
		return nil, ErrCheckinRewardCampaignInvalidLifecycleFilter.WithMetadata(map[string]string{"lifecycle": lifecycle})
	}
	today, _ := s.currentBeijingDay()
	currentDay, err := s.parseCheckinDate(today)
	if err != nil {
		return nil, err
	}
	entities, err := s.entClient.CheckinRewardCampaign.Query().
		Order(dbent.Desc(checkinrewardcampaign.FieldStartDate), dbent.Desc(checkinrewardcampaign.FieldID)).
		All(ctx)
	if err != nil {
		return nil, campaignStorageError("CHECKIN_REWARD_CAMPAIGN_LIST_FAILED", "failed to list check-in reward campaigns", err)
	}
	out := make([]CheckinRewardCampaign, 0, len(entities))
	for _, entity := range entities {
		mapped, mapErr := s.mapCheckinRewardCampaign(entity, currentDay)
		if mapErr != nil {
			return nil, mapErr
		}
		if filter == "" || mapped.LifecycleStatus == filter {
			out = append(out, *mapped)
		}
	}
	return out, nil
}

func (s *CheckinService) GetRewardCampaign(ctx context.Context, id int64) (*CheckinRewardCampaign, error) {
	entity, err := s.entClient.CheckinRewardCampaign.Get(ctx, id)
	if err != nil {
		return nil, checkinRewardCampaignLookupError(err, id)
	}
	return s.mapCheckinRewardCampaignForToday(entity)
}

func (s *CheckinService) CreateRewardCampaign(ctx context.Context, input CreateCheckinRewardCampaignInput) (*CheckinRewardCampaign, error) {
	name, startDate, endDate, tiers, err := s.normalizeCheckinRewardCampaignInput(input.Name, input.StartDate, input.EndDate, input.RewardTiers)
	if err != nil {
		return nil, err
	}
	var created *dbent.CheckinRewardCampaign
	err = s.withCheckinRewardCampaignStorageTx(ctx, func(client *dbent.Client) error {
		create := client.CheckinRewardCampaign.Create().
			SetName(name).
			SetStatus(domain.CheckinRewardCampaignStatusDraft).
			SetStartDate(startDate).
			SetEndDate(endDate).
			SetRewardTiers(tiers)
		if input.AdminID > 0 {
			create.SetCreatedBy(input.AdminID).SetUpdatedBy(input.AdminID)
		}
		var saveErr error
		created, saveErr = create.Save(ctx)
		return saveErr
	})
	if err != nil {
		return nil, campaignStorageError("CHECKIN_REWARD_CAMPAIGN_CREATE_FAILED", "failed to create check-in reward campaign", err)
	}
	return s.mapCheckinRewardCampaignForToday(created)
}

func (s *CheckinService) UpdateRewardCampaign(ctx context.Context, id int64, input UpdateCheckinRewardCampaignInput) (*CheckinRewardCampaign, error) {
	name, startDate, endDate, tiers, err := s.normalizeCheckinRewardCampaignInput(input.Name, input.StartDate, input.EndDate, input.RewardTiers)
	if err != nil {
		return nil, err
	}
	var updated *dbent.CheckinRewardCampaign
	err = s.withCheckinRewardCampaignStorageTx(ctx, func(client *dbent.Client) error {
		campaign, getErr := getCheckinRewardCampaignForMutation(ctx, client, id)
		if getErr != nil {
			return checkinRewardCampaignLookupError(getErr, id)
		}
		if campaign.Status != domain.CheckinRewardCampaignStatusDraft {
			return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		update := client.CheckinRewardCampaign.UpdateOneID(id).
			Where(checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusDraft)).
			SetName(name).
			SetStartDate(startDate).
			SetEndDate(endDate).
			SetRewardTiers(tiers)
		if input.AdminID > 0 {
			update.SetUpdatedBy(input.AdminID)
		}
		var saveErr error
		updated, saveErr = update.Save(ctx)
		return saveErr
	})
	if err != nil {
		if infraerrors.Reason(err) != "" {
			return nil, err
		}
		if dbent.IsNotFound(err) {
			return nil, s.checkCampaignDraftMutationFailure(ctx, id)
		}
		return nil, campaignStorageError("CHECKIN_REWARD_CAMPAIGN_UPDATE_FAILED", "failed to update check-in reward campaign", err)
	}
	return s.mapCheckinRewardCampaignForToday(updated)
}

func (s *CheckinService) EnableRewardCampaign(ctx context.Context, id, adminID int64) (*CheckinRewardCampaign, error) {
	var updated *dbent.CheckinRewardCampaign
	var exclusionCampaign *dbent.CheckinRewardCampaign
	err := s.withCheckinCampaignConfigTx(ctx, func(client *dbent.Client, repo SettingRepository) error {
		// The baseline read deliberately precedes every campaign row lock. This is
		// the shared advisory-lock protocol used by UpdateConfig as well.
		baseline, err := s.getCheckinConfigFromRepository(ctx, repo)
		if err != nil {
			return err
		}
		campaign, err := getCheckinRewardCampaignForMutation(ctx, client, id)
		if err != nil {
			return checkinRewardCampaignLookupError(err, id)
		}
		today, _ := s.currentBeijingDay()
		currentDay, err := s.parseCheckinDate(today)
		if err != nil {
			return err
		}
		lifecycle := deriveCheckinRewardCampaignLifecycle(campaign.Status, campaign.StartDate, campaign.EndDate, currentDay)
		campaignEnded := compareCheckinCampaignCalendarDate(campaign.EndDate, currentDay) < 0
		campaignStartsAfterToday := compareCheckinCampaignCalendarDate(campaign.StartDate, currentDay) > 0
		if campaignEnded || (lifecycle != CheckinRewardCampaignLifecycleDraft && !(lifecycle == CheckinRewardCampaignLifecycleDisabled && campaignStartsAfterToday)) {
			return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		tiers, err := normalizeCheckinRewardTiers(campaign.RewardTiers)
		if err != nil {
			return err
		}
		merged := cloneCheckinConfig(*baseline)
		merged.Tiers = tiers
		if _, err := normalizeCheckinConfig(merged); err != nil {
			return ErrCheckinCampaignIncompatibleWithConfig.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation)).WithCause(err)
		}
		conflict, err := findOverlappingEnabledCheckinRewardCampaign(ctx, client, campaign)
		if err != nil {
			return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_OVERLAP_CHECK_FAILED", "failed to check campaign date overlap", err)
		}
		if conflict != nil {
			return checkinRewardCampaignOverlapError(conflict, s.beijingLocation)
		}
		update := client.CheckinRewardCampaign.UpdateOneID(campaign.ID).
			Where(checkinrewardcampaign.StatusEQ(campaign.Status)).
			SetStatus(domain.CheckinRewardCampaignStatusEnabled).
			SetRewardTiers(tiers)
		if adminID > 0 {
			update.SetUpdatedBy(adminID)
		}
		updated, err = update.Save(ctx)
		if err != nil {
			if isCheckinRewardCampaignExclusionError(err) {
				exclusionCampaign = campaign
				return ErrCheckinRewardCampaignOverlap.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation)).WithCause(err)
			}
			if dbent.IsNotFound(err) {
				return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
			}
			return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_ENABLE_FAILED", "failed to enable check-in reward campaign", err)
		}
		return nil
	})
	if err != nil {
		// PostgreSQL aborts the transaction after an exclusion violation, so the
		// human-readable conflicting row must be loaded after the helper rolls back.
		if exclusionCampaign != nil {
			if conflict, queryErr := findOverlappingEnabledCheckinRewardCampaign(ctx, s.entClient, exclusionCampaign); queryErr == nil && conflict != nil {
				return nil, checkinRewardCampaignOverlapError(conflict, s.beijingLocation)
			}
		}
		return nil, err
	}
	return s.mapCheckinRewardCampaignForToday(updated)
}

func (s *CheckinService) DisableRewardCampaign(ctx context.Context, id, adminID int64) (*CheckinRewardCampaign, error) {
	var updated *dbent.CheckinRewardCampaign
	err := s.withCheckinCampaignConfigTx(ctx, func(client *dbent.Client, repo SettingRepository) error {
		// Keep the same advisory -> baseline -> campaign row order as Enable and UpdateConfig.
		if _, err := s.getCheckinConfigFromRepository(ctx, repo); err != nil {
			return err
		}
		campaign, err := getCheckinRewardCampaignForMutation(ctx, client, id)
		if err != nil {
			return checkinRewardCampaignLookupError(err, id)
		}
		today, _ := s.currentBeijingDay()
		currentDay, err := s.parseCheckinDate(today)
		if err != nil {
			return err
		}
		lifecycle := deriveCheckinRewardCampaignLifecycle(campaign.Status, campaign.StartDate, campaign.EndDate, currentDay)
		if lifecycle == CheckinRewardCampaignLifecycleEnded {
			return ErrCheckinRewardCampaignHistoryProtected.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		if lifecycle != CheckinRewardCampaignLifecycleUpcoming && lifecycle != CheckinRewardCampaignLifecycleActive {
			return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		update := client.CheckinRewardCampaign.UpdateOneID(campaign.ID).
			Where(checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled)).
			SetStatus(domain.CheckinRewardCampaignStatusDisabled)
		if adminID > 0 {
			update.SetUpdatedBy(adminID)
		}
		updated, err = update.Save(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
			}
			return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_DISABLE_FAILED", "failed to disable check-in reward campaign", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.mapCheckinRewardCampaignForToday(updated)
}

func (s *CheckinService) CopyRewardCampaign(ctx context.Context, id int64, name string, adminID int64) (*CheckinRewardCampaign, error) {
	name, err := normalizeCheckinRewardCampaignName(name)
	if err != nil {
		return nil, err
	}
	var copied *dbent.CheckinRewardCampaign
	err = s.withCheckinRewardCampaignStorageTx(ctx, func(client *dbent.Client) error {
		source, getErr := getCheckinRewardCampaignForMutation(ctx, client, id)
		if getErr != nil {
			return checkinRewardCampaignLookupError(getErr, id)
		}
		tiers, normalizeErr := normalizeCheckinRewardTiers(source.RewardTiers)
		if normalizeErr != nil {
			return ErrCheckinCampaignDataIntegrity.
				WithMetadata(checkinCampaignMetadata(source, s.beijingLocation)).
				WithCause(normalizeErr)
		}
		create := client.CheckinRewardCampaign.Create().
			SetName(name).
			SetStatus(domain.CheckinRewardCampaignStatusDraft).
			SetStartDate(source.StartDate).
			SetEndDate(source.EndDate).
			SetRewardTiers(tiers)
		if adminID > 0 {
			create.SetCreatedBy(adminID).SetUpdatedBy(adminID)
		}
		var createErr error
		copied, createErr = create.Save(ctx)
		return createErr
	})
	if err != nil {
		if infraerrors.Reason(err) != "" {
			return nil, err
		}
		return nil, campaignStorageError("CHECKIN_REWARD_CAMPAIGN_COPY_FAILED", "failed to copy check-in reward campaign", err)
	}
	return s.mapCheckinRewardCampaignForToday(copied)
}

func (s *CheckinService) DeleteRewardCampaign(ctx context.Context, id int64) error {
	return s.withCheckinRewardCampaignStorageTx(ctx, func(client *dbent.Client) error {
		campaign, err := getCheckinRewardCampaignForMutation(ctx, client, id)
		if err != nil {
			return checkinRewardCampaignLookupError(err, id)
		}
		if campaign.Status != domain.CheckinRewardCampaignStatusDraft {
			return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		referenced, err := client.UserCheckin.Query().Where(usercheckin.RewardCampaignIDEQ(id)).Exist(ctx)
		if err != nil {
			return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_REFERENCE_CHECK_FAILED", "failed to check campaign history references", err)
		}
		if referenced {
			return ErrCheckinRewardCampaignReferenced.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		deleted, err := client.CheckinRewardCampaign.Delete().Where(
			checkinrewardcampaign.IDEQ(id),
			checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusDraft),
		).Exec(ctx)
		if err != nil {
			if dbent.IsConstraintError(err) {
				return ErrCheckinRewardCampaignReferenced.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation)).WithCause(err)
			}
			return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_DELETE_FAILED", "failed to delete check-in reward campaign", err)
		}
		if deleted == 0 {
			return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
		}
		return nil
	})
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

	storageDay := checkinCampaignQueryDate(client, day)
	rows, err := client.CheckinRewardCampaign.Query().
		Where(
			checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled),
			checkinrewardcampaign.StartDateLTE(storageDay),
			checkinrewardcampaign.EndDateGTE(storageDay),
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
		if compareCheckinCampaignCalendarDate(currentDay, startDate) < 0 {
			return CheckinRewardCampaignLifecycleUpcoming
		}
		if compareCheckinCampaignCalendarDate(currentDay, endDate) > 0 {
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

func formatCheckinCampaignDate(value time.Time, _ *time.Location) string {
	return checkinCampaignCalendarDate(value)
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
			checkinrewardcampaign.EndDateGTE(checkinCampaignQueryDate(client, day)),
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

func validCheckinRewardCampaignLifecycle(value string) bool {
	switch value {
	case CheckinRewardCampaignLifecycleDraft,
		CheckinRewardCampaignLifecycleUpcoming,
		CheckinRewardCampaignLifecycleActive,
		CheckinRewardCampaignLifecycleEnded,
		CheckinRewardCampaignLifecycleDisabled:
		return true
	default:
		return false
	}
}

func (s *CheckinService) withCheckinRewardCampaignStorageTx(ctx context.Context, fn func(*dbent.Client) error) error {
	if s == nil || s.entClient == nil || fn == nil {
		return ErrCheckinCampaignTransactionUnavailable
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CheckinService) normalizeCheckinRewardCampaignInput(
	name, startDateValue, endDateValue string,
	tiers []CheckinRewardTier,
) (string, time.Time, time.Time, []CheckinRewardTier, error) {
	name, err := normalizeCheckinRewardCampaignName(name)
	if err != nil {
		return "", time.Time{}, time.Time{}, nil, err
	}
	startDate, err := s.parseRewardCampaignDate("start_date", startDateValue)
	if err != nil {
		return "", time.Time{}, time.Time{}, nil, err
	}
	endDate, err := s.parseRewardCampaignDate("end_date", endDateValue)
	if err != nil {
		return "", time.Time{}, time.Time{}, nil, err
	}
	if startDate.After(endDate) {
		return "", time.Time{}, time.Time{}, nil, ErrCheckinRewardCampaignInvalidDateRange.WithMetadata(map[string]string{
			"start_date": startDateValue,
			"end_date":   endDateValue,
		})
	}
	normalizedTiers, err := normalizeCheckinRewardTiers(tiers)
	if err != nil {
		return "", time.Time{}, time.Time{}, nil, err
	}
	return name, startDate, endDate, normalizedTiers, nil
}

func normalizeCheckinRewardCampaignName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", ErrCheckinRewardCampaignInvalidName
	}
	return name, nil
}

func checkinCampaignCalendarDate(value time.Time) string {
	return value.Format("2006-01-02")
}

func compareCheckinCampaignCalendarDate(left, right time.Time) int {
	return strings.Compare(checkinCampaignCalendarDate(left), checkinCampaignCalendarDate(right))
}

func checkinCampaignStorageDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func checkinCampaignQueryDate(client *dbent.Client, value time.Time) time.Time {
	if client != nil && client.Driver().Dialect() == dialect.Postgres {
		return checkinCampaignStorageDate(value)
	}
	return value
}

func (s *CheckinService) parseRewardCampaignDate(field, value string) (time.Time, error) {
	parsed, err := s.parseCheckinDate(value)
	if err != nil {
		return time.Time{}, ErrCheckinRewardCampaignInvalidDate.WithMetadata(map[string]string{
			"field": field,
			"value": value,
		})
	}
	return parsed, nil
}

func (s *CheckinService) mapCheckinRewardCampaignForToday(entity *dbent.CheckinRewardCampaign) (*CheckinRewardCampaign, error) {
	today, _ := s.currentBeijingDay()
	currentDay, err := s.parseCheckinDate(today)
	if err != nil {
		return nil, err
	}
	return s.mapCheckinRewardCampaign(entity, currentDay)
}

func (s *CheckinService) mapCheckinRewardCampaign(entity *dbent.CheckinRewardCampaign, currentDay time.Time) (*CheckinRewardCampaign, error) {
	if entity == nil {
		return nil, ErrCheckinCampaignDataIntegrity
	}
	tiers, err := normalizeCheckinRewardTiers(entity.RewardTiers)
	if err != nil {
		return nil, ErrCheckinCampaignDataIntegrity.
			WithMetadata(checkinCampaignMetadata(entity, s.beijingLocation)).
			WithCause(err)
	}
	return checkinRewardCampaignFromEntity(entity, tiers, currentDay, s.beijingLocation), nil
}

func checkinRewardCampaignLookupError(err error, id int64) error {
	if dbent.IsNotFound(err) {
		return ErrCheckinRewardCampaignNotFound.WithMetadata(map[string]string{"campaign_id": strconv.FormatInt(id, 10)})
	}
	return campaignStorageError("CHECKIN_REWARD_CAMPAIGN_GET_FAILED", "failed to get check-in reward campaign", err)
}

func campaignStorageError(reason, message string, cause error) error {
	return infraerrors.InternalServer(reason, message).WithCause(cause)
}

func (s *CheckinService) checkCampaignDraftMutationFailure(ctx context.Context, id int64) error {
	campaign, err := s.entClient.CheckinRewardCampaign.Get(ctx, id)
	if err != nil {
		return checkinRewardCampaignLookupError(err, id)
	}
	return ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(checkinCampaignMetadata(campaign, s.beijingLocation))
}

func getCheckinRewardCampaignForMutation(ctx context.Context, client *dbent.Client, id int64) (*dbent.CheckinRewardCampaign, error) {
	query := client.CheckinRewardCampaign.Query().Where(checkinrewardcampaign.IDEQ(id))
	if client.Driver().Dialect() == dialect.Postgres {
		query.ForUpdate()
	}
	return query.Only(ctx)
}

func findOverlappingEnabledCheckinRewardCampaign(
	ctx context.Context,
	client *dbent.Client,
	campaign *dbent.CheckinRewardCampaign,
) (*dbent.CheckinRewardCampaign, error) {
	conflict, err := client.CheckinRewardCampaign.Query().Where(
		checkinrewardcampaign.IDNEQ(campaign.ID),
		checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled),
		checkinrewardcampaign.StartDateLTE(campaign.EndDate),
		checkinrewardcampaign.EndDateGTE(campaign.StartDate),
	).Order(dbent.Asc(checkinrewardcampaign.FieldStartDate), dbent.Asc(checkinrewardcampaign.FieldID)).First(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	return conflict, err
}

func checkinRewardCampaignOverlapError(conflict *dbent.CheckinRewardCampaign, loc *time.Location) error {
	return ErrCheckinRewardCampaignOverlap.WithMetadata(map[string]string{
		"conflict_campaign_id":   strconv.FormatInt(conflict.ID, 10),
		"conflict_campaign_name": conflict.Name,
		"conflict_start_date":    formatCheckinCampaignDate(conflict.StartDate, loc),
		"conflict_end_date":      formatCheckinCampaignDate(conflict.EndDate, loc),
	})
}

func isCheckinRewardCampaignExclusionError(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr != nil {
		return pqErr.Code == "23P01" || pqErr.Constraint == "checkin_reward_campaigns_enabled_dates_excl"
	}
	return strings.Contains(err.Error(), "checkin_reward_campaigns_enabled_dates_excl")
}
