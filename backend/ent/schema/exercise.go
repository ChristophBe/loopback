package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Exercise is a quiz, flashcard deck or bad-code review. The type-specific
// content (questions, cards, findings) is stored as JSON in payload and
// validated against api/schemas by the importer.
type Exercise struct {
	ent.Schema
}

// Fields of the Exercise.
func (Exercise) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty(),
		field.Enum("type").Values("quiz", "flashcard", "review"),
		field.String("title").NotEmpty(),
		field.JSON("payload", map[string]any{}),
	}
}

// Edges of the Exercise.
func (Exercise) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("course", Course.Type).Ref("exercises").Unique().Required(),
		edge.From("content_version", ContentVersion.Type).Ref("exercises").Unique(),
	}
}

// Indexes of the Exercise.
func (Exercise) Indexes() []ent.Index {
	return []ent.Index{index.Fields("slug").Edges("course").Unique()}
}
