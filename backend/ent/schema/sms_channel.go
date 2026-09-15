package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type SmsChannel struct{ ent.Schema }

func (SmsChannel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sms_channels"}}
}

func (SmsChannel) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Unique(),
		field.String("public_name").MaxLen(128).NotEmpty(),
		field.String("role").MaxLen(16).Default("primary"),
		field.Int64("provider_id"),
		field.Bool("enabled").Default(false),
		field.Bool("visible").Default(true),
		field.Bool("healthy").Default(false),
		field.Int("sort_order").Default(0),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}
