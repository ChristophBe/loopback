// Package http wires the Echo server to the generated API interface.
package http

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
)

// BasePath is the prefix of every API route.
const BasePath = "/api/v1"

// Server implements gen.StrictServerInterface.
type Server struct {
	version string
	rules   gamification.Rules
	auth    *auth.Service
	db      *ent.Client
}

var _ gen.StrictServerInterface = (*Server)(nil)

// New returns the Echo router with all API routes registered.
func New(version string, rules gamification.Rules, log *logrus.Logger, authSvc *auth.Service, db *ent.Client, opts ...Option) (*echo.Echo, error) {
	if authSvc == nil {
		return nil, errors.New("auth service is required")
	}
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded spec: %w", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.RequestID())
	e.Use(requestLogger(log))
	e.Use(middleware.Recover())
	e.Use(noStore(BasePath + "/auth/"))
	e.Use(authSvc.Middleware(spec, BasePath))

	s := &Server{version: version, rules: rules, auth: authSvc, db: db}
	gen.RegisterHandlersWithBaseURL(e, gen.NewStrictHandler(s, nil), BasePath)
	for _, opt := range opts {
		opt(s, e)
	}
	return e, nil
}

// GetHealth reports that the API is up.
func (s *Server) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return gen.GetHealth200JSONResponse{Status: gen.Ok}, nil
}

// GetConfig returns the API version and the gamification rules.
func (s *Server) GetConfig(context.Context, gen.GetConfigRequestObject) (gen.GetConfigResponseObject, error) {
	return gen.GetConfig200JSONResponse{
		Version: s.version,
		Gamification: gen.GamificationRules{
			XpPerLesson:      s.rules.XpPerLesson,
			XpPerLevel:       s.rules.XpPerLevel,
			DailyGoalLessons: s.rules.DailyGoalLessons,
		},
	}, nil
}

// noStore keeps token responses out of caches (RFC 6749 section 5.1).
func noStore(prefix string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.HasPrefix(c.Request().URL.Path, prefix) {
				c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
				c.Response().Header().Set("Pragma", "no-cache")
			}
			return next(c)
		}
	}
}

func requestLogger(log *logrus.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogRequestID: true,
		LogLatency:   true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			log.WithFields(logrus.Fields{
				"request_id": v.RequestID,
				"method":     v.Method,
				"uri":        v.URI,
				"status":     v.Status,
				"latency":    v.Latency.String(),
			}).Info("request")
			return nil
		},
	})
}
