package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/ChristophBe/loopback/backend/ent/identity"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

// fakeProvider is an OAuth provider without network access.
type fakeProvider struct {
	profile auth.Profile
	err     error
}

func (f *fakeProvider) AuthURL(state, redirectURL string) string {
	return "https://provider.test/authorize?" + url.Values{"state": {state}, "redirect_uri": {redirectURL}}.Encode()
}

func (f *fakeProvider) Exchange(_ context.Context, code, _ string) (auth.Profile, error) {
	if code != "good" {
		return auth.Profile{}, errors.New("bad code")
	}
	return f.profile, f.err
}

func stateOf(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func TestOAuthFlow(t *testing.T) {
	db := newDB(t)
	svc, clk := newService(t, db)
	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("Oauth-%d@Example.com", suffix)
	gh := &fakeProvider{profile: auth.Profile{ID: fmt.Sprintf("gh-%d", suffix), Email: email, EmailVerified: true}}
	svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "https://loopback.test/api/v1/", Providers: map[string]auth.Provider{"github": gh}})
	ctx := t.Context()

	start := func(provider string) (string, string) {
		authURL, cookie, err := svc.StartOAuth(provider)
		if err != nil {
			t.Fatalf("start %s: %v", provider, err)
		}
		return stateOf(t, authURL), cookie.Value
	}

	t.Run("start", func(t *testing.T) {
		authURL, cookie, err := svc.StartOAuth("github")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(authURL)
		if got := u.Query().Get("redirect_uri"); got != "https://loopback.test/api/v1/auth/oauth/github/callback" {
			t.Errorf("redirect_uri = %q", got)
		}
		if cookie.Name != auth.OAuthStateCookie || !cookie.HttpOnly || !cookie.Secure || cookie.Value == "" {
			t.Errorf("cookie = %+v", cookie)
		}
		if _, _, err := svc.StartOAuth("google"); !errors.Is(err, auth.ErrUnknownProvider) {
			t.Errorf("unconfigured provider: %v", err)
		}
	})

	t.Run("creates user and identity", func(t *testing.T) {
		state, nonce := start("github")
		tokens, err := svc.FinishOAuth(ctx, "github", "good", state, nonce)
		if err != nil {
			t.Fatal(err)
		}
		id, err := svc.ParseAccessToken(tokens.AccessToken)
		if err != nil {
			t.Fatal(err)
		}
		ident := db.Identity.Query().Where(identity.ProviderUserID(gh.profile.ID)).WithUser().OnlyX(ctx)
		if ident.Provider != identity.ProviderGithub || ident.Edges.User.ID != id || ident.Edges.User.Email != fmt.Sprintf("oauth-%d@example.com", suffix) {
			t.Errorf("identity = %+v, user = %+v", ident, ident.Edges.User)
		}
	})

	t.Run("signs in again with the same identity", func(t *testing.T) {
		state, nonce := start("github")
		tokens, err := svc.FinishOAuth(ctx, "github", "good", state, nonce)
		if err != nil {
			t.Fatal(err)
		}
		ident := db.Identity.Query().Where(identity.ProviderUserID(gh.profile.ID)).WithUser().OnlyX(ctx)
		if id, _ := svc.ParseAccessToken(tokens.AccessToken); id != ident.Edges.User.ID {
			t.Errorf("signed in as %d, want %d", id, ident.Edges.User.ID)
		}
	})

	t.Run("links to the user with the same verified email", func(t *testing.T) {
		existing := newUser(t, db)
		svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "https://loopback.test", Providers: map[string]auth.Provider{
			"github": &fakeProvider{profile: auth.Profile{ID: fmt.Sprintf("gh-link-%d", suffix), Email: existing.Email, EmailVerified: true}},
		}})
		state, nonce := start("github")
		tokens, err := svc.FinishOAuth(ctx, "github", "good", state, nonce)
		if err != nil {
			t.Fatal(err)
		}
		if id, _ := svc.ParseAccessToken(tokens.AccessToken); id != existing.ID {
			t.Errorf("signed in as %d, want %d", id, existing.ID)
		}
		if n := existing.QueryIdentities().CountX(ctx); n != 1 {
			t.Errorf("identities = %d, want 1", n)
		}
	})

	t.Run("rejects the state", func(t *testing.T) {
		svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "https://loopback.test", Providers: map[string]auth.Provider{"github": gh, "google": gh}})
		state, nonce := start("github")
		_, otherNonce := start("github")
		googleState, googleNonce := start("google")
		cases := map[string][3]string{
			"missing cookie":    {"github", state, ""},
			"wrong cookie":      {"github", state, otherNonce},
			"empty state":       {"github", "", nonce},
			"garbage state":     {"github", "a.b.c.d", nonce},
			"forged signature":  {"github", state[:len(state)-2] + "AA", nonce},
			"other provider":    {"google", state, nonce},
			"provider mismatch": {"github", googleState, googleNonce},
		}
		for name, c := range cases {
			if _, err := svc.FinishOAuth(ctx, c[0], "good", c[1], c[2]); !errors.Is(err, auth.ErrInvalidState) {
				t.Errorf("%s: err = %v, want ErrInvalidState", name, err)
			}
		}
		clk.advance(11 * time.Minute)
		if _, err := svc.FinishOAuth(ctx, "github", "good", state, nonce); !errors.Is(err, auth.ErrInvalidState) {
			t.Errorf("expired: err = %v", err)
		}
	})

	t.Run("rejects the provider response", func(t *testing.T) {
		svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "https://loopback.test", Providers: map[string]auth.Provider{
			"github": gh,
			"google": &fakeProvider{profile: auth.Profile{ID: "g1", Email: "x@example.com"}},
		}})
		state, nonce := start("github")
		if _, err := svc.FinishOAuth(ctx, "github", "bad", state, nonce); !errors.Is(err, auth.ErrOAuthExchange) {
			t.Errorf("bad code: %v", err)
		}
		state, nonce = start("google")
		if _, err := svc.FinishOAuth(ctx, "google", "good", state, nonce); !errors.Is(err, auth.ErrEmailUnverified) {
			t.Errorf("unverified email: %v", err)
		}
		if _, err := svc.FinishOAuth(ctx, "gitlab", "good", state, nonce); !errors.Is(err, auth.ErrUnknownProvider) {
			t.Errorf("unknown provider: %v", err)
		}
	})
}

func TestLoadOAuthConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cfg := auth.LoadOAuthConfig(env(nil))
	if cfg.BaseURL != "http://localhost:8080/api/v1" || len(cfg.Providers) != 0 {
		t.Errorf("defaults: %+v", cfg)
	}
	cfg = auth.LoadOAuthConfig(env(map[string]string{
		"OAUTH_BASE_URL":             "https://x.test/api/v1",
		"OAUTH_GITHUB_CLIENT_ID":     "id",
		"OAUTH_GITHUB_CLIENT_SECRET": "secret",
		"OAUTH_GOOGLE_CLIENT_ID":     "id-only",
	}))
	if cfg.BaseURL != "https://x.test/api/v1" || cfg.Providers["github"] == nil || cfg.Providers["google"] != nil {
		t.Errorf("configured: %+v", cfg)
	}
	u, _ := url.Parse(cfg.Providers["github"].AuthURL("st", "https://x.test/cb"))
	if u.Host != "github.com" || u.Query().Get("state") != "st" || u.Query().Get("client_id") != "id" || u.Query().Get("redirect_uri") != "https://x.test/cb" {
		t.Errorf("github auth url: %s", u)
	}
}
