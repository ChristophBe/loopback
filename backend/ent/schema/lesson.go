package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Lesson is one step on a course's fixed path. Its fields follow
// api/schemas/lesson.schema.json.
type Lesson struct {
	ent.Schema
}

// Fields of the Lesson.
func (Lesson) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty(),
		field.Int("order").Positive(),
		field.String("title").NotEmpty(),
		field.Int("minutes").Positive(),
		field.Int("xp").Positive(),
		field.Text("body").Default(""),
		field.JSON("owned_files", []string{}).Optional(),
		// starter_files and test_files map a path relative to the learner's
		// project to the file content, as read from the lesson's starterFiles
		// and testFiles directories.
		field.JSON("starter_files", map[string]string{}).Optional(),
		field.JSON("test_files", map[string]string{}).Optional(),
		field.JSON("hints", []string{}).Optional(),
		field.JSON("failure_explanations", map[string]string{}).Optional(),
	}
}

// Edges of the Lesson.
func (Lesson) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("course", Course.Type).Ref("lessons").Unique().Required(),
		edge.From("content_version", ContentVersion.Type).Ref("lessons").Unique(),
	}
}

// Indexes of the Lesson.
func (Lesson) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").Edges("course").Unique(),
		index.Fields("order").Edges("course").Unique(),
	}
}
