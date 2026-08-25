package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ResourceLike stores one user reaction for either a post or a comment.
type ResourceLike struct { ent.Schema }

func (ResourceLike) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "resource_likes"}}
}

func (ResourceLike) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("post_id").Optional().Nillable(),
		field.Int64("comment_id").Optional().Nillable(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ResourceLike) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "post_id").Unique(),
		index.Fields("user_id", "comment_id").Unique(),
		index.Fields("post_id"),
		index.Fields("comment_id"),
	}
}
