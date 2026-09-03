package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// LotteryAttemptGrant records one administrator-issued reward-wallet credit.
type LotteryAttemptGrant struct {
	ent.Schema
}

func (LotteryAttemptGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_attempt_grants",
			Checks: map[string]string{
				"lottery_attempt_grants_amount_check": "amount > 0",
			},
		},
	}
}

func (LotteryAttemptGrant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int("amount").Positive(),
		field.String("description").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("created_by").Positive(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
