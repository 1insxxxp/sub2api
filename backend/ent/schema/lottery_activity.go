package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LotteryActivity stores the single globally configurable lottery activity and
// its per-user attempt policy.
type LotteryActivity struct {
	ent.Schema
}

func (LotteryActivity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_activities",
			Checks: map[string]string{
				"lottery_activities_status_check":        "status IN ('draft', 'active', 'disabled', 'ended')",
				"lottery_activities_attempt_mode_check":  "attempt_mode IN ('daily', 'total')",
				"lottery_activities_attempt_limit_check": "attempt_limit > 0",
				"lottery_activities_dates_check":         "starts_at IS NULL OR ends_at IS NULL OR starts_at <= ends_at",
			},
		},
	}
}

func (LotteryActivity) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(120).NotEmpty(),
		field.String("description").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").MaxLen(20).Default("draft"),
		field.String("attempt_mode").MaxLen(20).Default("daily"),
		field.Int("attempt_limit").Default(1),
		field.Time("starts_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ends_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("created_by").Optional().Nillable(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryActivity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("prizes", LotteryPrize.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("draws", LotteryDraw.Type).Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}

func (LotteryActivity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").StorageKey("lottery_activities_status_idx"),
		index.Fields("attempt_mode").StorageKey("lottery_activities_attempt_mode_idx"),
	}
}
