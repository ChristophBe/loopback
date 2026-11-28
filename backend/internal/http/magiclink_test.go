package http_test

import (
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
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

type captureMailer struct {
	mu   sync.Mutex
	to   string
	body string
}

func (c *captureMailer) Send(_ context.Context, to, _, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.to, c.body = to, body
	return nil
}

func TestMagicLinkEndpoints(t *testing.T) {
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
	mailer := &captureMailer{}
	ml, err := auth.NewMagicLinks(svc, mailer, auth.MagicLinkConfig{BaseURL: "https://app.example", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	e, err := http.New("test", gamification.Rules{}, log, svc, nil, http.WithMagicLinks(ml))
	if err != nil {
		t.Fatal(err)
	}
	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	email := "http-" + time.Now().Format("150405.000000000") + "@example.com"
	if rec := post("magic-link", `{"email":"`+email+`"}`); rec.Code != nethttp.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	if mailer.to != email {
		t.Fatalf("mailed to %q", mailer.to)
	}
	var token string
	for _, f := range strings.Fields(mailer.body) {
		if u, err := url.Parse(f); err == nil && u.Host == "app.example" {
			token = u.Query().Get("token")
		}
	}
	if token == "" {
		t.Fatalf("no link in %q", mailer.body)
	}

	rec := post("magic-link/verify", `{"token":"`+token+`"}`)
	var got gen.TokenResponse
	if rec.Code != nethttp.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.AccessToken == "" || got.RefreshToken == "" {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := svc.ParseAccessToken(got.AccessToken); err != nil {
		t.Errorf("access token: %v", err)
	}

	rec = post("magic-link/verify", `{"token":"`+token+`"}`)
	var apiErr gen.Error
	if rec.Code != nethttp.StatusUnauthorized || json.Unmarshal(rec.Body.Bytes(), &apiErr) != nil || apiErr.Error != gen.InvalidGrant {
		t.Errorf("reuse: %d %s", rec.Code, rec.Body.String())
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"bad email":     post("magic-link", `{"email":"not-an-email"}`),
		"missing email": post("magic-link", `{}`),
		"missing token": post("magic-link/verify", `{}`),
		"empty token":   post("magic-link/verify", `{"token":""}`),
	} {
		if rec.Code != nethttp.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestMagicLinkDisabledWithoutConfig(t *testing.T) {
	svc, err := auth.New(nil, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	e, err := http.New("test", gamification.Rules{}, log, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"magic-link", "magic-link/verify"} {
		req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/"+path, strings.NewReader(`{"email":"a@b.example","token":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != nethttp.StatusNotImplemented {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}
