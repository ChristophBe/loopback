// Package auth issues and verifies access tokens, rotates refresh tokens and
// protects the routes that the OpenAPI spec marks as secured.
package auth

import (
	"errors"
	"fmt"
	"time"
)

const (
	// DefaultAccessTTL is the lifetime of an access token.
	DefaultAccessTTL = 15 * time.Minute
	// DefaultRefreshTTL is the lifetime of a refresh token.
	DefaultRefreshTTL = 30 * 24 * time.Hour
	// MinSecretLength is the shortest accepted HS256 secret in bytes.
	MinSecretLength = 32
)

// Config holds the token settings.
type Config struct {
	// Secret signs the access tokens (HS256).
	Secret     []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// LoadConfig reads JWT_SECRET (required), JWT_ACCESS_TTL and
// REFRESH_TOKEN_TTL (Go durations such as 15m) with getenv.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Secret:     []byte(getenv("JWT_SECRET")),
		AccessTTL:  DefaultAccessTTL,
		RefreshTTL: DefaultRefreshTTL,
	}
	for _, d := range []struct {
		key string
		dst *time.Duration
	}{{"JWT_ACCESS_TTL", &cfg.AccessTTL}, {"REFRESH_TOKEN_TTL", &cfg.RefreshTTL}} {
		v := getenv(d.key)
		if v == "" {
			continue
		}
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", d.key, err)
		}
		*d.dst = parsed
	}
	return cfg, cfg.Validate()
}

// Validate rejects a missing or weak secret and non-positive lifetimes.
func (c Config) Validate() error {
	switch {
	case len(c.Secret) == 0:
		return errors.New("JWT_SECRET is not set")
	case len(c.Secret) < MinSecretLength:
		return fmt.Errorf("JWT_SECRET must be at least %d bytes", MinSecretLength)
	case c.AccessTTL < time.Second:
		return errors.New("access token lifetime must be at least 1s")
	case c.RefreshTTL < time.Second:
		return errors.New("refresh token lifetime must be at least 1s")
	}
	return nil
}
