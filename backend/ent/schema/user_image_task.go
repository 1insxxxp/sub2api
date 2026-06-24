package schema

import (
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserImageTask stores persistent async image generation task state.
type UserImageTask struct {
	ent.Schema
}

func (UserImageTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_image_tasks"},
	}
}

func (UserImageTask) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("api_key_id").
			Optional().
			Nillable(),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int64("image_id").
			Optional().
			Nillable(),
		field.String("mode").
			MaxLen(32).
			NotEmpty(),
		field.String("status").
			MaxLen(32).
			Validate(func(value string) error {
				switch value {
				case "queued", "running", "succeeded", "failed":
					return nil
				default:
					return fmt.Errorf("invalid image task status: %s", value)
				}
			}),
		field.String("model").
			MaxLen(128).
			NotEmpty(),
		field.Text("prompt").
			Optional().
			Nillable(),
		field.String("aspect_ratio").
			MaxLen(16).
			NotEmpty(),
		field.String("quality").
			MaxLen(32).
			NotEmpty(),
		field.String("output_format").
			MaxLen(16).
			Default("png"),
		field.String("background").
			MaxLen(24).
			Default("auto"),
		field.String("size").
			MaxLen(32).
			NotEmpty(),
		field.Float("estimated_cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int("source_image_count").
			Default(0),
		field.Text("reference_object_keys").
			Optional().
			Nillable(),
		field.String("error_reason").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Text("error_message").
			Optional().
			Nillable(),
		field.Time("started_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserImageTask) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_image_tasks").
			Field("user_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("image", UserImage.Type).
			Ref("tasks").
			Field("image_id").
			Unique(),
	}
}

func (UserImageTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("status", "created_at"),
		index.Fields("image_id"),
	}
}
