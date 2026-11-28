package auth_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ChristophBe/loopback/backend/internal/auth"
)

func TestDeviceCodeExpiry(t *testing.T) {
	svc, err := auth.New(nil, auth.Config{Secret: secret, AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Now()}
	auth.SetClock(svc, c.now)

	d, err := svc.StartDeviceAuthorization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ExchangeDeviceCode(t.Context(), d.DeviceCode); !errors.Is(err, auth.ErrAuthorizationPending) {
		t.Errorf("before expiry: %v", err)
	}

	c.advance(auth.DeviceCodeTTL + time.Second)
	if err := svc.ApproveDeviceCode(t.Context(), 1, strings.ToLower(d.UserCode)); !errors.Is(err, auth.ErrUserCodeNotFound) {
		t.Errorf("approve after expiry: %v", err)
	}
	if _, err := svc.ExchangeDeviceCode(t.Context(), d.DeviceCode); !errors.Is(err, auth.ErrDeviceCodeExpired) {
		t.Errorf("exchange after expiry: %v", err)
	}
	// The expired code is removed.
	if _, err := svc.ExchangeDeviceCode(t.Context(), d.DeviceCode); !errors.Is(err, auth.ErrDeviceCodeInvalid) {
		t.Errorf("exchange again: %v", err)
	}
}

func TestApproveKeepsFirstApprover(t *testing.T) {
	svc, err := auth.New(nil, auth.Config{Secret: secret, AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	d, err := svc.StartDeviceAuthorization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApproveDeviceCode(t.Context(), 1, d.UserCode); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApproveDeviceCode(t.Context(), 2, d.UserCode); !errors.Is(err, auth.ErrUserCodeNotFound) {
		t.Errorf("second approval: %v", err)
	}
}

func TestMemoryDeviceStoreRejectsDuplicateUserCode(t *testing.T) {
	store, now := auth.NewMemoryDeviceStore(), time.Now()
	g := auth.DeviceGrant{DeviceCodeHash: "a", UserCode: "ABCDEFGH", ExpiresAt: now.Add(time.Minute)}
	if err := store.Create(t.Context(), g, now); err != nil {
		t.Fatal(err)
	}
	g.DeviceCodeHash = "b"
	if err := store.Create(t.Context(), g, now); !errors.Is(err, auth.ErrUserCodeTaken) {
		t.Errorf("duplicate: %v", err)
	}
	// Once the first grant expired the code can be reused.
	if err := store.Create(t.Context(), g, now.Add(2*time.Minute)); err != nil {
		t.Errorf("after expiry: %v", err)
	}
}
