package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserCheckinBlacklist stores users blocked from receiving check-in rewards.
type UserCheckinBlacklist struct {
	ent.Schema
}

func (UserCheckinBlacklist) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_checkin_blacklist"},
	}
}

func (UserCheckinBlacklist) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserCheckinBlacklist) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Int64("removed_by").
			Optional().
			Nillable(),
		field.Time("removed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserCheckinBlacklist) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("checkin_blacklist_entries").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserCheckinBlacklist) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("removed_at"),
		index.Fields("user_id").
			Unique().
			Annotations(entsql.IndexWhere("removed_at IS NULL")),
	}
}
