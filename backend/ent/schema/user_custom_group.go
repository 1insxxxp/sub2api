package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserCustomGroup is a user-owned virtual group whose models route to concrete groups.
type UserCustomGroup struct{ ent.Schema }

func (UserCustomGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_custom_groups"}}
}

func (UserCustomGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (UserCustomGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("status").MaxLen(20).Default(domain.StatusActive),
	}
}

func (UserCustomGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("custom_groups").Field("user_id").Unique().Required(),
		edge.To("models", UserCustomGroupModel.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("api_keys", APIKey.Type),
		edge.To("usage_logs", UsageLog.Type),
	}
}

func (UserCustomGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("deleted_at"),
		index.Fields("user_id", "name").Unique().StorageKey("idx_user_custom_groups_owner_name_active").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
