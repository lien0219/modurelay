package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ResourceNotification notifies an author when another user comments or replies.
type ResourceNotification struct { ent.Schema }

func (ResourceNotification) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "resource_notifications"}}
}

func (ResourceNotification) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("actor_id"),
		field.String("actor_username").MaxLen(100).NotEmpty(),
		field.String("actor_role").MaxLen(20).NotEmpty(),
		field.Int64("post_id"),
		field.Int64("comment_id").Optional().Nillable(),
		field.String("kind").MaxLen(20).NotEmpty(),
		field.Bool("read").Default(false),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
