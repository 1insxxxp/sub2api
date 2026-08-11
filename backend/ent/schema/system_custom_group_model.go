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

// SystemCustomGroupModel maps a public model on a subscription group to a
// concrete source group and upstream model.
type SystemCustomGroupModel struct{ ent.Schema }

func (SystemCustomGroupModel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "system_custom_group_models",
			Checks: map[string]string{
				"system_custom_group_models_no_self_reference": "group_id <> source_group_id",
			},
		},
	}
}

func (SystemCustomGroupModel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.String("public_model").MaxLen(200).NotEmpty(),
		field.Int64("source_group_id"),
		field.String("source_model").MaxLen(200).NotEmpty(),
		field.Bool("enabled").Default(true),
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

func (SystemCustomGroupModel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Ref("system_custom_routes").
			Field("group_id").
			Unique().
			Required(),
		edge.From("source_group", Group.Type).
			Ref("system_custom_source_routes").
			Field("source_group_id").
			Unique().
			Required(),
	}
}

func (SystemCustomGroupModel) Indexes() []ent.Index {
	return []ent.Index{
		// Production uses the corresponding case-insensitive expression indexes
		// from migration 221a. These Ent indexes keep generated schemas aligned
		// for environments that use Ent's schema creation directly.
		index.Fields("group_id", "public_model").Unique(),
		index.Fields("group_id", "source_group_id", "source_model").Unique(),
		index.Fields("source_group_id"),
	}
}
