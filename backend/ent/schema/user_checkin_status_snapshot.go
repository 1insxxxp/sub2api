package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserCheckinStatusSnapshot stores the latest check-in streak state for a user.
type UserCheckinStatusSnapshot struct {
	ent.Schema
}

func (UserCheckinStatusSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_checkin_status_snapshots"},
	}
}

func (UserCheckinStatusSnapshot) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserCheckinStatusSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int("current_streak").
			Default(1),
		field.String("last_checkin_date").
			MaxLen(10).
			NotEmpty(),
		field.Int("lifetime_checkin_days").
			Default(1),
	}
}

func (UserCheckinStatusSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("checkin_status_snapshots").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserCheckinStatusSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("last_checkin_date"),
	}
}
