package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserCheckin records one daily check-in reward for one user.
type UserCheckin struct {
	ent.Schema
}

func (UserCheckin) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_checkins"},
	}
}

func (UserCheckin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("checkin_date").
			MaxLen(10).
			NotEmpty(),
		field.Float("reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("balance_before").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("balance_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("streak_day").
			Default(1),
		field.Float("base_reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("bonus_reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int("lottery_attempts_reward").
			Default(0).
			NonNegative(),
		field.Float("total_reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("previous_day_usage_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("usage_rebate_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("reward_cap_adjustment").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int64("reward_campaign_id").
			Optional().
			Nillable(),
		field.String("reward_campaign_name").
			MaxLen(120).
			Default(""),
		field.JSON("reward_campaign_tiers_snapshot", []domain.CheckinRewardTier{}).
			Default([]domain.CheckinRewardTier{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (UserCheckin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("checkins").
			Field("user_id").
			Unique().
			Required(),
		edge.From("reward_campaign", CheckinRewardCampaign.Type).
			Ref("checkins").
			Field("reward_campaign_id").
			Unique(),
	}
}

func (UserCheckin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "checkin_date").Unique(),
		index.Fields("checkin_date"),
		index.Fields("user_id"),
		index.Fields("reward_campaign_id").
			StorageKey("user_checkins_reward_campaign_id_idx"),
	}
}
