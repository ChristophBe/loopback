// Command api serves the loopback REST API.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	lbhttp "github.com/ChristophBe/loopback/backend/internal/http"
	"github.com/ChristophBe/loopback/backend/internal/mail"
)

// version is set at build time with -ldflags.
var version = "dev"

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	rules, err := gamification.Load(env("GAMIFICATION_FILE", "../api/gamification.yaml"))
	if err != nil {
		log.WithError(err).Fatal("load gamification rules")
	}

	authCfg, err := auth.LoadConfig(os.Getenv)
	if err != nil {
		log.WithError(err).Fatal("load auth config")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	db, err := ent.Open("postgres", dbURL)
	if err != nil {
		log.WithError(err).Fatal("open database")
	}
	defer func() { _ = db.Close() }()
	authSvc, err := auth.New(db, authCfg)
	if err != nil {
		log.WithError(err).Fatal("create auth service")
	}

	magicLinks, err := newMagicLinks(authSvc)
	if err != nil {
		log.WithError(err).Fatal("create magic-link sign-in")
	}

	opts := []lbhttp.Option{lbhttp.WithMagicLinks(magicLinks)}
	if os.Getenv("AUTH_DEV_LOGIN") == "true" {
		log.Warn("AUTH_DEV_LOGIN is on: POST /auth/dev-login signs in without credentials")
		opts = append(opts, lbhttp.WithDevLogin())
	}
	e, err := lbhttp.New(version, rules, log, authSvc, db, opts...)
	if err != nil {
		log.WithError(err).Fatal("create server")
	}
	addr := env("API_ADDR", ":8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			log.WithError(err).Error("shutdown")
		}
	}()

	log.WithField("addr", addr).Info("listening")
	if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.WithError(err).Fatal("serve")
	}
}

// newMagicLinks returns nil, and the endpoints answer 501, when SMTP_HOST is
// not set.
func newMagicLinks(svc *auth.Service) (*auth.MagicLinks, error) {
	if os.Getenv("SMTP_HOST") == "" {
		return nil, nil
	}
	smtpCfg, err := mail.LoadSMTPConfig(os.Getenv)
	if err != nil {
		return nil, err
	}
	cfg, err := auth.LoadMagicLinkConfig(os.Getenv)
	if err != nil {
		return nil, err
	}
	return auth.NewMagicLinks(svc, mail.NewSMTP(smtpCfg), cfg)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
