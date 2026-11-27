package http

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
)

// The auth handlers are stubs until the auth implementation lands.

// RequestMagicLink is not implemented yet.
func (s *Server) RequestMagicLink(context.Context, gen.RequestMagicLinkRequestObject) (gen.RequestMagicLinkResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// StartOAuth is not implemented yet.
func (s *Server) StartOAuth(context.Context, gen.StartOAuthRequestObject) (gen.StartOAuthResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// StartDeviceAuthorization is not implemented yet.
func (s *Server) StartDeviceAuthorization(context.Context, gen.StartDeviceAuthorizationRequestObject) (gen.StartDeviceAuthorizationResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// ExchangeDeviceCode is not implemented yet.
func (s *Server) ExchangeDeviceCode(context.Context, gen.ExchangeDeviceCodeRequestObject) (gen.ExchangeDeviceCodeResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// RefreshTokens is not implemented yet.
func (s *Server) RefreshTokens(context.Context, gen.RefreshTokensRequestObject) (gen.RefreshTokensResponseObject, error) {
	return nil, echo.ErrNotImplemented
}
