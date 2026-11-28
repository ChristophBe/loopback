package auth

import (
	"context"
	"fmt"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/user"
)

// DevEmail is the fixed account that dev login signs in.
const DevEmail = "dev@loopback.local"

// DevLogin signs in the dev user and creates it when missing. It checks no
// credentials and signs in only DevEmail, so it cannot reach other accounts,
// but the caller must still expose it for local use only.
func (s *Service) DevLogin(ctx context.Context) (Tokens, error) {
	email := DevEmail
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
