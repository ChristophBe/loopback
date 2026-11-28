package http

import (
	"context"
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

// WithMagicLinks enables the email sign-in. Without it the endpoints answer
// 501.
func WithMagicLinks(m *auth.MagicLinks) Option {
	return func(s *Server, _ *echo.Echo) { s.magicLinks = m }
}

// RequestMagicLink emails a sign-in link. The answer is the same whether or
// not an account exists.
func (s *Server) RequestMagicLink(ctx context.Context, req gen.RequestMagicLinkRequestObject) (gen.RequestMagicLinkResponseObject, error) {
	if s.magicLinks == nil {
		return nil, echo.ErrNotImplemented
	}
	if req.Body == nil {
		return gen.RequestMagicLink400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	}
	err := s.magicLinks.Request(ctx, string(req.Body.Email))
	if errors.Is(err, auth.ErrInvalidEmail) {
		return gen.RequestMagicLink400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.RequestMagicLink202Response{}, nil
}

// VerifyMagicLink signs the learner in with the token of an emailed link.
func (s *Server) VerifyMagicLink(ctx context.Context, req gen.VerifyMagicLinkRequestObject) (gen.VerifyMagicLinkResponseObject, error) {
	if s.magicLinks == nil {
		return nil, echo.ErrNotImplemented
	}
	if req.Body == nil || req.Body.Token == "" {
		return gen.VerifyMagicLink400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	}
	tokens, err := s.magicLinks.Verify(ctx, req.Body.Token)
	if errors.Is(err, auth.ErrInvalidToken) {
		return gen.VerifyMagicLink401JSONResponse{Error: gen.InvalidGrant}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.VerifyMagicLink200JSONResponse{TokensJSONResponse: gen.TokensJSONResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
	}}, nil
}
