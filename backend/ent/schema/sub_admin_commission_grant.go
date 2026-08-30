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

// SubAdminCommissionGrant stores group financial visibility granted to a secondary admin.
type SubAdminCommissionGrant struct {
	ent.Schema
}

func (SubAdminCommissionGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sub_admin_commission_grants"},
	}
}

func (SubAdminCommissionGrant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("sub_admin_user_id"),
		field.Int64("group_id"),
		field.Time("granted_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Bool("enabled").
			Default(true),
		field.Int64("created_by").
			Optional().
			Nillable(),
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

func (SubAdminCommissionGrant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sub_admin", User.Type).
			Unique().
			Required().
			Field("sub_admin_user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id").
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("creator", User.Type).
			Unique().
			Field("created_by").
			Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}

func (SubAdminCommissionGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sub_admin_user_id", "group_id").
			Unique().
			StorageKey("idx_sub_admin_commission_grants_active_unique").
			Annotations(entsql.IndexWhere("enabled = TRUE")),
		index.Fields("sub_admin_user_id", "enabled", "granted_date").
			StorageKey("idx_sub_admin_commission_grants_sub_admin_enabled"),
		index.Fields("group_id", "enabled").
			StorageKey("idx_sub_admin_commission_grants_group_enabled"),
	}
}
