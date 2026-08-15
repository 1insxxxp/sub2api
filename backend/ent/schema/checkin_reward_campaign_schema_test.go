package schema

import (
	"testing"

	"entgo.io/ent/dialect/entsql"
	"github.com/stretchr/testify/require"
)

func TestCheckinRewardCampaignSchemaConstraints(t *testing.T) {
	var annotation *entsql.Annotation
	for _, raw := range (CheckinRewardCampaign{}).Annotations() {
		if value, ok := raw.(entsql.Annotation); ok {
			annotation = &value
			break
		}
	}
	require.NotNil(t, annotation)
	require.Equal(t,
		"status IN ('draft', 'enabled', 'disabled')",
		annotation.Checks["checkin_reward_campaigns_status_check"],
	)
	require.Equal(t,
		"start_date <= end_date",
		annotation.Checks["checkin_reward_campaigns_date_order_check"],
	)

	var validators []func(string) error
	for _, entField := range (CheckinRewardCampaign{}).Fields() {
		descriptor := entField.Descriptor()
		if descriptor.Name != "status" {
			continue
		}
		for _, raw := range descriptor.Validators {
			if validator, ok := raw.(func(string) error); ok {
				validators = append(validators, validator)
			}
		}
	}
	require.NotEmpty(t, validators)
	for _, value := range []string{"draft", "enabled", "disabled"} {
		for _, validator := range validators {
			require.NoError(t, validator(value))
		}
	}
	var invalidErr error
	for _, validator := range validators {
		if err := validator("invalid"); err != nil {
			invalidErr = err
			break
		}
	}
	require.Error(t, invalidErr)

	var hasRewardCampaignIDIndex bool
	for _, schemaIndex := range (UserCheckin{}).Indexes() {
		descriptor := schemaIndex.Descriptor()
		if len(descriptor.Fields) == 1 && descriptor.Fields[0] == "reward_campaign_id" {
			require.Equal(t, "user_checkins_reward_campaign_id_idx", descriptor.StorageKey)
			hasRewardCampaignIDIndex = true
		}
	}
	require.True(t, hasRewardCampaignIDIndex)
}
