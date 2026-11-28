package http_test

import (
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/ent/user"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

func TestDevLogin(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = db.Close() })
	svc, err := auth.New(db, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: 10 * time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)

	serve := func(e *echo.Echo, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/dev-login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	off, err := http.New("test", gamification.Rules{}, log, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(off, ""); rec.Code != nethttp.StatusNotFound {
		t.Errorf("flag off: %d, want 404", rec.Code)
	}

	on, err := http.New("test", gamification.Rules{}, log, svc, http.WithDevLogin())
	if err != nil {
		t.Fatal(err)
	}
	login := func(body string) (int, gen.TokenResponse) {
		rec := serve(on, body)
		var got gen.TokenResponse
		if rec.Code == nethttp.StatusOK && json.Unmarshal(rec.Body.Bytes(), &got) != nil {
			t.Fatalf("body: %s", rec.Body.String())
		}
		if rec.Code == nethttp.StatusOK && rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
		}
		return rec.Code, got
	}

	code, first := login("")
	if code != nethttp.StatusOK || first.RefreshToken == "" || first.ExpiresIn != 600 {
		t.Fatalf("login: %d %+v", code, first)
	}
	_, second := login(`{"email":"victim@example.com"}`)
	id1, err1 := svc.ParseAccessToken(first.AccessToken)
	id2, err2 := svc.ParseAccessToken(second.AccessToken)
	if err1 != nil || err2 != nil || id1 != id2 {
		t.Errorf("dev user is not fixed: %d (%v) vs %d (%v)", id1, err1, id2, err2)
	}
	if n := db.User.Query().CountX(t.Context()); n < 1 || db.User.Query().Where(user.Email("victim@example.com")).ExistX(t.Context()) {
		t.Errorf("dev login must not touch other accounts (users: %d)", n)
	}
}
