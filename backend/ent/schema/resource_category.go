package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ResourceCategory groups resource-sharing posts for discovery and filtering.
type ResourceCategory struct { ent.Schema }

func (ResourceCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "resource_categories"}}
}

func (ResourceCategory) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").MaxLen(80).NotEmpty().Unique(),
		field.String("name").MaxLen(120).NotEmpty(),
		field.String("description").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Int("sort_order").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
