package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/ent/session"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newService(t *testing.T, db *ent.Client) (*auth.Service, *clock) {
	t.Helper()
	svc, err := auth.New(db, auth.Config{Secret: secret, AccessTTL: 15 * time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	clk := &clock{t: time.Now()}
	auth.SetClock(svc, clk.now)
	return svc, clk
}

func newDB(t *testing.T) *ent.Client {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newUser(t *testing.T, db *ent.Client) *ent.User {
	t.Helper()
	return db.User.Create().SetEmail(fmt.Sprintf("auth-%s-%d@example.com", t.Name(), time.Now().UnixNano())).SaveX(context.Background())
}

func TestLoadConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	good := string(secret)

	cfg, err := auth.LoadConfig(env(map[string]string{"JWT_SECRET": good}))
	if err != nil || cfg.AccessTTL != 15*time.Minute || cfg.RefreshTTL != auth.DefaultRefreshTTL {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	cfg, err = auth.LoadConfig(env(map[string]string{"JWT_SECRET": good, "JWT_ACCESS_TTL": "5m"}))
	if err != nil || cfg.AccessTTL != 5*time.Minute {
		t.Fatalf("custom ttl: %+v, %v", cfg, err)
	}
	for name, m := range map[string]map[string]string{
		"unset":      {},
		"short":      {"JWT_SECRET": "short"},
		"bad ttl":    {"JWT_SECRET": good, "JWT_ACCESS_TTL": "soon"},
		"zero ttl":   {"JWT_SECRET": good, "JWT_ACCESS_TTL": "0s"},
		"subsecond":  {"JWT_SECRET": good, "JWT_ACCESS_TTL": "500ms"},
		"negative":   {"JWT_SECRET": good, "REFRESH_TOKEN_TTL": "-1h"},
		"empty ttls": {"JWT_SECRET": " "},
	} {
		if _, err := auth.LoadConfig(env(m)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestAccessToken(t *testing.T) {
	svc, clk := newService(t, nil)
	token, ttl, err := svc.IssueAccessToken(42)
	if err != nil || ttl != 15*time.Minute {
		t.Fatalf("issue: %v, ttl %v", err, ttl)
	}
	if id, err := svc.ParseAccessToken(token); err != nil || id != 42 {
		t.Fatalf("parse = %d, %v", id, err)
	}

	sign := func(m jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
		s, err := jwt.NewWithClaims(m, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	valid := jwt.RegisteredClaims{Issuer: "loopback", Subject: "7", ExpiresAt: jwt.NewNumericDate(clk.now().Add(time.Minute))}
	noExp := valid
	noExp.ExpiresAt = nil
	otherIssuer := valid
	otherIssuer.Issuer = "someone"
	badSubject := valid
	badSubject.Subject = "abc"
	last := token[len(token)-1]
	flipped := token[:len(token)-1] + map[bool]string{true: "A", false: "B"}[last != 'A']

	for name, tok := range map[string]string{
		"empty":        "",
		"garbage":      "not-a-jwt",
		"tampered":     flipped,
		"wrong secret": sign(jwt.SigningMethodHS256, []byte("another-secret-another-secret-123"), valid),
		"wrong alg":    sign(jwt.SigningMethodHS512, secret, valid),
		"alg none":     sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid),
		"no expiry":    sign(jwt.SigningMethodHS256, secret, noExp),
		"issuer":       sign(jwt.SigningMethodHS256, secret, otherIssuer),
		"subject":      sign(jwt.SigningMethodHS256, secret, badSubject),
	} {
		if _, err := svc.ParseAccessToken(tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	clk.advance(15*time.Minute + time.Second)
	if _, err := svc.ParseAccessToken(token); err == nil {
		t.Error("expired token accepted")
	}
}

const testSpec = `
openapi: 3.0.3
info: {title: t, version: "1"}
security:
  - bearerAuth: []
paths:
  /me:
    get:
      responses: {"200": {description: ok}}
  /things/{id}:
    get:
      parameters: [{name: id, in: path, required: true, schema: {type: string}}]
      responses: {"200": {description: ok}}
  /open:
    get:
      security: []
      responses: {"200": {description: ok}}
  /either:
    get:
      security:
        - bearerAuth: []
        - {}
      responses: {"200": {description: ok}}
  /own:
    get:
      security:
        - bearerAuth: []
      responses: {"200": {description: ok}}
components:
  securitySchemes:
    bearerAuth: {type: http, scheme: bearer}
`

func TestMiddleware(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(testSpec))
	if err != nil {
		t.Fatal(err)
	}
	svc, clk := newService(t, nil)
	e := echo.New()
	e.Use(svc.Middleware(spec, "/api"))
	handler := func(c echo.Context) error {
		id, ok := auth.UserID(c.Request().Context())
		return c.String(http.StatusOK, fmt.Sprintf("%v:%d", ok, id))
	}
	for _, p := range []string{"/api/me", "/api/things/:id", "/api/open", "/api/own", "/api/either"} {
		e.GET(p, handler)
	}

	valid, _, _ := svc.IssueAccessToken(5)

	do := func(path, header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	good := "Bearer " + valid

	for _, path := range []string{"/api/me", "/api/things/1", "/api/own"} {
		if rec := do(path, good); rec.Code != http.StatusOK || rec.Body.String() != "true:5" {
			t.Errorf("%s valid: %d %q", path, rec.Code, rec.Body.String())
		}
		for name, header := range map[string]string{
			"missing":   "",
			"basic":     "Basic abc",
			"no token":  "Bearer",
			"tampered":  good + "x",
			"not a jwt": "Bearer abc",
		} {
			rec := do(path, header)
			var body gen.Error
			if rec.Code != http.StatusUnauthorized || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != gen.Unauthorized {
				t.Errorf("%s %s: %d %s", path, name, rec.Code, rec.Body.String())
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("%s %s: missing WWW-Authenticate", path, name)
			}
		}
	}

	if rec := do("/api/either", ""); rec.Code != http.StatusOK {
		t.Errorf("optional auth route: %d", rec.Code)
	}
	if rec := do("/api/open", ""); rec.Code != http.StatusOK || rec.Body.String() != "false:0" {
		t.Errorf("open route: %d %q", rec.Code, rec.Body.String())
	}

	clk.advance(16 * time.Minute)
	if rec := do("/api/me", good); rec.Code != http.StatusUnauthorized {
		t.Errorf("expired: %d", rec.Code)
	}
}

func TestRefreshRotation(t *testing.T) {
	db := newDB(t)
	svc, _ := newService(t, db)
	ctx := context.Background()
	u := newUser(t, db)
	device := "laptop"

	first, err := svc.IssueTokens(ctx, u.ID, &device)
	if err != nil {
		t.Fatal(err)
	}
	if db.Session.Query().Where(session.TokenHash(first.RefreshToken)).ExistX(ctx) {
		t.Fatal("refresh token stored in plain text")
	}

	second, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh token was not rotated")
	}
	if id, err := svc.ParseAccessToken(second.AccessToken); err != nil || id != u.ID {
		t.Errorf("new access token: %d, %v", id, err)
	}
	sessions := u.QuerySessions().AllX(ctx)
	revoked := 0
	for _, s := range sessions {
		if s.Revoked {
			revoked++
		}
		if s.Device == nil || *s.Device != device {
			t.Errorf("device = %v", s.Device)
		}
	}
	if len(sessions) != 2 || revoked != 1 {
		t.Errorf("sessions = %d, revoked = %d", len(sessions), revoked)
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken); err != auth.ErrInvalidToken {
		t.Errorf("reused token: %v", err)
	}
	if _, err := svc.Refresh(ctx, second.RefreshToken); err != nil {
		t.Errorf("new token should still work: %v", err)
	}
}

func TestRefreshRejects(t *testing.T) {
	db := newDB(t)
	svc, clk := newService(t, db)
	ctx := context.Background()
	u := newUser(t, db)

	if _, err := svc.Refresh(ctx, "unknown-token"); err != auth.ErrInvalidToken {
		t.Errorf("unknown: %v", err)
	}
	if _, err := svc.Refresh(ctx, ""); err != auth.ErrInvalidToken {
		t.Errorf("empty: %v", err)
	}

	tokens, err := svc.IssueTokens(ctx, u.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	clk.advance(time.Hour + time.Second)
	if _, err := svc.Refresh(ctx, tokens.RefreshToken); err != auth.ErrInvalidToken {
		t.Errorf("expired: %v", err)
	}
	if n := u.QuerySessions().CountX(ctx); n != 1 {
		t.Errorf("a rejected refresh created a session: %d", n)
	}
}

func TestRefreshConcurrent(t *testing.T) {
	db := newDB(t)
	svc, _ := newService(t, db)
	ctx := context.Background()
	u := newUser(t, db)
	tokens, err := svc.IssueTokens(ctx, u.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	const n = 8
	results := make(chan error, n)
	for range n {
		go func() {
			_, err := svc.Refresh(ctx, tokens.RefreshToken)
			results <- err
		}()
	}
	ok := 0
	for range n {
		if err := <-results; err == nil {
			ok++
		} else if err != auth.ErrInvalidToken && !strings.Contains(err.Error(), "serializ") {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d concurrent refreshes succeeded, want 1", ok)
	}
}
