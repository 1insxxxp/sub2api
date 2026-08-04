package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserCustomGroupModel maps one public model to one concrete source group.
type UserCustomGroupModel struct{ ent.Schema }

func (UserCustomGroupModel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_custom_group_models"}}
}

func (UserCustomGroupModel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("custom_group_id"),
		field.String("public_model").MaxLen(200).NotEmpty(),
		field.Int64("source_group_id"),
		field.String("source_model").MaxLen(200).NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UserCustomGroupModel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("custom_group", UserCustomGroup.Type).Ref("models").Field("custom_group_id").Unique().Required(),
		edge.From("source_group", Group.Type).Ref("custom_model_routes").Field("source_group_id").Unique().Required(),
	}
}

func (UserCustomGroupModel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("custom_group_id", "public_model").Unique(),
		index.Fields("source_group_id"),
	}
}
