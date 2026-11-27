package ent_test

import (
	"context"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/course"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/ent/exercise"
)

func TestContentEntities(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	client := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	cv := client.ContentVersion.Create().SetChecksum("sum-" + t.Name()).SaveX(ctx)
	c := client.Course.Create().SetLanguage(course.LanguageGo).SetProject("kv-store").SetTitle("KV store").SaveX(ctx)
	l := client.Lesson.Create().
		SetSlug("01-hello").SetOrder(1).SetTitle("Hello").SetMinutes(5).SetXp(10).
		SetBody("## Objective").SetOwnedFiles([]string{"main.go"}).SetHints([]string{"read the test"}).
		SetStarterFiles(map[string]string{"main.go": "package main\n"}).SetTestFiles(map[string]string{"main_test.go": "package main\n"}).
		SetCourse(c).SetContentVersion(cv).SaveX(ctx)
	e := client.Exercise.Create().
		SetSlug("quiz-maps").SetType(exercise.TypeQuiz).SetTitle("Maps").
		SetPayload(map[string]any{"questions": []any{}}).SetCourse(c).SetContentVersion(cv).SaveX(ctx)

	got := client.Course.Query().Where(course.Project("kv-store")).WithLessons().WithExercises().OnlyX(ctx)
	if len(got.Edges.Lessons) != 1 || got.Edges.Lessons[0].ID != l.ID {
		t.Errorf("lessons = %v", got.Edges.Lessons)
	}
	if len(got.Edges.Exercises) != 1 || got.Edges.Exercises[0].ID != e.ID {
		t.Errorf("exercises = %v", got.Edges.Exercises)
	}
	if gl := client.Lesson.GetX(ctx, l.ID); gl.Slug != "01-hello" || gl.OwnedFiles[0] != "main.go" || gl.Hints[0] != "read the test" ||
		gl.StarterFiles["main.go"] != "package main\n" || gl.TestFiles["main_test.go"] != "package main\n" {
		t.Errorf("lesson = %+v", gl)
	}
	if ge := client.Exercise.GetX(ctx, e.ID); ge.Type != exercise.TypeQuiz || ge.Payload["questions"] == nil {
		t.Errorf("exercise = %+v", ge)
	}
	if n := cv.QueryLessons().CountX(ctx); n != 1 {
		t.Errorf("version lessons = %d", n)
	}
	if n := cv.QueryExercises().CountX(ctx); n != 1 {
		t.Errorf("version exercises = %d", n)
	}
	if _, err := client.Lesson.Create().SetSlug("01-hello").SetOrder(2).SetTitle("x").SetMinutes(1).SetXp(1).SetCourse(c).Save(ctx); !ent.IsConstraintError(err) {
		t.Errorf("duplicate slug: err = %v", err)
	}
}
