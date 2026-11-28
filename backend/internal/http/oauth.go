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
// Every answer after the state check clears the state cookie, so a state works once. A
// forged or mismatching state keeps the cookie, so that it cannot delete the cookie of another flow.
func (s *Server) FinishOAuth(ctx context.Context, req gen.FinishOAuthRequestObject) (gen.FinishOAuthResponseObject, error) {
	var cookieNonce string
	if req.Params.OauthState != nil {
		cookieNonce = *req.Params.OauthState
	}
	tokens, err := s.auth.FinishOAuth(ctx, string(req.Provider), req.Params.Code, req.Params.State, cookieNonce)
	clearCookie := s.auth.ClearOAuthStateCookie().String()
	badRequest := func(code gen.ErrorError) gen.FinishOAuthResponseObject {
		return gen.FinishOAuth400JSONResponse{Body: gen.Error{Error: code}, Headers: gen.FinishOAuth400ResponseHeaders{SetCookie: &clearCookie}}
	}
	switch {
	case errors.Is(err, auth.ErrUnknownProvider):
		return gen.FinishOAuth404JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.NotFound}}, nil
	case errors.Is(err, auth.ErrInvalidState):
		return gen.FinishOAuth400JSONResponse{Body: gen.Error{Error: gen.InvalidRequest}}, nil
	case errors.Is(err, auth.ErrOAuthExchange):
		return badRequest(gen.InvalidGrant), nil
	case errors.Is(err, auth.ErrEmailUnverified):
		return badRequest(gen.AccessDenied), nil
	case errors.Is(err, auth.ErrUnverifiedAccount):
		return gen.FinishOAuth409JSONResponse{Body: gen.Error{Error: gen.Conflict}, Headers: gen.FinishOAuth409ResponseHeaders{SetCookie: &clearCookie}}, nil
	case err != nil:
		return nil, err
	}
	return gen.FinishOAuth200JSONResponse{
		Body: gen.TokenResponse{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
		},
		Headers: gen.FinishOAuth200ResponseHeaders{SetCookie: &clearCookie},
	}, nil
}
