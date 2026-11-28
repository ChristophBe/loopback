package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/identity"
	"github.com/ChristophBe/loopback/backend/ent/user"
)

// DefaultMagicLinkTTL is the lifetime of a sign-in link.
const DefaultMagicLinkTTL = 15 * time.Minute

// ErrInvalidEmail is returned for an address that cannot receive mail.
var ErrInvalidEmail = errors.New("invalid email address")

// Mailer sends a plain-text email. internal/mail.SMTP implements it.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// MagicLinkConfig holds the settings of the email sign-in.
type MagicLinkConfig struct {
	// BaseURL is the origin of the web app. The link is BaseURL/auth/verify?token=...
	BaseURL string
	TTL     time.Duration
}

// LoadMagicLinkConfig reads WEB_BASE_URL (required) and MAGIC_LINK_TTL (a Go
// duration, default 15m) with getenv.
func LoadMagicLinkConfig(getenv func(string) string) (MagicLinkConfig, error) {
	cfg := MagicLinkConfig{BaseURL: getenv("WEB_BASE_URL"), TTL: DefaultMagicLinkTTL}
	if v := getenv("MAGIC_LINK_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return MagicLinkConfig{}, fmt.Errorf("MAGIC_LINK_TTL: %w", err)
		}
		cfg.TTL = d
	}
	return cfg, cfg.validate()
}

func (c MagicLinkConfig) validate() error {
	u, err := url.Parse(c.BaseURL)
	switch {
	case c.BaseURL == "":
		return errors.New("WEB_BASE_URL is not set")
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		return errors.New("WEB_BASE_URL must be an http(s) URL")
	case c.TTL < time.Second:
		return errors.New("magic link lifetime must be at least 1s")
	}
	return nil
}

// MagicLinks sends sign-in links and signs the learner in with them. The
// link token is stateless: it carries the email address, a nonce and the
// expiry, signed with the JWT secret. A used link is remembered as a revoked
// session whose hash is derived from the nonce; its unique token_hash makes
// the link work once.
type MagicLinks struct {
	svc    *Service
	mailer Mailer
	cfg    MagicLinkConfig
}

// NewMagicLinks returns the magic-link flow on top of the token service.
func NewMagicLinks(svc *Service, mailer Mailer, cfg MagicLinkConfig) (*MagicLinks, error) {
	if svc == nil || svc.db == nil || mailer == nil {
		return nil, errors.New("auth service with a database and a mailer are required")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &MagicLinks{svc: svc, mailer: mailer, cfg: cfg}, nil
}

// Request emails a sign-in link to the address. It does not reveal whether an
// account exists: the mail goes out for every valid address.
func (m *MagicLinks) Request(ctx context.Context, email string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	token, err := m.sign(email)
	if err != nil {
		return err
	}
	link := strings.TrimRight(m.cfg.BaseURL, "/") + "/auth/verify?token=" + url.QueryEscape(token)
	body := fmt.Sprintf("Sign in to loopback with this link:\n\n%s\n\nThe link works once and expires in %s. "+
		"If you did not ask for it, ignore this email.\n", link, m.cfg.TTL.Round(time.Minute))
	return m.mailer.Send(ctx, email, "Your loopback sign-in link", body)
}

// Verify signs the learner in with the token of a link and creates the
// account on first use. An expired, tampered or already used token yields
// ErrInvalidToken.
func (m *MagicLinks) Verify(ctx context.Context, token string) (Tokens, error) {
	claims, err := m.parse(token)
	if err != nil {
		return Tokens{}, err
	}
	userID, err := m.ensureUser(ctx, claims.Email)
	if err != nil {
		return Tokens{}, err
	}

	tx, err := m.svc.db.Tx(ctx)
	if err != nil {
		return Tokens{}, fmt.Errorf("begin magic link sign-in: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// The unique token_hash lets only the first use of the nonce succeed.
	err = tx.Session.Create().
		SetUserID(userID).
		SetTokenHash(hashToken("magic-link:" + claims.Nonce)).
		SetExpiresAt(time.Unix(claims.Expires, 0)).
		SetRevoked(true).
		Exec(ctx)
	if ent.IsConstraintError(err) {
		return Tokens{}, ErrInvalidToken
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("consume magic link: %w", err)
	}
	tokens, err := m.svc.issue(ctx, tx.Client(), userID, nil)
	if err != nil {
		return Tokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return Tokens{}, fmt.Errorf("commit magic link sign-in: %w", err)
	}
	return tokens, nil
}

type linkClaims struct {
	Email   string `json:"e"`
	Nonce   string `json:"n"`
	Expires int64  `json:"x"`
}

func (m *MagicLinks) sign(email string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	payload, err := json.Marshal(linkClaims{
		Email:   email,
		Nonce:   base64.RawURLEncoding.EncodeToString(nonce),
		Expires: m.svc.now().Add(m.cfg.TTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("encode magic link: %w", err)
	}
	p := base64.RawURLEncoding.EncodeToString(payload)
	return p + "." + base64.RawURLEncoding.EncodeToString(m.mac(p)), nil
}

func (m *MagicLinks) parse(token string) (linkClaims, error) {
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return linkClaims{}, ErrInvalidToken
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, m.mac(p)) {
		return linkClaims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return linkClaims{}, ErrInvalidToken
	}
	var c linkClaims
	if err := json.Unmarshal(payload, &c); err != nil || c.Email == "" || c.Nonce == "" {
		return linkClaims{}, ErrInvalidToken
	}
	if !m.svc.now().Before(time.Unix(c.Expires, 0)) {
		return linkClaims{}, ErrInvalidToken
	}
	return c, nil
}

// mac signs with a purpose-bound key so that a link token cannot be mistaken
// for any other signed value.
func (m *MagicLinks) mac(payload string) []byte {
	h := hmac.New(sha256.New, m.svc.cfg.Secret)
	h.Write([]byte("loopback/magic-link/v1."))
	h.Write([]byte(payload))
	return h.Sum(nil)
}

// ensureUser returns the id of the user with the email, creating the user and
// its email identity on first sign-in. A concurrent first sign-in that loses
// the race on the unique constraints reads the winner's rows.
func (m *MagicLinks) ensureUser(ctx context.Context, email string) (int, error) {
	db := m.svc.db
	u, err := db.User.Query().Where(user.Email(email)).Only(ctx)
	if ent.IsNotFound(err) {
		u, err = db.User.Create().SetEmail(email).Save(ctx)
		if ent.IsConstraintError(err) {
			u, err = db.User.Query().Where(user.Email(email)).Only(ctx)
		}
	}
	if err != nil {
		return 0, fmt.Errorf("find or create user: %w", err)
	}
	exists, err := db.Identity.Query().
		Where(identity.ProviderEQ(identity.ProviderEmail), identity.ProviderUserID(email)).
		Exist(ctx)
	if err != nil {
		return 0, fmt.Errorf("find identity: %w", err)
	}
	if !exists {
		err = db.Identity.Create().SetProvider(identity.ProviderEmail).SetProviderUserID(email).SetUserID(u.ID).Exec(ctx)
		if err != nil && !ent.IsConstraintError(err) {
			return 0, fmt.Errorf("create identity: %w", err)
		}
	}
	return u.ID, nil
}

// normalizeEmail trims and lower-cases a plain address such as a@b.example.
func normalizeEmail(s string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(s))
	if err != nil || addr.Name != "" || strings.ContainsAny(addr.Address, "\r\n") {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(addr.Address), nil
}
