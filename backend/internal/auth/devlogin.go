package auth

import (
	"context"
	"fmt"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/user"
)

// DevEmail is the account that dev login signs in when no email is given.
const DevEmail = "dev@loopback.local"

// DevLogin signs in the user with the email and creates it when missing. It
// checks no credentials, so the caller must only expose it for local use.
func (s *Service) DevLogin(ctx context.Context, email string) (Tokens, error) {
	if email == "" {
		email = DevEmail
	}
	id, err := s.db.User.Query().Where(user.Email(email)).OnlyID(ctx)
	if ent.IsNotFound(err) {
		var u *ent.User
		u, err = s.db.User.Create().SetEmail(email).Save(ctx)
		if err == nil {
			id = u.ID
		}
		if ent.IsConstraintError(err) { // lost a race with a parallel request
			id, err = s.db.User.Query().Where(user.Email(email)).OnlyID(ctx)
		}
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("find dev user: %w", err)
	}
	return s.IssueTokens(ctx, id, nil)
}
