// Command import validates the lesson content and loads it into the database.
//
// Configuration, all through the environment:
//
//	DATABASE_URL  Postgres connection URL (required)
//	CONTENT_DIR   content directory, default "content"
//	CONTENT_SHA   git SHA of the content, used when `git rev-parse HEAD` fails
//	              in CONTENT_DIR (a container without git or without .git)
package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/signal"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/internal/content"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, log); err != nil {
		log.WithError(err).Error("import failed")
		stop()
		os.Exit(1) // #nosec G104 -- stop is called explicitly because os.Exit skips defers
	}
}

func run(ctx context.Context, log *logrus.Logger) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is not set")
	}
	dir := os.Getenv("CONTENT_DIR")
	if dir == "" {
		dir = "content"
	}

	bundle, err := content.Load(dir)
	if err != nil {
		return err
	}
	sha, err := content.GitSHA(ctx, dir)
	if err != nil {
		return err
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		return err
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()

	res, err := content.Import(ctx, client, bundle, sha)
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"sha": sha, "versionCreated": res.VersionCreated,
		"created": res.Created, "updated": res.Updated, "unchanged": res.Unchanged,
	}).Info("import finished")
	return nil
}
