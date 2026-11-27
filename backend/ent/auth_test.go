package ent_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/ent/identity"
	"github.com/ChristophBe/loopback/backend/ent/session"
	"github.com/ChristophBe/loopback/backend/ent/user"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

// uniq returns a value that differs per test run, because the test database
// keeps its rows between runs.
func uniq(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s-%d", prefix, t.Name(), time.Now().UnixNano())
}

func TestIdentity(t *testing.T) {
	client := enttest.Open(t, "postgres", testDatabaseURL(t))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	ghID := uniq(t, "gh")
	u := client.User.Create().SetEmail(uniq(t, "identity") + "@example.com").SaveX(ctx)
	gh := client.Identity.Create().SetProvider(identity.ProviderGithub).SetProviderUserID(ghID).SetUser(u).SaveX(ctx)
	client.Identity.Create().SetProvider(identity.ProviderGoogle).SetProviderUserID(ghID).SetUser(u).SaveX(ctx)

	got := client.Identity.Query().
		Where(identity.ProviderEQ(identity.ProviderGithub), identity.ProviderUserID(ghID)).
		WithUser().OnlyX(ctx)
	if got.ID != gh.ID || got.Edges.User.ID != u.ID {
		t.Errorf("identity = %+v", got)
	}
	if n := u.QueryIdentities().CountX(ctx); n != 2 {
		t.Errorf("user identities = %d", n)
	}

	other := client.User.Create().SetEmail(uniq(t, "other") + "@example.com").SaveX(ctx)
	if _, err := client.Identity.Create().SetProvider(identity.ProviderGithub).SetProviderUserID(ghID).SetUser(other).Save(ctx); !ent.IsConstraintError(err) {
		t.Errorf("duplicate (provider, provider user id): err = %v, want a constraint error", err)
	}
	if _, err := client.Identity.Create().SetProvider(identity.ProviderGithub).SetProviderUserID(ghID + "-2").Save(ctx); err == nil {
		t.Error("want an error for an identity without a user")
	}
}

func TestSession(t *testing.T) {
	client := enttest.Open(t, "postgres", testDatabaseURL(t))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	hash := uniq(t, "hash")
	u := client.User.Create().SetEmail(uniq(t, "session") + "@example.com").SaveX(ctx)
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	s := client.Session.Create().SetTokenHash(hash).SetExpiresAt(exp).SetDevice("jetbrains").SetUser(u).SaveX(ctx)
	if s.Revoked {
		t.Error("a new session must not be revoked")
	}

	got := client.Session.Query().Where(session.TokenHash(hash)).WithUser().OnlyX(ctx)
	if got.ID != s.ID || got.Edges.User.ID != u.ID || !got.ExpiresAt.Equal(exp) || got.Device == nil || *got.Device != "jetbrains" {
		t.Errorf("session = %+v", got)
	}

	client.Session.UpdateOne(s).SetRevoked(true).ExecX(ctx)
	if !client.Session.GetX(ctx, s.ID).Revoked {
		t.Error("revoked flag not stored")
	}

	if _, err := client.Session.Create().SetTokenHash(hash).SetExpiresAt(exp).SetUser(u).Save(ctx); !ent.IsConstraintError(err) {
		t.Errorf("duplicate token hash: err = %v, want a constraint error", err)
	}
	noDevice := client.Session.Create().SetTokenHash(hash + "-2").SetExpiresAt(exp).SetUser(u).SaveX(ctx)
	if noDevice.Device != nil {
		t.Error("device must be optional")
	}
	if n := u.QuerySessions().CountX(ctx); n != 2 {
		t.Errorf("user sessions = %d", n)
	}

	client.User.DeleteOne(u).ExecX(ctx)
	if n := client.Session.Query().Where(session.HasUserWith(user.ID(u.ID))).CountX(ctx); n != 0 {
		t.Errorf("sessions after deleting the user = %d, want 0", n)
	}
}

// scratchDatabase creates an empty database next to the test database and
// returns its connection and URL.
func scratchDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	base := testDatabaseURL(t)
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)")
		_ = admin.Close()
	})
	return db, u.String()
}

func migrationFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(files) < 2 {
		t.Fatalf("migrations = %v, err = %v", files, err)
	}
	sort.Strings(files)
	return files
}

func applyMigration(t *testing.T, db *sql.DB, file string) {
	t.Helper()
	b, err := os.ReadFile(file) //nolint:gosec // test reads the repository migrations
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("apply %s: %v", filepath.Base(file), err)
	}
}

func exerciseAuthTables(t *testing.T, dbURL string, userID int) {
	t.Helper()
	ctx := context.Background()
	client, err := ent.Open("postgres", dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	u := client.User.GetX(ctx, userID)
	client.Identity.Create().SetProvider(identity.ProviderEmail).SetProviderUserID(u.Email).SetUser(u).SaveX(ctx)
	client.Session.Create().SetTokenHash("h").SetExpiresAt(time.Now().Add(time.Hour)).SetUser(u).SaveX(ctx)
	if _, err := client.Identity.Create().SetProvider(identity.ProviderEmail).SetProviderUserID(u.Email).SetUser(u).Save(ctx); !ent.IsConstraintError(err) {
		t.Errorf("duplicate identity: err = %v", err)
	}
	client.User.DeleteOne(u).ExecX(ctx)
	if n := client.Session.Query().CountX(ctx) + client.Identity.Query().CountX(ctx); n != 0 {
		t.Errorf("rows left after deleting the user = %d", n)
	}
}

func TestMigrationsOnEmptyDatabase(t *testing.T) {
	db, dbURL := scratchDatabase(t)
	for _, f := range migrationFiles(t) {
		applyMigration(t, db, f)
	}
	var id int
	if err := db.QueryRow(`INSERT INTO users (email, created_at) VALUES ('a-' || now()::text || '@example.com', now()) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	exerciseAuthTables(t, dbURL, id)
}

func TestMigrationOnTopOfInit(t *testing.T) {
	db, dbURL := scratchDatabase(t)
	files := migrationFiles(t)
	if !strings.HasSuffix(files[0], "_init.sql") {
		t.Fatalf("first migration = %s", files[0])
	}
	applyMigration(t, db, files[0])
	var id int
	if err := db.QueryRow(`INSERT INTO users (email, created_at) VALUES ('old-' || now()::text || '@example.com', now()) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, f := range files[1:] {
		applyMigration(t, db, f)
	}
	exerciseAuthTables(t, dbURL, id)
}
