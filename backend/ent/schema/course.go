package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Course is one project in one language, for example the Go project "kv-store".
type Course struct {
	ent.Schema
}

// Fields of the Course.
func (Course) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("language").Values("go", "rust").Immutable(),
		field.String("project").NotEmpty().Immutable(),
		field.String("title").NotEmpty(),
	}
}

// Edges of the Course.
func (Course) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("lessons", Lesson.Type),
		edge.To("exercises", Exercise.Type),
	}
}

// Indexes of the Course.
func (Course) Indexes() []ent.Index {
	return []ent.Index{index.Fields("language", "project").Unique()}
}
