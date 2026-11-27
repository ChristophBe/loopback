package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
)

type userIDKey struct{}

// UserID returns the id of the authenticated user that Middleware stored in
// the request context.
func UserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey{}).(int)
	return id, ok
}

// Middleware enforces the bearerAuth scheme on every operation of the spec
// that requires security, which is derived from the spec: an operation's own
// `security` wins over the top-level one, and `security: []` leaves it open.
// Routes that the spec does not list pass through unchanged. basePath is the
// prefix the routes are registered under.
func (s *Service) Middleware(spec *openapi3.T, basePath string) echo.MiddlewareFunc {
	secured := securedRoutes(spec, basePath)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if !secured[req.Method+" "+c.Path()] {
				return next(c)
			}
			token, ok := bearerToken(req.Header.Get(echo.HeaderAuthorization))
			if !ok {
				return unauthorized(c, "missing bearer token")
			}
			id, err := s.ParseAccessToken(token)
			if err != nil {
				return unauthorized(c, "invalid or expired access token")
			}
			c.SetRequest(req.WithContext(context.WithValue(req.Context(), userIDKey{}, id)))
			return next(c)
		}
	}
}

// securedRoutes returns "METHOD /echo/path/:param" for each secured operation.
func securedRoutes(spec *openapi3.T, basePath string) map[string]bool {
	secured := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		route := basePath + strings.NewReplacer("{", ":", "}", "").Replace(path)
		for method, op := range item.Operations() {
			requirements := spec.Security
			if op.Security != nil {
				requirements = *op.Security
			}
			if len(requirements) > 0 {
				secured[method+" "+route] = true
			}
		}
	}
	return secured
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(c echo.Context, description string) error {
	c.Response().Header().Set(echo.HeaderWWWAuthenticate, "Bearer")
	return c.JSON(http.StatusUnauthorized, gen.Error{Error: gen.Unauthorized, ErrorDescription: &description})
}
