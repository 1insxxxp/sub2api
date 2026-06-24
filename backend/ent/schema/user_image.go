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

// UserImage stores generated AI image metadata owned by a user.
type UserImage struct {
	ent.Schema
}

func (UserImage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_images"},
	}
}

func (UserImage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("mode").
			MaxLen(32).
			NotEmpty(),
		field.String("model").
			MaxLen(128).
			NotEmpty(),
		field.Text("prompt").
			Optional().
			Nillable(),
		field.String("aspect_ratio").
			MaxLen(16).
			NotEmpty(),
		field.String("size").
			MaxLen(32).
			NotEmpty(),
		field.String("image_url").
			MaxLen(2048).
			NotEmpty(),
		field.String("storage_driver").
			MaxLen(32).
			Default("local"),
		field.String("storage_object_key").
			MaxLen(1024).
			NotEmpty(),
		field.String("mime_type").
			MaxLen(128).
			Default("image/png"),
		field.String("output_format").
			MaxLen(16).
			Default("png"),
		field.String("background").
			MaxLen(24).
			Default("auto"),
		field.Int64("bytes").
			Default(0),
		field.Float("cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int64("usage_log_id").
			Optional().
			Nillable(),
		field.Int("source_image_count").
			Default(0),
		field.Time("expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("deleted_at").
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

func (UserImage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_images").
			Field("user_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("tasks", UserImageTask.Type),
	}
}

func (UserImage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("deleted_at"),
		index.Fields("expires_at"),
		index.Fields("storage_object_key"),
	}
}
