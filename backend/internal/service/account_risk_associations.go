package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

//go:embed queries/account_risk_associations.sql
var listRiskAssociationSQL string

type RiskAssociationFilters struct{ Search, Reason, Status, StartDate, EndDate string }
type RiskAssociationEvidence struct {
	Reason      string    `json:"reason"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
type RiskAssociation struct {
	UserEmail           string                    `json:"user_email"`
	RelatedEmail        string                    `json:"related_email"`
	InviterEmail        string                    `json:"inviter_email"`
	Reasons             []string                  `json:"reasons"`
	Evidence            []RiskAssociationEvidence `json:"evidence"`
	Status              string                    `json:"status"`
	HighRisk            bool                      `json:"high_risk"`
	FirstSeenAt         time.Time                 `json:"first_seen_at"`
	LastSeenAt          time.Time                 `json:"last_seen_at"`
	CheckinCount        int64                     `json:"checkin_count"`
	RelatedCheckinCount int64                     `json:"related_checkin_count"`
	RewardAmount        float64                   `json:"reward_amount"`
	RelatedRewardAmount float64                   `json:"related_reward_amount"`
	UserAgent           string                    `json:"user_agent"`
	RelatedUserAgent    string                    `json:"related_user_agent"`
}

func (s *CheckinService) ListRiskAssociations(ctx context.Context, page, pageSize int, f RiskAssociationFilters) ([]RiskAssociation, int64, error) {
	invalid := infraerrors.BadRequest("INVALID_RISK_ASSOCIATION_FILTER", "invalid association filter")
	if page < 1 || pageSize < 1 || pageSize > 100 || page > 1000000 || len(f.Search) > 255 {
		return nil, 0, invalid
	}
	switch f.Status {
	case "", "none", "pending", "approved", "rejected", "observed":
	default:
		return nil, 0, invalid
	}
	switch f.Reason {
	case "", "registration_ip", "login_ip", "usage_ip", "checkin_ip", "payment_ip", "browser", "invitation":
	default:
		return nil, 0, invalid
	}
	var start, end *time.Time
	if f.StartDate != "" {
		v, err := time.ParseInLocation(time.DateOnly, f.StartDate, s.beijingLocation)
		if err != nil {
			return nil, 0, invalid
		}
		start = &v
	}
	if f.EndDate != "" {
		v, err := time.ParseInLocation(time.DateOnly, f.EndDate, s.beijingLocation)
		if err != nil {
			return nil, 0, invalid
		}
		v = v.AddDate(0, 0, 1)
		end = &v
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, 0, invalid
	}
	now := s.now()
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := s.entClient.QueryContext(queryCtx, listRiskAssociationSQL, now.AddDate(0, 0, -90), now.AddDate(0, 0, -30), strings.TrimSpace(f.Search), f.Reason, f.Status, start, end, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	var raw []byte
	if err := scanRiskRow(rows, &total, &raw); err != nil {
		return nil, 0, err
	}
	var items []RiskAssociation
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
