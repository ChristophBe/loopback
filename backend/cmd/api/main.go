// Command api serves the loopback REST API.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/internal/gamification"
	lbhttp "github.com/ChristophBe/loopback/backend/internal/http"
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

	e := lbhttp.New(version, rules, log)
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

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
