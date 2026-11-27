package http_test

import (
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

func newServer() nethttp.Handler {
	log := logrus.New()
	log.SetOutput(io.Discard)
	rules := gamification.Rules{XpPerLesson: 50, XpPerLevel: 500, DailyGoalLessons: 1}
	e, err := http.New("test", rules, log, nil)
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
