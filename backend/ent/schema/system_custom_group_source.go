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

// SystemCustomGroupSource records an ordered source group selected by a
// system custom subscription group.
type SystemCustomGroupSource struct{ ent.Schema }

func (SystemCustomGroupSource) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "system_custom_group_sources",
			Checks: map[string]string{
				"system_custom_group_sources_no_self_reference":    "group_id <> source_group_id",
				"system_custom_group_sources_priority_nonnegative": "priority >= 0",
			},
		},
	}
}

func (SystemCustomGroupSource) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.Int64("source_group_id"),
		field.Int("priority").Min(0),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SystemCustomGroupSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Ref("system_custom_sources").
			Field("group_id").
			Unique().
			Required(),
		edge.From("source_group", Group.Type).
			Ref("system_custom_source_references").
			Field("source_group_id").
			Unique().
			Required(),
	}
}

func (SystemCustomGroupSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "source_group_id").Unique(),
		index.Fields("group_id", "priority").Unique(),
		index.Fields("source_group_id"),
	}
}
