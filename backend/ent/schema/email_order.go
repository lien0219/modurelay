package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type EmailOrder struct{ ent.Schema }

func (EmailOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "email_orders"}}
}
func (EmailOrder) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("public_id", uuid.UUID{}).Unique(), field.String("order_no").MaxLen(64).Unique(), field.Int64("user_id"), field.Int64("channel_id"), field.Int64("provider_id"), field.Int64("service_id"),
		field.String("provider_inbox_id").MaxLen(256).Default(""), field.String("email_address").MaxLen(320).Default(""), field.String("address_type").MaxLen(32).Default("gmail"), field.String("status").MaxLen(32).Default("creating"),
		field.Float("sale_price_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}).Default(0), field.Float("provider_cost_estimate_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}).Default(0), field.JSON("pricing_rule_snapshot", map[string]any{}),
		field.Float("success_rate_snapshot").Optional(), field.String("success_rate_grade_snapshot").MaxLen(2).Default(""), field.String("refund_policy_snapshot").MaxLen(64).Default("refund_if_no_message"), field.String("capture_policy_snapshot").MaxLen(64).Default("on_target_email_received"),
		field.Float("reserved_amount").Default(0), field.Float("captured_amount").Default(0), field.Float("released_amount").Default(0), field.Float("refunded_amount").Default(0),
		field.Time("inbox_created_at").Optional(), field.Time("waiting_started_at").Optional(), field.Time("first_message_at").Optional(), field.Time("completed_at").Optional(), field.Time("cancelled_at").Optional(), field.Time("expires_at").Optional(), field.Int("poll_count").Default(0), field.Time("next_poll_at").Optional(), field.Time("last_polled_at").Optional(), field.Int("provider_request_count").Default(0),
		field.String("error_code").MaxLen(64).Default(""), field.String("error_public_message").MaxLen(256).Default(""), field.String("error_admin_message").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""), field.String("refund_status").MaxLen(32).Default("not_requested"), field.String("refund_reason").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""), field.String("idempotency_key").MaxLen(128), field.JSON("metadata", map[string]any{}), field.Time("created_at"), field.Time("updated_at"),
	}
}
func (EmailOrder) Indexes() []ent.Index { return nil }
