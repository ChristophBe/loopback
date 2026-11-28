package http

import (
	"context"
	"errors"

	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
)

// StartDeviceAuthorization creates a device code and a user code.
func (s *Server) StartDeviceAuthorization(ctx context.Context, _ gen.StartDeviceAuthorizationRequestObject) (gen.StartDeviceAuthorizationResponseObject, error) {
	d, err := s.auth.StartDeviceAuthorization(ctx)
	if err != nil {
		return nil, err
	}
	return gen.StartDeviceAuthorization200JSONResponse{
		DeviceCode:      d.DeviceCode,
		UserCode:        d.UserCode,
		VerificationUri: d.VerificationURI,
		Interval:        d.Interval,
		ExpiresIn:       int(d.ExpiresIn.Seconds()),
	}, nil
}

// ApproveDeviceCode approves the user code for the signed-in user.
func (s *Server) ApproveDeviceCode(ctx context.Context, req gen.ApproveDeviceCodeRequestObject) (gen.ApproveDeviceCodeResponseObject, error) {
	if req.Body == nil || req.Body.UserCode == "" {
		return gen.ApproveDeviceCode400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
	}
	userID, ok := auth.UserID(ctx)
	if !ok {
		return gen.ApproveDeviceCode401JSONResponse{Error: gen.Unauthorized}, nil
	}
	err := s.auth.ApproveDeviceCode(ctx, userID, req.Body.UserCode)
	if errors.Is(err, auth.ErrUserCodeNotFound) {
		return gen.ApproveDeviceCode404JSONResponse{Error: gen.NotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.ApproveDeviceCode204Response{}, nil
}

// ExchangeDeviceCode returns tokens once the device code is approved.
func (s *Server) ExchangeDeviceCode(ctx context.Context, req gen.ExchangeDeviceCodeRequestObject) (gen.ExchangeDeviceCodeResponseObject, error) {
	if req.Body == nil || req.Body.DeviceCode == "" {
		return deviceError(gen.InvalidRequest), nil
	}
	tokens, err := s.auth.ExchangeDeviceCode(ctx, req.Body.DeviceCode)
	switch {
	case errors.Is(err, auth.ErrAuthorizationPending):
		return deviceError(gen.AuthorizationPending), nil
	case errors.Is(err, auth.ErrDeviceCodeExpired):
		return deviceError(gen.ExpiredToken), nil
	case errors.Is(err, auth.ErrDeviceCodeInvalid):
		return deviceError(gen.InvalidGrant), nil
	case err != nil:
		return nil, err
	}
	return gen.ExchangeDeviceCode200JSONResponse{TokensJSONResponse: gen.TokensJSONResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
	}}, nil
}

func deviceError(code gen.ErrorError) gen.ExchangeDeviceCode400JSONResponse {
	return gen.ExchangeDeviceCode400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: code}}
}
