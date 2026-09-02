package schema

import (
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LotteryDraw is an immutable snapshot of one completed draw and its reward.
type LotteryDraw struct {
	ent.Schema
}

func (LotteryDraw) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "lottery_draws"}}
}

func (LotteryDraw) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("activity_id").Optional().Nillable(),
		field.Int64("prize_id").Optional().Nillable(),
		field.Int64("user_id"),
		field.String("prize_name").MaxLen(120),
		field.String("prize_type").MaxLen(20),
		field.Float("balance_amount").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("product_content").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("attempt_key").MaxLen(128).Unique(),
		field.String("attempt_source").MaxLen(20).Default("activity").Validate(func(value string) error {
			switch value {
			case "activity", "wallet":
				return nil
			default:
				return fmt.Errorf("unsupported lottery attempt source %q", value)
			}
		}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryDraw) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("activity", LotteryActivity.Type).Ref("draws").Field("activity_id").Unique(),
		edge.From("prize", LotteryPrize.Type).Ref("draws").Field("prize_id").Unique(),
	}
}

func (LotteryDraw) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at").StorageKey("lottery_draws_user_created_at_idx"),
	}
}
