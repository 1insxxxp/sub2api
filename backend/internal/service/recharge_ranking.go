package service

import (
	"context"
	"time"
)

// RechargeRankingItem is the positive balance credited to one user, grouped by
// the source of the credit. Amounts are account balance units.
type RechargeRankingItem struct {
	UserID          int64      `json:"user_id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	InviterID       *int64     `json:"inviter_id"`
	InviterEmail    string     `json:"inviter_email"`
	InviterUsername string     `json:"inviter_username"`
	Balance         float64    `json:"balance"`
	TotalAmount     float64    `json:"total_amount"`
	OnlineAmount    float64    `json:"online_amount"`
	RedeemAmount    float64    `json:"redeem_amount"`
	AffiliateAmount float64    `json:"affiliate_amount"`
	AdminAmount     float64    `json:"admin_amount"`
	RewardAmount    float64    `json:"reward_amount"`
	RefundAmount    float64    `json:"refund_amount"`
	OtherAmount     float64    `json:"other_amount"`
	RechargeCount   int64      `json:"recharge_count"` // Online, redeem and positive admin credits only.
	LastRechargedAt *time.Time `json:"last_recharged_at,omitempty"`
}

type RechargeRankingSummary struct {
	TotalAmount     float64 `json:"total_amount"`
	OnlineAmount    float64 `json:"online_amount"`
	RedeemAmount    float64 `json:"redeem_amount"`
	AffiliateAmount float64 `json:"affiliate_amount"`
	AdminAmount     float64 `json:"admin_amount"`
	RewardAmount    float64 `json:"reward_amount"`
	RefundAmount    float64 `json:"refund_amount"`
	OtherAmount     float64 `json:"other_amount"`
	RechargeCount   int64   `json:"recharge_count"`
	RechargeUsers   int64   `json:"recharge_users"`
}

type RechargeRankingResponse struct {
	Items   []RechargeRankingItem  `json:"items"`
	Total   int64                  `json:"total"`
	Summary RechargeRankingSummary `json:"summary"`
}

// RechargeRankingRepository is implemented by the usage repository because it
// already owns the shared SQL connection and admin usage aggregation queries.
type RechargeRankingRepository interface {
	GetRechargeRanking(ctx context.Context, startTime, endTime time.Time, userID int64, source, sortBy, sortOrder string, offset, limit int) (*RechargeRankingResponse, error)
}
