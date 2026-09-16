package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type EmailMessage struct{ ent.Schema }

func (EmailMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "email_messages"}}
}
func (EmailMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("email_order_id"), field.String("provider_message_id").MaxLen(256).Default(""), field.String("from_address").MaxLen(320).Default(""), field.String("from_name").MaxLen(256).Default(""), field.String("to_address").MaxLen(320).Default(""), field.String("subject").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""), field.String("text_body").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""), field.String("html_body").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""), field.String("verification_code").MaxLen(32).Default(""), field.String("verification_url").MaxLen(2048).Default(""), field.Float("verification_confidence").Default(0), field.String("verification_method").MaxLen(64).Default(""), field.Time("received_at"), field.String("dedupe_hash").MaxLen(128), field.JSON("raw_payload", map[string]any{}), field.Time("created_at"), field.Time("updated_at"),
	}
}
