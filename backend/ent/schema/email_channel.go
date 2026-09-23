package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type EmailChannel struct{ ent.Schema }

func (EmailChannel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "email_channels"}}
}
func (EmailChannel) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Unique(), field.String("public_name").MaxLen(128).NotEmpty(),
		field.String("role").MaxLen(16).Default("primary"), field.Int64("provider_id"), field.String("email_type").MaxLen(32).Default("temporary_gmail"),
		field.String("privacy_level").MaxLen(32).Default("public_temporary"), field.Bool("enabled").Default(false), field.Bool("visible").Default(true), field.Bool("healthy").Default(false),
		field.Float("sale_price").Default(0), field.Int("order_ttl_seconds").Default(900), field.String("capture_policy").MaxLen(64).Default("on_target_email_received"),
		field.String("refund_policy").MaxLen(64).Default("refund_if_no_message"), field.JSON("polling_backoff", []int{2, 4, 7, 10, 15, 20, 30, 45, 60}), field.Int("max_provider_requests_per_order").Default(60),
		field.Int("sort_order").Default(0), field.JSON("metadata", map[string]any{}), field.Time("created_at"), field.Time("updated_at"),
	}
}
