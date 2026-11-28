package http

import (
	"context"
	"errors"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

// StartOAuth redirects to the provider and sets the state cookie.
func (s *Server) StartOAuth(_ context.Context, req gen.StartOAuthRequestObject) (gen.StartOAuthResponseObject, error) {
	url, cookie, err := s.auth.StartOAuth(string(req.Provider))
	if errors.Is(err, auth.ErrUnknownProvider) {
		return gen.StartOAuth404JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.NotFound}}, nil
	}
	if err != nil {
		return nil, err
	}
	setCookie := cookie.String()
	return gen.StartOAuth302Response{Headers: gen.StartOAuth302ResponseHeaders{Location: &url, SetCookie: &setCookie}}, nil
}

// FinishOAuth checks the state, signs the provider account in and returns tokens.
func (s *Server) FinishOAuth(ctx context.Context, req gen.FinishOAuthRequestObject) (gen.FinishOAuthResponseObject, error) {
	var cookieNonce string
	if req.Params.OauthState != nil {
		cookieNonce = *req.Params.OauthState
	}
	tokens, err := s.auth.FinishOAuth(ctx, string(req.Provider), req.Params.Code, req.Params.State, cookieNonce)
	switch {
	case errors.Is(err, auth.ErrUnknownProvider):
		return gen.FinishOAuth404JSONResponse{Error: gen.NotFound}, nil
	case errors.Is(err, auth.ErrInvalidState):
		return gen.FinishOAuth400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	case errors.Is(err, auth.ErrOAuthExchange):
		return gen.FinishOAuth400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidGrant}}, nil
	case errors.Is(err, auth.ErrEmailUnverified):
		return gen.FinishOAuth400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.AccessDenied}}, nil
	case err != nil:
		return nil, err
	}
	return gen.FinishOAuth200JSONResponse{TokensJSONResponse: gen.TokensJSONResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
	}}, nil
}
