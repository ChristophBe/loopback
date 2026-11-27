package http_test

import (
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

func newServer() nethttp.Handler {
	log := logrus.New()
	log.SetOutput(io.Discard)
	rules := gamification.Rules{XpPerLesson: 50, XpPerLevel: 500, DailyGoalLessons: 1}
	svc, err := auth.New(nil, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		panic(err)
	}
	e, err := http.New("test", rules, log, svc)
	if err != nil {
		panic(err)
	}
	return e
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newServer().ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/api/v1/healthz", nil))

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got gen.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != gen.Ok {
		t.Errorf("status = %q, want %q", got.Status, gen.Ok)
	}
}

func TestConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	newServer().ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/api/v1/config", nil))

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got gen.Config
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != "test" || got.Gamification.XpPerLesson != 50 {
		t.Errorf("config = %+v", got)
	}
}

func TestAuthEndpointsNotImplemented(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{nethttp.MethodPost, "/api/v1/auth/magic-link", `{"email":"a@example.com"}`},
		{nethttp.MethodGet, "/api/v1/auth/oauth/github", ""},
		{nethttp.MethodPost, "/api/v1/auth/device", ""},
		{nethttp.MethodPost, "/api/v1/auth/token", `{"deviceCode":"d"}`},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newServer().ServeHTTP(rec, req)

			if rec.Code != nethttp.StatusNotImplemented {
				t.Fatalf("status = %d, want 501", rec.Code)
			}
		})
	}
}

// The middleware matches routes by their Echo path, so every operation of the
// spec must be registered under exactly that path. Otherwise it would be open.
func TestSpecRoutesAreRegistered(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)
	svc, err := auth.New(nil, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	e, err := http.New("test", gamification.Rules{}, log, svc)
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, r := range e.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			echoPath := http.BasePath + strings.NewReplacer("{", ":", "}", "").Replace(path)
			if !registered[method+" "+echoPath] {
				t.Errorf("%s %s is not registered", method, echoPath)
			}
		}
	}
}

func TestTokenResponsesAreNotCached(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/magic-link", strings.NewReader(`{"email":"a@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	newServer().ServeHTTP(rec, req)
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}
