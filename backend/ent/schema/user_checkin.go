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

// UserCheckin records one successful daily check-in.
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
		field.Float("total_reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
	}
}

func (UserCheckin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("checkins").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserCheckin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "checkin_date").Unique(),
		index.Fields("user_id"),
		index.Fields("checkin_date"),
	}
}
