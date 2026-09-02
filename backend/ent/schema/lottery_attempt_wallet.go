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

// LotteryAttemptWallet stores a user's persistent promotional draw balance.
type LotteryAttemptWallet struct {
	ent.Schema
}

func (LotteryAttemptWallet) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_attempt_wallets",
			Checks: map[string]string{
				"lottery_attempt_wallets_balance_check": "balance >= 0",
			},
		},
	}
}

func (LotteryAttemptWallet) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int("balance").Default(0).NonNegative(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryAttemptWallet) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("lottery_attempt_wallet").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (LotteryAttemptWallet) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
	}
}
