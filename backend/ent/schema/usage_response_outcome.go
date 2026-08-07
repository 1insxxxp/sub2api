package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UsageResponseOutcome stores privacy-safe response classification evidence.
type UsageResponseOutcome struct{ ent.Schema }

func (UsageResponseOutcome) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "usage_response_outcomes"}}
}

func (UsageResponseOutcome) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("usage_log_id").Optional().Nillable(),
		field.String("request_id").MaxLen(64).NotEmpty(),
		field.Int64("api_key_id"),
		field.Int64("user_id"),
		field.Int64("account_id"),
		field.Int64("group_id").Optional().Nillable(),
		field.Int("http_status").Default(0),
		field.Int("upstream_status").Default(0),
		field.Bool("has_text").Default(false),
		field.Bool("has_tool_call").Default(false),
		field.Bool("has_reasoning").Default(false),
		field.Bool("has_media").Default(false),
		field.Int64("output_bytes").Default(0),
		field.Int("event_count").Default(0),
		field.Bool("stream_completed").Default(false),
		field.String("finish_reason").MaxLen(100).Default(""),
		field.String("disconnect_source").MaxLen(20).Default("none"),
		field.String("upstream_error_kind").MaxLen(32).Default("none"),
		field.Int16("collector_version").Default(1),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UsageResponseOutcome) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("usage_log_id").Unique(),
		index.Fields("request_id", "api_key_id").Unique(),
		index.Fields("user_id", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("account_id", "created_at"),
	}
}
