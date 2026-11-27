package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/session"
)

// Tokens is a new access and refresh token pair.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	// ExpiresIn is the lifetime of the access token.
	ExpiresIn time.Duration
}

// Service issues and verifies tokens. The sign-in flows use IssueTokens.
type Service struct {
	db  *ent.Client
	cfg Config
	now func() time.Time
}

// New returns a Service. db may be nil when only access tokens are verified.
func New(db *ent.Client, cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Service{db: db, cfg: cfg, now: time.Now}, nil
}

// IssueTokens starts a new session for the user.
func (s *Service) IssueTokens(ctx context.Context, userID int, device *string) (Tokens, error) {
	return s.issue(ctx, s.db, userID, device)
}

// Refresh exchanges a refresh token for a new pair. The old session is
// revoked, so the token works once; a revoked, expired or unknown token yields
// ErrInvalidToken.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return Tokens{}, fmt.Errorf("begin refresh: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	old, err := tx.Session.Query().
		Where(session.TokenHash(hashToken(refreshToken))).
		WithUser().
		Only(ctx)
	if ent.IsNotFound(err) {
		return Tokens{}, ErrInvalidToken
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("find session: %w", err)
	}

	// The conditional update makes the rotation atomic: of two concurrent
	// requests with the same token only one revokes the session.
	revoked, err := tx.Session.Update().
		Where(session.ID(old.ID), session.Revoked(false), session.ExpiresAtGT(s.now())).
		SetRevoked(true).
		Save(ctx)
	if err != nil {
		return Tokens{}, fmt.Errorf("revoke session: %w", err)
	}
	if revoked == 0 {
		return Tokens{}, ErrInvalidToken
	}

	tokens, err := s.issue(ctx, tx.Client(), old.Edges.User.ID, old.Device)
	if err != nil {
		return Tokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return Tokens{}, fmt.Errorf("commit refresh: %w", err)
	}
	return tokens, nil
}

func (s *Service) issue(ctx context.Context, db *ent.Client, userID int, device *string) (Tokens, error) {
	access, ttl, err := s.IssueAccessToken(userID)
	if err != nil {
		return Tokens{}, err
	}
	refresh, hash, err := newRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	err = db.Session.Create().
		SetUserID(userID).
		SetTokenHash(hash).
		SetExpiresAt(s.now().Add(s.cfg.RefreshTTL)).
		SetNillableDevice(device).
		Exec(ctx)
	if err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	return Tokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: ttl}, nil
}
