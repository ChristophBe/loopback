package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

type fakeProvider struct{ profile auth.Profile }

func (f fakeProvider) AuthURL(state, _ string) string {
	return "https://provider.test/authorize?state=" + url.QueryEscape(state)
}

func (f fakeProvider) Exchange(_ context.Context, code, _ string) (auth.Profile, error) {
	if code != "good" {
		return auth.Profile{}, fmt.Errorf("%w: bad code", auth.ErrOAuthExchange)
	}
	return f.profile, nil
}

func TestOAuthEndpoints(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db := enttest.Open(t, "postgres", dbURL)
	t.Cleanup(func() { _ = db.Close() })
	svc, err := auth.New(db, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: 10 * time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	profile := auth.Profile{ID: fmt.Sprintf("g-%d", time.Now().UnixNano()), Email: fmt.Sprintf("http-oauth-%d@example.com", time.Now().UnixNano()), EmailVerified: true}
	svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "http://localhost/api/v1", Providers: map[string]auth.Provider{"github": fakeProvider{profile}}})
	log := logrus.New()
	log.SetOutput(io.Discard)
	e, err := http.New("test", gamification.Rules{}, log, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	get := func(target string, cookie *nethttp.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(nethttp.MethodGet, target, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	rec := get("/api/v1/auth/oauth/github", nil)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if rec.Code != nethttp.StatusFound || err != nil || loc.Host != "provider.test" {
		t.Fatalf("start: %d %v", rec.Code, rec.Header())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "oauth_state" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %v", cookies)
	}
	state := loc.Query().Get("state")
	callback := "/api/v1/auth/oauth/github/callback?code=good&state=" + url.QueryEscape(state)

	for _, provider := range []string{"gitlab", "google"} {
		if rec := get("/api/v1/auth/oauth/"+provider, nil); rec.Code != nethttp.StatusNotFound || !strings.Contains(rec.Body.String(), "not_found") {
			t.Errorf("start %s: %d %s", provider, rec.Code, rec.Body.String())
		}
		if rec := get("/api/v1/auth/oauth/"+provider+"/callback?code=good&state=x", nil); rec.Code != nethttp.StatusNotFound {
			t.Errorf("callback %s: %d", provider, rec.Code)
		}
	}
	for name, c := range map[string]struct {
		target string
		cookie *nethttp.Cookie
	}{
		"no cookie":     {callback, nil},
		"wrong cookie":  {callback, &nethttp.Cookie{Name: "oauth_state", Value: "other"}}, //nolint:gosec // test request cookie
		"tampered":      {callback + "x", cookies[0]},
		"missing state": {"/api/v1/auth/oauth/github/callback?code=good", cookies[0]},
	} {
		if rec := get(c.target, c.cookie); rec.Code != nethttp.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := get("/api/v1/auth/oauth/github/callback?code=bad&state="+url.QueryEscape(state), cookies[0]); rec.Code != nethttp.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Errorf("bad code: %d %s", rec.Code, rec.Body.String())
	}

	rec = get(callback, cookies[0])
	var tokens gen.TokenResponse
	if rec.Code != nethttp.StatusOK || json.Unmarshal(rec.Body.Bytes(), &tokens) != nil || tokens.RefreshToken == "" {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := svc.ParseAccessToken(tokens.AccessToken); err != nil {
		t.Errorf("access token: %v", err)
	}
	if n := db.Identity.Query().CountX(t.Context()); n < 1 {
		t.Errorf("identities = %d", n)
	}
	assertCleared(t, rec)

	// An unverified user with the same email is not linked: 409.
	unverified := db.User.Create().SetEmail(fmt.Sprintf("http-oauth-unverified-%d@example.com", time.Now().UnixNano())).SaveX(t.Context())
	svc.ConfigureOAuth(auth.OAuthConfig{BaseURL: "http://localhost/api/v1", Providers: map[string]auth.Provider{
		"github": fakeProvider{auth.Profile{ID: fmt.Sprintf("g-unv-%d", time.Now().UnixNano()), Email: unverified.Email, EmailVerified: true}},
	}})
	rec = get("/api/v1/auth/oauth/github", nil)
	loc, _ = url.Parse(rec.Header().Get("Location"))
	cookie := rec.Result().Cookies()[0]
	rec = get("/api/v1/auth/oauth/github/callback?code=good&state="+url.QueryEscape(loc.Query().Get("state")), cookie)
	if rec.Code != nethttp.StatusConflict || !strings.Contains(rec.Body.String(), "conflict") {
		t.Fatalf("unverified: %d %s", rec.Code, rec.Body.String())
	}
	assertCleared(t, rec)
	if n := unverified.QueryIdentities().CountX(t.Context()); n != 0 {
		t.Errorf("identities of the unverified user = %d", n)
	}

	// A forged state keeps the cookie, so it cannot delete the cookie of another flow.
	rec = get("/api/v1/auth/oauth/github/callback?code=good&state=x", cookie)
	if rec.Code != nethttp.StatusBadRequest || len(rec.Header()["Set-Cookie"]) != 0 {
		t.Fatalf("bad state: %d %v", rec.Code, rec.Header())
	}
}

func assertCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" && c.Value == "" && c.MaxAge < 0 {
			return
		}
	}
	t.Errorf("oauth_state cookie not cleared: %v", rec.Header()["Set-Cookie"])
}
