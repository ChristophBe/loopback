package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
)

// Option customises New.
type Option func(*Server, *echo.Echo)

// WithDevLogin registers POST /auth/dev-login, which signs in the fixed dev user
// without credentials. It is for local development only and is not part of the
// OpenAPI spec, so without this option the route answers 404.
func WithDevLogin() Option {
	return func(s *Server, e *echo.Echo) {
		e.POST(BasePath+"/auth/dev-login", s.devLogin)
	}
}

func (s *Server) devLogin(c echo.Context) error {
	tokens, err := s.auth.DevLogin(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, gen.TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.ExpiresIn.Seconds()),
	})
}
