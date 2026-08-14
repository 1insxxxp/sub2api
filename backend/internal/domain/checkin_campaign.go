package domain

const (
	CheckinRewardCampaignStatusDraft    = "draft"
	CheckinRewardCampaignStatusEnabled  = "enabled"
	CheckinRewardCampaignStatusDisabled = "disabled"
)

type CheckinRewardTier struct {
	Amount      float64 `json:"amount"`
	Probability float64 `json:"probability"`
	SortOrder   int     `json:"sort_order"`
}
