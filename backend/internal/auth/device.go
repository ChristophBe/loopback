package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	// DeviceCodeTTL is how long a device code can be approved and exchanged.
	DeviceCodeTTL = 10 * time.Minute
	// DevicePollInterval is the minimum number of seconds between polls.
	DevicePollInterval = 5
	// DefaultVerificationURI is where the learner enters the user code.
	DefaultVerificationURI = "http://localhost:5173/device"

	// userCodeAlphabet leaves out characters that are easy to confuse.
	userCodeAlphabet = "BCDFGHJKMNPQRSTVWXYZ23456789"
	userCodeLength   = 8
)

var (
	// ErrAuthorizationPending means the learner has not approved the code yet.
	ErrAuthorizationPending = errors.New("authorization pending")
	// ErrDeviceCodeExpired means the device code ran out before it was exchanged.
	ErrDeviceCodeExpired = errors.New("device code expired")
	// ErrDeviceCodeInvalid means the code is unknown or was already exchanged.
	ErrDeviceCodeInvalid = errors.New("invalid device code")
	// ErrUserCodeNotFound means no pending device code has this user code.
	ErrUserCodeNotFound = errors.New("user code not found")
)

// DeviceAuthorization is what the plugin shows and polls with.
type DeviceAuthorization struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        int
	ExpiresIn       time.Duration
}

// DeviceGrant is one pending device-code sign-in.
type DeviceGrant struct {
	// DeviceCodeHash is the hash of the secret device code.
	DeviceCodeHash string
	// UserCode is the normalized code the learner enters.
	UserCode  string
	ExpiresAt time.Time
	// UserID is set once the learner approved the code.
	UserID int
}

// DeviceStore keeps device grants until they are exchanged or expire.
type DeviceStore interface {
	// Create stores a new grant.
	Create(ctx context.Context, g DeviceGrant) error
	// Approve assigns the user to the unexpired grant with the user code.
	Approve(ctx context.Context, userCode string, userID int, now time.Time) error
	// Redeem removes the approved grant and returns its user. It returns
	// ErrAuthorizationPending, ErrDeviceCodeExpired or ErrDeviceCodeInvalid.
	Redeem(ctx context.Context, deviceCodeHash string, now time.Time) (userID int, err error)
}

// MemoryDeviceStore is an in-memory DeviceStore. Grants are lost on restart
// and are not shared between API instances, which is fine for a single
// instance because a grant lives for minutes.
type MemoryDeviceStore struct {
	mu     sync.Mutex
	grants map[string]*DeviceGrant // by device code hash
}

// NewMemoryDeviceStore returns an empty store.
func NewMemoryDeviceStore() *MemoryDeviceStore {
	return &MemoryDeviceStore{grants: map[string]*DeviceGrant{}}
}

// Create implements DeviceStore. It also drops expired grants.
func (m *MemoryDeviceStore) Create(_ context.Context, g DeviceGrant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, old := range m.grants {
		if !old.ExpiresAt.After(now) {
			delete(m.grants, k)
		}
	}
	m.grants[g.DeviceCodeHash] = &g
	return nil
}

// Approve implements DeviceStore.
func (m *MemoryDeviceStore) Approve(_ context.Context, userCode string, userID int, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, g := range m.grants {
		if g.UserCode == userCode && g.ExpiresAt.After(now) {
			g.UserID = userID
			return nil
		}
	}
	return ErrUserCodeNotFound
}

// Redeem implements DeviceStore.
func (m *MemoryDeviceStore) Redeem(_ context.Context, hash string, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.grants[hash]
	if !ok {
		return 0, ErrDeviceCodeInvalid
	}
	if !g.ExpiresAt.After(now) {
		delete(m.grants, hash)
		return 0, ErrDeviceCodeExpired
	}
	if g.UserID == 0 {
		return 0, ErrAuthorizationPending
	}
	delete(m.grants, hash)
	return g.UserID, nil
}

// SetDeviceStore replaces the store of device grants.
func (s *Service) SetDeviceStore(store DeviceStore) { s.devices = store }

// SetVerificationURI sets the page where learners enter the user code.
func (s *Service) SetVerificationURI(uri string) { s.verificationURI = uri }

// StartDeviceAuthorization creates a device code and a user code.
func (s *Service) StartDeviceAuthorization(ctx context.Context) (DeviceAuthorization, error) {
	deviceCode, hash, err := newRefreshToken()
	if err != nil {
		return DeviceAuthorization{}, err
	}
	raw := make([]byte, userCodeLength)
	if _, err := rand.Read(raw); err != nil {
		return DeviceAuthorization{}, err
	}
	code := make([]byte, userCodeLength)
	for i, b := range raw {
		code[i] = userCodeAlphabet[int(b)%len(userCodeAlphabet)]
	}
	userCode := string(code)
	err = s.devices.Create(ctx, DeviceGrant{
		DeviceCodeHash: hash,
		UserCode:       userCode,
		ExpiresAt:      s.now().Add(DeviceCodeTTL),
	})
	if err != nil {
		return DeviceAuthorization{}, err
	}
	return DeviceAuthorization{
		DeviceCode:      deviceCode,
		UserCode:        userCode[:4] + "-" + userCode[4:],
		VerificationURI: s.verificationURI,
		Interval:        DevicePollInterval,
		ExpiresIn:       DeviceCodeTTL,
	}, nil
}

// ApproveDeviceCode lets the user sign in the device with the user code. The
// code is matched ignoring case, dashes and spaces.
func (s *Service) ApproveDeviceCode(ctx context.Context, userID int, userCode string) error {
	normalized := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(userCode))
	return s.devices.Approve(ctx, normalized, userID, s.now())
}

// ExchangeDeviceCode returns tokens once the user approved the device code.
// Before that it returns ErrAuthorizationPending.
func (s *Service) ExchangeDeviceCode(ctx context.Context, deviceCode string) (Tokens, error) {
	userID, err := s.devices.Redeem(ctx, hashToken(deviceCode), s.now())
	if err != nil {
		return Tokens{}, err
	}
	device := "plugin"
	return s.IssueTokens(ctx, userID, &device)
}
