package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// EmailProvider is internal configuration and is never exposed to users.
type EmailProvider struct{ ent.Schema }

func (EmailProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "email_providers"}}
}
func (EmailProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Unique(), field.String("name").MaxLen(128).NotEmpty(),
		field.String("base_url").MaxLen(512).NotEmpty(), field.String("credential_ref").MaxLen(256).Default(""),
		field.Bool("enabled").Default(false), field.String("health_status").MaxLen(32).Default("unknown"),
		field.JSON("capabilities", map[string]any{}), field.JSON("billing", map[string]any{}), field.JSON("metadata", map[string]any{}),
		field.Time("created_at"), field.Time("updated_at"),
	}
}
