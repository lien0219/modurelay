package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ResourceComment supports threaded replies through parent_id.
type ResourceComment struct {
	ent.Schema
}

func (ResourceComment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "resource_comments"}}
}

func (ResourceComment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("post_id"),
		field.Int64("parent_id").Optional().Nillable(),
		field.Int64("author_id"),
		field.String("author_username").MaxLen(100).NotEmpty(),
		field.String("author_role").MaxLen(20).NotEmpty(),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).NotEmpty(),
		field.String("status").MaxLen(20).Default("published"),
		field.Int("like_count").Default(0),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
