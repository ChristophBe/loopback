package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Identity is a provider account that signs a User in, for example a GitHub
// account. One User can have several identities.
type Identity struct {
	ent.Schema
}

// Fields of the Identity.
func (Identity) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("provider").Values("github", "google", "email").Immutable(),
		field.String("provider_user_id").NotEmpty().Immutable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Identity.
func (Identity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("identities").Unique().Required(),
	}
}

// Indexes of the Identity.
func (Identity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("provider", "provider_user_id").Unique()}
}
