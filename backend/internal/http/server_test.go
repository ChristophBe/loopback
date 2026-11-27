package http_test

import (
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
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
	return http.New("test", rules, log)
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
