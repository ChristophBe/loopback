package http

import (
	"context"
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

// The other auth handlers are stubs until the sign-in flows land.

// StartOAuth is not implemented yet.
func (s *Server) StartOAuth(context.Context, gen.StartOAuthRequestObject) (gen.StartOAuthResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// RefreshTokens rotates the refresh token and returns a new token pair.
func (s *Server) RefreshTokens(ctx context.Context, req gen.RefreshTokensRequestObject) (gen.RefreshTokensResponseObject, error) {
	if req.Body == nil || req.Body.RefreshToken == "" {
		return gen.RefreshTokens400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	}
	tokens, err := s.auth.Refresh(ctx, req.Body.RefreshToken)
	if errors.Is(err, auth.ErrInvalidToken) {
		return gen.RefreshTokens401JSONResponse{Error: gen.InvalidGrant}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.RefreshTokens200JSONResponse{TokensJSONResponse: gen.TokensJSONResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
	}}, nil
}
