package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "loopback"

// ErrInvalidToken is returned for every token that cannot be accepted:
// malformed, wrongly signed, expired, revoked or unknown.
var ErrInvalidToken = errors.New("invalid token")

// IssueAccessToken returns a signed HS256 JWT for the user and its lifetime.
func (s *Service) IssueAccessToken(userID int) (string, time.Duration, error) {
	now := s.now()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   strconv.Itoa(userID),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.Secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign access token: %w", err)
	}
	return signed, s.cfg.AccessTTL, nil
}

// ParseAccessToken verifies the signature (HS256 only), issuer and expiry and
// returns the user id.
func (s *Service) ParseAccessToken(token string) (int, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return s.cfg.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return 0, ErrInvalidToken
	}
	id, err := strconv.Atoi(claims.Subject)
	if err != nil || id <= 0 {
		return 0, ErrInvalidToken
	}
	return id, nil
}

// newRefreshToken returns a random token and its hash. Only the hash is
// stored.
func newRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

// hashToken is the SHA-256 of the token in hex. The token has 256 bits of
// entropy, so a fast hash is enough and the database lookup by hash needs no
// constant-time comparison.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
