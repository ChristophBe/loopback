package http_test

import (
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

type deviceEnv struct {
	e   *echo.Echo
	svc *auth.Service
}

// expiredStore reports every grant as expired.
type expiredStore struct{ auth.DeviceStore }

func (expiredStore) Redeem(context.Context, string, time.Time) (int, error) {
	return 0, auth.ErrDeviceCodeExpired
}

func newDeviceEnv(t *testing.T, db *ent.Client) *deviceEnv {
	t.Helper()
	svc, err := auth.New(db, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: 10 * time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	env := &deviceEnv{svc: svc}
	log := logrus.New()
	log.SetOutput(io.Discard)
	env.e, err = http.New("test", gamification.Rules{}, log, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (d *deviceEnv) post(path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	d.e.ServeHTTP(rec, req)
	return rec
}

func (d *deviceEnv) start(t *testing.T) gen.DeviceAuthorization {
	t.Helper()
	rec := d.post("/auth/device", "", "")
	var got gen.DeviceAuthorization
	if rec.Code != nethttp.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	return got
}

func (d *deviceEnv) exchange(deviceCode string) *httptest.ResponseRecorder {
	return d.post("/auth/token", fmt.Sprintf(`{"deviceCode":%q}`, deviceCode), "")
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) gen.ErrorError {
	t.Helper()
	var body gen.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body.Error
}

func TestStartDeviceAuthorization(t *testing.T) {
	d := newDeviceEnv(t, nil)
	a, b := d.start(t), d.start(t)
	if a.DeviceCode == "" || a.DeviceCode == b.DeviceCode || a.UserCode == b.UserCode {
		t.Errorf("codes are not unique: %+v %+v", a, b)
	}
	if len(a.UserCode) != 9 || a.UserCode[4] != '-' {
		t.Errorf("user code = %q", a.UserCode)
	}
	if a.VerificationUri == "" || a.Interval < 1 || a.ExpiresIn != 600 {
		t.Errorf("response = %+v", a)
	}
}

func TestDeviceCodePendingAndExpired(t *testing.T) {
	d := newDeviceEnv(t, nil)
	a := d.start(t)

	rec := d.exchange(a.DeviceCode)
	if rec.Code != nethttp.StatusBadRequest || errorCode(t, rec) != gen.AuthorizationPending {
		t.Errorf("pending: %d %s", rec.Code, rec.Body.String())
	}

	d.svc.SetDeviceStore(expiredStore{})
	rec = d.exchange(a.DeviceCode)
	if rec.Code != nethttp.StatusBadRequest || errorCode(t, rec) != gen.ExpiredToken {
		t.Errorf("expired: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeviceCodeErrors(t *testing.T) {
	d := newDeviceEnv(t, nil)
	if rec := d.exchange("unknown"); rec.Code != nethttp.StatusBadRequest || errorCode(t, rec) != gen.InvalidGrant {
		t.Errorf("unknown: %d %s", rec.Code, rec.Body.String())
	}
	if rec := d.exchange(""); rec.Code != nethttp.StatusBadRequest || errorCode(t, rec) != gen.InvalidRequest {
		t.Errorf("empty: %d %s", rec.Code, rec.Body.String())
	}

	if rec := d.post("/auth/device/approve", `{"userCode":"ABCD-EFGH"}`, ""); rec.Code != nethttp.StatusUnauthorized {
		t.Errorf("approve without token: %d", rec.Code)
	}
	access, _, err := d.svc.IssueAccessToken(1)
	if err != nil {
		t.Fatal(err)
	}
	if rec := d.post("/auth/device/approve", `{"userCode":"ABCD-EFGH"}`, access); rec.Code != nethttp.StatusNotFound {
		t.Errorf("approve unknown code: %d %s", rec.Code, rec.Body.String())
	}
	if rec := d.post("/auth/device/approve", `{"userCode":""}`, access); rec.Code != nethttp.StatusBadRequest {
		t.Errorf("approve empty code: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeviceCodeApproval(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = db.Close() })
	d := newDeviceEnv(t, db)
	u := db.User.Create().SetEmail(fmt.Sprintf("device-%d@example.com", time.Now().UnixNano())).SaveX(t.Context())
	access, _, err := d.svc.IssueAccessToken(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	a := d.start(t)
	if rec := d.exchange(a.DeviceCode); errorCode(t, rec) != gen.AuthorizationPending {
		t.Fatalf("before approval: %d %s", rec.Code, rec.Body.String())
	}

	// Case, dashes and spaces of the user code do not matter.
	typed := strings.ToLower(strings.ReplaceAll(a.UserCode, "-", " "))
	rec := d.post("/auth/device/approve", fmt.Sprintf(`{"userCode":%q}`, typed), access)
	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}

	rec = d.exchange(a.DeviceCode)
	var got gen.TokenResponse
	if rec.Code != nethttp.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("exchange: %d %s", rec.Code, rec.Body.String())
	}
	if id, err := d.svc.ParseAccessToken(got.AccessToken); err != nil || id != u.ID {
		t.Errorf("access token: %d, %v", id, err)
	}
	if got.RefreshToken == "" || got.ExpiresIn != 600 {
		t.Errorf("response = %+v", got)
	}

	// The device code works once.
	if rec := d.exchange(a.DeviceCode); errorCode(t, rec) != gen.InvalidGrant {
		t.Errorf("reuse: %d %s", rec.Code, rec.Body.String())
	}
}
