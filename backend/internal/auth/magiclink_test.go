package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/ent/identity"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

type sentMail struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
	err  error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentMail{to, subject, body})
	return nil
}

// token returns the token of the last link that was mailed.
func (f *fakeMailer) token(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("no mail was sent")
	}
	for _, field := range strings.Fields(f.sent[len(f.sent)-1].body) {
		if u, err := url.Parse(field); err == nil && u.Host == "app.example" {
			return u.Query().Get("token")
		}
	}
	t.Fatalf("no link in %q", f.sent[len(f.sent)-1].body)
	return ""
}

func newMagicLinks(t *testing.T) (*auth.MagicLinks, *auth.Service, *ent.Client, *fakeMailer, *clock) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = db.Close() })
	svc, clk := newService(t, db)
	mailer := &fakeMailer{}
	m, err := auth.NewMagicLinks(svc, mailer, auth.MagicLinkConfig{BaseURL: "https://app.example/", TTL: 10 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return m, svc, db, mailer, clk
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

func TestMagicLinkSignIn(t *testing.T) {
	m, svc, db, mailer, _ := newMagicLinks(t)
	ctx := t.Context()
	email := uniqueEmail("learner")

	if err := m.Request(ctx, "  "+strings.ToUpper(email)+" "); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].to != email || !strings.Contains(mailer.sent[0].body, "https://app.example/auth/verify?token=") {
		t.Fatalf("mail = %+v", mailer.sent)
	}
	token := mailer.token(t)

	tokens, err := m.Verify(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.ParseAccessToken(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	user := db.User.GetX(ctx, id)
	if user.Email != email {
		t.Errorf("email = %q, want %q", user.Email, email)
	}
	if !db.Identity.Query().Where(identity.ProviderEQ(identity.ProviderEmail), identity.ProviderUserID(email)).ExistX(ctx) {
		t.Error("no email identity")
	}
	if _, err := svc.Refresh(ctx, tokens.RefreshToken); err != nil {
		t.Errorf("refresh token of the new session: %v", err)
	}

	// The link works once.
	if _, err := m.Verify(ctx, token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("second use: %v, want ErrInvalidToken", err)
	}

	// A second sign-in reuses the account.
	if err := m.Request(ctx, email); err != nil {
		t.Fatal(err)
	}
	again, err := m.Verify(ctx, mailer.token(t))
	if err != nil {
		t.Fatal(err)
	}
	if id2, _ := svc.ParseAccessToken(again.AccessToken); id2 != id {
		t.Errorf("user id = %d, want %d", id2, id)
	}
}

func TestMagicLinkConcurrentUseSucceedsOnce(t *testing.T) {
	m, _, _, mailer, _ := newMagicLinks(t)
	ctx := t.Context()
	if err := m.Request(ctx, uniqueEmail("race")); err != nil {
		t.Fatal(err)
	}
	token := mailer.token(t)

	var wg sync.WaitGroup
	results := make(chan error, 6)
	for range cap(results) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Verify(ctx, token)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d sign-ins succeeded, want 1", ok)
	}
}

func TestMagicLinkRejectsBadTokens(t *testing.T) {
	m, _, _, mailer, clk := newMagicLinks(t)
	ctx := t.Context()
	if err := m.Request(ctx, uniqueEmail("bad")); err != nil {
		t.Fatal(err)
	}
	token := mailer.token(t)
	payload, sig, _ := strings.Cut(token, ".")

	for name, tok := range map[string]string{
		"empty":         "",
		"no signature":  payload,
		"wrong sig":     payload + "." + strings.Repeat("A", len(sig)),
		"tampered body": "x" + payload + "." + sig,
		"garbage":       "a.b",
	} {
		if _, err := m.Verify(ctx, tok); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("%s: %v, want ErrInvalidToken", name, err)
		}
	}

	clk.advance(10 * time.Minute)
	if _, err := m.Verify(ctx, token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("expired: %v, want ErrInvalidToken", err)
	}
}

func TestMagicLinkRequestErrors(t *testing.T) {
	m, _, _, mailer, _ := newMagicLinks(t)
	for _, in := range []string{"", "nope", "Name <a@b.example>", "a@b.example\r\nBcc: x@y.example"} {
		if err := m.Request(t.Context(), in); !errors.Is(err, auth.ErrInvalidEmail) {
			t.Errorf("%q: %v, want ErrInvalidEmail", in, err)
		}
	}
	if len(mailer.sent) != 0 {
		t.Errorf("sent %d mails for invalid addresses", len(mailer.sent))
	}
	mailer.err = errors.New("smtp down")
	if err := m.Request(t.Context(), "a@b.example"); err == nil {
		t.Error("mailer failure was swallowed")
	}
}

func TestMagicLinkConfig(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cfg, err := auth.LoadMagicLinkConfig(get(map[string]string{"WEB_BASE_URL": "http://localhost:5173", "MAGIC_LINK_TTL": "5m"}))
	if err != nil || cfg.TTL != 5*time.Minute {
		t.Errorf("cfg = %+v, %v", cfg, err)
	}
	cfg, err = auth.LoadMagicLinkConfig(get(map[string]string{"WEB_BASE_URL": "http://localhost:5173"}))
	if err != nil || cfg.TTL != auth.DefaultMagicLinkTTL {
		t.Errorf("default ttl: %+v, %v", cfg, err)
	}
	for name, env := range map[string]map[string]string{
		"missing url": {},
		"bad url":     {"WEB_BASE_URL": "localhost"},
		"bad ttl":     {"WEB_BASE_URL": "http://x.example", "MAGIC_LINK_TTL": "soon"},
		"zero ttl":    {"WEB_BASE_URL": "http://x.example", "MAGIC_LINK_TTL": "0s"},
	} {
		if _, err := auth.LoadMagicLinkConfig(get(env)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := auth.NewMagicLinks(nil, &fakeMailer{}, auth.MagicLinkConfig{}); err == nil {
		t.Error("nil service accepted")
	}
}
