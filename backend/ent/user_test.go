package ent_test

import (
	"context"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent/enttest"
)

func TestCreateUser(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	client := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = client.Close() })

	u, err := client.User.Create().SetEmail("learner@example.com").Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "learner@example.com" {
		t.Errorf("email = %q", u.Email)
	}
	if _, err := client.User.Create().SetEmail("learner@example.com").Save(context.Background()); err == nil {
		t.Error("want a unique violation for a duplicate email")
	}
}
