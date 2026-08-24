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

// LotteryPrizeItem is one single-use custom product payload.
type LotteryPrizeItem struct {
	ent.Schema
}

func (LotteryPrizeItem) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_prize_items",
			Checks: map[string]string{
				"lottery_prize_items_status_check": "status IN ('available', 'claimed')",
			},
		},
	}
}

func (LotteryPrizeItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("prize_id"),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").MaxLen(20).Default("available"),
		field.Int64("claimed_by").Optional().Nillable(),
		field.Time("claimed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryPrizeItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("prize", LotteryPrize.Type).Ref("items").Field("prize_id").Unique().Required(),
	}
}

func (LotteryPrizeItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("prize_id", "status").StorageKey("lottery_prize_items_prize_status_idx"),
	}
}
