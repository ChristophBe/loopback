package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Session is one refresh token of a User. Only the hash of the token is
// stored, so a database leak does not expose usable tokens.
type Session struct {
	ent.Schema
}

// Fields of the Session.
func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").NotEmpty().Unique().Immutable().Sensitive(),
		field.Time("expires_at"),
		field.Bool("revoked").Default(false),
		field.String("device").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Session.
func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Unique().Required(),
	}
}
