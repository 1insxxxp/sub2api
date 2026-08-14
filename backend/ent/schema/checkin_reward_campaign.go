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

// CheckinRewardCampaign stores a Beijing-calendar campaign that replaces the
// random base reward tiers while it is enabled.
type CheckinRewardCampaign struct {
	ent.Schema
}

func (CheckinRewardCampaign) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "checkin_reward_campaigns"},
	}
}

func (CheckinRewardCampaign) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(120).
			NotEmpty(),
		field.String("status").
			MaxLen(20).
			Default(domain.CheckinRewardCampaignStatusDraft),
		field.Time("start_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Time("end_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.JSON("reward_tiers", []domain.CheckinRewardTier{}).
			Default([]domain.CheckinRewardTier{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Int64("updated_by").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CheckinRewardCampaign) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("checkins", UserCheckin.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (CheckinRewardCampaign) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "start_date", "end_date").
			StorageKey("checkin_reward_campaigns_status_dates_idx"),
	}
}
