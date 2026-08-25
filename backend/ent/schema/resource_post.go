package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ResourcePost is a user-generated resource-sharing topic.
type ResourcePost struct {
	ent.Schema
}

func (ResourcePost) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "resource_posts"}}
}

func (ResourcePost) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("category_id"),
		field.Int64("author_id"),
		field.String("author_username").MaxLen(100).NotEmpty(),
		field.String("author_role").MaxLen(20).NotEmpty(),
		field.String("title").MaxLen(200).NotEmpty(),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).NotEmpty(),
		field.String("status").MaxLen(20).Default("published"),
		field.Int("view_count").Default(0),
		field.Int("like_count").Default(0),
		field.Int("comment_count").Default(0),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
