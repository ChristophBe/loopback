package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ContentVersion records one import run. Lessons and exercises point to the
// version that last wrote them.
type ContentVersion struct {
	ent.Schema
}

// Fields of the ContentVersion.
func (ContentVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("checksum").NotEmpty().Unique().Immutable(),
		field.Time("imported_at").Default(time.Now).Immutable(),
	}
}

// Edges of the ContentVersion.
func (ContentVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("lessons", Lesson.Type),
		edge.To("exercises", Exercise.Type),
	}
}
