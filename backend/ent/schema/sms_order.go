package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type SmsOrder struct{ ent.Schema }

func (SmsOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sms_orders"}}
}

func (SmsOrder) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("public_id", uuid.UUID{}).Unique(),
		field.Int64("user_id"),
		field.Int64("channel_id"),
		field.Int64("provider_id"),
		field.Int64("service_id"),
		field.Int64("country_id"),
		field.String("product_type").MaxLen(16),
		field.String("status").MaxLen(32).Default("pending"),
		field.String("provider_order_id").MaxLen(256).Default(""),
		field.String("phone_number").MaxLen(64).Default(""),
		field.Float("provider_cost_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}).Default(0),
		field.Float("sale_price_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}).Default(0),
		field.Float("success_rate_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(8,5)"}).Optional(),
		field.String("success_rate_source_snapshot").MaxLen(32).Default("unavailable"),
		field.String("success_rate_grade_snapshot").MaxLen(2).Default(""),
		field.Float("success_rate_multiplier_snapshot").SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}).Default(1),
		field.String("currency_snapshot").MaxLen(8).Default("USD"),
		field.Time("expires_at").Optional(),
		field.String("idempotency_key").MaxLen(128),
		field.String("refund_status").MaxLen(32).Default("not_requested"),
		field.String("refund_reason").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.String("last_provider_error").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.JSON("metadata", map[string]any{}),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}
