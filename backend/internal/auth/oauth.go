package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/identity"
	"github.com/ChristophBe/loopback/backend/ent/user"
)

const (
	// OAuthStateCookie is the cookie that binds an OAuth flow to the browser.
	OAuthStateCookie = "oauth_state"
	oauthStateTTL    = 10 * time.Minute
)

var (
	// ErrUnknownProvider is returned for a provider that is not configured.
	ErrUnknownProvider = errors.New("unknown oauth provider")
	// ErrInvalidState is returned when the state is malformed, forged, expired,
	// for another provider or does not match the cookie.
	ErrInvalidState = errors.New("invalid oauth state")
	// ErrOAuthExchange is returned when the provider rejects the code. A
	// Provider wraps it for that case only, so outages stay server errors.
	ErrOAuthExchange = errors.New("oauth code exchange failed")
	// ErrEmailUnverified is returned when the provider gives no verified email.
	ErrEmailUnverified = errors.New("provider returned no verified email")
	// ErrUnverifiedAccount is returned when the email belongs to a user whose
	// email is not verified. The provider account is not linked to it.
	ErrUnverifiedAccount = errors.New("user with this email is not verified")
)

// Profile is the account that a provider signed in.
type Profile struct {
	// ID is the stable id of the account at the provider.
	ID    string
	Email string
	// EmailVerified is true when the provider vouches for the email.
	EmailVerified bool
}

// Provider is an OAuth identity provider. Tests replace it with a fake.
type Provider interface {
	// AuthURL returns the authorization URL for the state.
	AuthURL(state, redirectURL string) string
	// Exchange trades the authorization code for the account profile.
	Exchange(ctx context.Context, code, redirectURL string) (Profile, error)
}

// OAuthConfig configures the OAuth sign-in.
type OAuthConfig struct {
	// BaseURL is the public URL of the API including the base path, for
	// example https://loopback.dev/api/v1. The provider redirects back to it.
	BaseURL string
	// Providers maps "github" and "google" to their implementation.
	Providers map[string]Provider
}

// ConfigureOAuth enables the OAuth sign-in. Without it every provider is unknown.
func (s *Service) ConfigureOAuth(cfg OAuthConfig) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	s.oauth = cfg
}

func (s *Service) oauthProvider(name string) (Provider, string, error) {
	p, ok := s.oauth.Providers[name]
	if !ok || p == nil {
		return nil, "", ErrUnknownProvider
	}
	return p, s.oauth.BaseURL + "/auth/oauth/" + name + "/callback", nil
}

// StartOAuth returns the provider's authorization URL and the state cookie
// that the callback checks. The state is signed with the JWT secret, so no
// server-side storage is needed.
func (s *Service) StartOAuth(provider string) (authURL string, cookie *http.Cookie, err error) {
	p, redirect, err := s.oauthProvider(provider)
	if err != nil {
		return "", nil, err
	}
	nonce, err := randomString(16)
	if err != nil {
		return "", nil, err
	}
	expires := s.now().Add(oauthStateTTL)
	cookie = s.stateCookie(nonce, expires, int(oauthStateTTL.Seconds()))
	return p.AuthURL(s.signState(provider, nonce, expires), redirect), cookie, nil
}

// ClearOAuthStateCookie returns the cookie that deletes the state cookie. The
// callback sends it with every answer, so a state works once.
func (s *Service) ClearOAuthStateCookie() *http.Cookie {
	return s.stateCookie("", time.Unix(0, 0), -1)
}

func (s *Service) stateCookie(value string, expires time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure follows the scheme of the public URL; local dev runs on http.
		Name:     OAuthStateCookie,
		Value:    value,
		Path:     s.oauthCookiePath(),
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.oauth.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

// oauthCookiePath scopes the state cookie to the OAuth routes below the path
// of the public base URL.
func (s *Service) oauthCookiePath() string {
	path := "/auth/oauth"
	if u, err := url.Parse(s.oauth.BaseURL); err == nil {
		path = u.Path + path
	}
	return path
}

// FinishOAuth checks the state against the cookie, exchanges the code and
// signs the account in: it uses the Identity of the provider account, links
// the account to the user with the same email if that email is verified, or
// creates the user. A user with an unverified email yields ErrUnverifiedAccount.
func (s *Service) FinishOAuth(ctx context.Context, provider, code, state, cookieNonce string) (Tokens, error) {
	p, redirect, err := s.oauthProvider(provider)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.verifyState(provider, state, cookieNonce); err != nil {
		return Tokens{}, err
	}
	profile, err := p.Exchange(ctx, code, redirect)
	if err != nil {
		return Tokens{}, fmt.Errorf("exchange %s code: %w", provider, err)
	}
	if profile.ID == "" || profile.Email == "" || !profile.EmailVerified {
		return Tokens{}, ErrEmailUnverified
	}
	userID, err := s.upsertIdentity(ctx, identity.Provider(provider), profile)
	if ent.IsConstraintError(err) {
		// A concurrent callback for the same account created the rows first.
		userID, err = s.upsertIdentity(ctx, identity.Provider(provider), profile)
	}
	if err != nil {
		return Tokens{}, err
	}
	return s.IssueTokens(ctx, userID, nil)
}

func (s *Service) upsertIdentity(ctx context.Context, provider identity.Provider, profile Profile) (int, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin oauth sign-in: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	id, err := tx.Identity.Query().
		Where(identity.ProviderEQ(provider), identity.ProviderUserID(profile.ID)).
		QueryUser().OnlyID(ctx)
	if ent.IsNotFound(err) {
		id, err = linkOrCreateUser(ctx, tx.Client(), provider, profile)
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit oauth sign-in: %w", err)
	}
	return id, nil
}

func linkOrCreateUser(ctx context.Context, db *ent.Client, provider identity.Provider, profile Profile) (int, error) {
	email := strings.ToLower(profile.Email)
	u, err := db.User.Query().Where(user.EmailEqualFold(email)).Only(ctx)
	if ent.IsNotFound(err) {
		// The provider vouches for the email, so the new user starts verified.
		u, err = db.User.Create().SetEmail(email).SetEmailVerified(true).Save(ctx)
	}
	if err != nil {
		return 0, fmt.Errorf("find or create user: %w", err)
	}
	if !u.EmailVerified {
		return 0, ErrUnverifiedAccount
	}
	userID := u.ID
	err = db.Identity.Create().
		SetProvider(provider).
		SetProviderUserID(profile.ID).
		SetUserID(userID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("create identity: %w", err)
	}
	return userID, nil
}

// signState returns "provider.nonce.expiry.signature".
func (s *Service) signState(provider, nonce string, expires time.Time) string {
	payload := provider + "." + nonce + "." + strconv.FormatInt(expires.Unix(), 10)
	return payload + "." + s.stateMAC(payload)
}

func (s *Service) stateMAC(payload string) string {
	mac := hmac.New(sha256.New, s.cfg.Secret)
	mac.Write([]byte("oauth-state:" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) verifyState(provider, state, cookieNonce string) error {
	i := strings.LastIndexByte(state, '.')
	if i < 0 || cookieNonce == "" {
		return ErrInvalidState
	}
	payload, sig := state[:i], state[i+1:]
	if !hmac.Equal([]byte(sig), []byte(s.stateMAC(payload))) {
		return ErrInvalidState
	}
	parts := strings.Split(payload, ".")
	if len(parts) != 3 || parts[0] != provider {
		return ErrInvalidState
	}
	expires, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !s.now().Before(time.Unix(expires, 0)) {
		return ErrInvalidState
	}
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(cookieNonce)) != 1 {
		return ErrInvalidState
	}
	return nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
