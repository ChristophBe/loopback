package http_test

import (
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
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

func TestRefreshEndpoint(t *testing.T) {
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
	e, err := http.New("test", gamification.Rules{}, log, svc, db)
	if err != nil {
		t.Fatal(err)
	}

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	refresh := func(token string) *httptest.ResponseRecorder {
		return post(fmt.Sprintf(`{"refreshToken":%q}`, token))
	}

	u := db.User.Create().SetEmail(fmt.Sprintf("refresh-%d@example.com", time.Now().UnixNano())).SaveX(t.Context())
	issued, err := svc.IssueTokens(t.Context(), u.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	rec := refresh(issued.RefreshToken)
	var got gen.TokenResponse
	if rec.Code != nethttp.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
	if id, err := svc.ParseAccessToken(got.AccessToken); err != nil || id != u.ID {
		t.Errorf("access token: %d, %v", id, err)
	}
	if got.ExpiresIn != 600 || got.RefreshToken == "" || got.RefreshToken == issued.RefreshToken {
		t.Errorf("response = %+v", got)
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"reused":  refresh(issued.RefreshToken),
		"unknown": refresh("nope"),
	} {
		var body gen.Error
		if rec.Code != nethttp.StatusUnauthorized || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != gen.InvalidGrant {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := refresh(""); rec.Code != nethttp.StatusBadRequest {
		t.Errorf("empty token: %d", rec.Code)
	}
	if rec := post("{"); rec.Code != nethttp.StatusBadRequest {
		t.Errorf("malformed body: %d", rec.Code)
	}
	// The new token pair works for another rotation.
	if rec := refresh(got.RefreshToken); rec.Code != nethttp.StatusOK {
		t.Errorf("second rotation: %d", rec.Code)
	}
}
