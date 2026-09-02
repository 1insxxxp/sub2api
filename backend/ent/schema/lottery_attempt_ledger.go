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

// LotteryAttemptLedger is an immutable audit row for one wallet change.
type LotteryAttemptLedger struct {
	ent.Schema
}

func (LotteryAttemptLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "lottery_attempt_ledger",
			Checks: map[string]string{
				"lottery_attempt_ledger_delta_check":       "delta <> 0",
				"lottery_attempt_ledger_balance_check":     "balance_after >= 0",
				"lottery_attempt_ledger_source_type_check": "source_type IN ('checkin_streak', 'lottery_draw')",
			},
		},
	}
}

func (LotteryAttemptLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int("delta").Validate(func(value int) error {
			if value == 0 {
				return fmt.Errorf("must not be zero")
			}
			return nil
		}),
		field.Int("balance_after").NonNegative(),
		field.String("source_type").MaxLen(32).Validate(func(value string) error {
			switch value {
			case "checkin_streak", "lottery_draw":
				return nil
			default:
				return fmt.Errorf("unsupported lottery attempt source type %q", value)
			}
		}),
		field.Int64("source_id").Positive(),
		field.String("description").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryAttemptLedger) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("lottery_attempt_ledger").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (LotteryAttemptLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_type", "source_id").Unique().StorageKey("lottery_attempt_ledger_source_uq"),
		index.Fields("user_id", "created_at").StorageKey("lottery_attempt_ledger_user_created_at_idx"),
	}
}
