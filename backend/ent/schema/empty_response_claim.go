package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EmptyResponseClaim records evaluation, review, and compensation ledger data.
type EmptyResponseClaim struct{ ent.Schema }

func (EmptyResponseClaim) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "empty_response_claims"}}
}

func (EmptyResponseClaim) Fields() []ent.Field {
	money := func(name string) ent.Field {
		return field.Float(name).Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"})
	}
	return []ent.Field{
		field.Int64("usage_log_id"),
		field.Int64("outcome_id").Optional().Nillable(),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("account_id"),
		field.Int64("group_id").Optional().Nillable(),
		field.Int64("subscription_id").Optional().Nillable(),
		field.String("status").MaxLen(24).Default("evaluating"),
		field.String("reason_code").MaxLen(64).Default(""),
		field.String("user_reason").MaxLen(255).Default(""),
		field.String("admin_note").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		money("original_actual_cost"),
		money("balance_refund"),
		money("subscription_refund"),
		money("api_key_quota_refund"),
		field.JSON("evidence", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("rule_version").Default(1),
		field.Int64("reviewed_by").Optional().Nillable(),
		field.Time("reviewed_at").Optional().Nillable(),
		field.Time("compensated_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (EmptyResponseClaim) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("usage_log_id").Unique(),
		index.Fields("status", "created_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("account_id", "created_at"),
	}
}
