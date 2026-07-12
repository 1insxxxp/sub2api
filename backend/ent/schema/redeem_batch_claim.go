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

// RedeemBatchClaim records a user's durable claim on a restricted redeem batch.
// It deliberately has no redeem-code foreign key so hard-deleting a code cannot
// restore eligibility for another code in the same activity batch.
type RedeemBatchClaim struct {
	ent.Schema
}

func (RedeemBatchClaim) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_batch_claims"},
	}
}

func (RedeemBatchClaim) Fields() []ent.Field {
	return []ent.Field{
		field.String("batch_id").
			NotEmpty().
			MaxLen(64),
		field.Int64("user_id"),
		field.Int64("redeem_code_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RedeemBatchClaim) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("batch_id", "user_id").Unique(),
		index.Fields("user_id"),
		index.Fields("redeem_code_id"),
	}
}
