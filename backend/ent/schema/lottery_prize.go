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

// LotteryPrize is one weighted result in a lottery activity.
type LotteryPrize struct {
	ent.Schema
}

func (LotteryPrize) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_prizes",
			Checks: map[string]string{
				"lottery_prizes_type_check":    "type IN ('balance', 'product')",
				"lottery_prizes_weight_check":  "weight > 0",
				"lottery_prizes_balance_check": "type <> 'balance' OR (balance_amount IS NOT NULL AND balance_amount > 0)",
			},
		},
	}
}

func (LotteryPrize) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("activity_id"),
		field.String("name").MaxLen(120).NotEmpty(),
		field.String("description").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("type").MaxLen(20),
		field.Int("weight").Default(1),
		field.Float("balance_amount").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Bool("enabled").Default(true),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryPrize) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("activity", LotteryActivity.Type).Ref("prizes").Field("activity_id").Unique().Required(),
		edge.To("items", LotteryPrizeItem.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("draws", LotteryDraw.Type).Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}

func (LotteryPrize) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("activity_id", "enabled", "sort_order").StorageKey("lottery_prizes_activity_enabled_sort_idx"),
	}
}
