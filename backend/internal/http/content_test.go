package http_test

import (
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/gen"
	"github.com/ChristophBe/loopback/backend/internal/auth"
	"github.com/ChristophBe/loopback/backend/internal/gamification"
	"github.com/ChristophBe/loopback/backend/internal/http"
)

type contentFixture struct {
	api      nethttp.Handler
	token    string
	course   *ent.Course
	other    *ent.Course
	lessons  []*ent.Lesson
	quiz     *ent.Exercise
	flash    *ent.Exercise
	otherEx  *ent.Exercise
	sparseID int
}

func newContentFixture(t *testing.T) *contentFixture {
	t.Helper()
	url := testDatabaseURL(t)
	db := enttest.Open(t, "postgres", url)
	t.Cleanup(func() { _ = db.Close() })

	svc, err := auth.New(db, auth.Config{Secret: []byte(strings.Repeat("s", 32)), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	e, err := http.New("test", gamification.Rules{}, log, svc, db)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.IssueAccessToken(1)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	project := fmt.Sprintf("p%d", time.Now().UnixNano())
	f := &contentFixture{api: e, token: token}
	f.course = db.Course.Create().SetLanguage("go").SetProject(project).SetTitle("Course").SaveX(ctx)
	f.other = db.Course.Create().SetLanguage("rust").SetProject(project).SetTitle("Other").SaveX(ctx)
	// Created out of order to prove the response is sorted by order.
	second := db.Lesson.Create().SetCourse(f.course).SetSlug("b").SetOrder(2).SetTitle("Second").SetMinutes(5).SetXp(50).SaveX(ctx)
	first := db.Lesson.Create().SetCourse(f.course).SetSlug("a").SetOrder(1).SetTitle("First").SetMinutes(5).SetXp(50).
		SetBody("# Objective").
		SetOwnedFiles([]string{"resp/parse.go"}).
		SetStarterFiles(map[string]string{"resp/parse.go": "package resp"}).
		SetTestFiles(map[string]string{"resp/parse_test.go": "package resp_test"}).
		SetHints([]string{"nudge", "solution"}).
		SetFailureExplanations(map[string]string{"TestParse": "check the prefix"}).
		SaveX(ctx)
	f.lessons = []*ent.Lesson{first, second}
	f.sparseID = second.ID
	f.quiz = db.Exercise.Create().SetCourse(f.course).SetSlug("q").SetType("quiz").SetTitle("Quiz").SetPayload(map[string]any{"questions": []any{}}).SaveX(ctx)
	f.flash = db.Exercise.Create().SetCourse(f.course).SetSlug("f").SetType("flashcard").SetTitle("Cards").SetPayload(map[string]any{"cards": []any{}}).SaveX(ctx)
	f.otherEx = db.Exercise.Create().SetCourse(f.other).SetSlug("q").SetType("quiz").SetTitle("Other quiz").SetPayload(map[string]any{}).SaveX(ctx)
	return f
}

func (f *contentFixture) get(path string, authed bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1"+path, nil)
	if authed {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) T {
	t.Helper()
	var v T
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestListCourses(t *testing.T) {
	f := newContentFixture(t)
	got := decode[[]gen.Course](t, f.get("/courses", true), nethttp.StatusOK)
	found := map[int]gen.Course{}
	for _, c := range got {
		found[c.Id] = c
	}
	c, ok := found[f.course.ID]
	if !ok || c.Language != gen.Language("go") || c.Project != f.course.Project || c.Title != "Course" {
		t.Errorf("course = %+v, found %v", c, ok)
	}
	if _, ok := found[f.other.ID]; !ok {
		t.Error("second course is missing")
	}
}

func TestListCourseLessons(t *testing.T) {
	f := newContentFixture(t)
	got := decode[[]gen.Lesson](t, f.get(fmt.Sprintf("/courses/%d/lessons", f.course.ID), true), nethttp.StatusOK)
	if len(got) != 2 || got[0].Slug != "a" || got[1].Slug != "b" {
		t.Fatalf("lessons = %+v, want a then b", got)
	}
	l := got[0]
	if l.CourseId != f.course.ID || l.Title != "First" || l.Body != "# Objective" || l.Minutes != 5 || l.Xp != 50 {
		t.Errorf("lesson = %+v", l)
	}
	if l.StarterFiles["resp/parse.go"] != "package resp" || l.TestFiles["resp/parse_test.go"] != "package resp_test" ||
		len(l.OwnedFiles) != 1 || len(l.Hints) != 2 || l.FailureExplanations["TestParse"] != "check the prefix" {
		t.Errorf("content fields = %+v", l)
	}
}

func TestListCourseLessonsEmptyCourse(t *testing.T) {
	f := newContentFixture(t)
	got := decode[[]gen.Lesson](t, f.get(fmt.Sprintf("/courses/%d/lessons", f.other.ID), true), nethttp.StatusOK)
	if got == nil || len(got) != 0 {
		t.Errorf("lessons = %#v, want an empty list", got)
	}
}

func TestGetLesson(t *testing.T) {
	f := newContentFixture(t)
	l := decode[gen.Lesson](t, f.get(fmt.Sprintf("/lessons/%d", f.lessons[0].ID), true), nethttp.StatusOK)
	if l.Id != f.lessons[0].ID || l.CourseId != f.course.ID || l.StarterFiles["resp/parse.go"] != "package resp" || len(l.Hints) != 2 {
		t.Errorf("lesson = %+v", l)
	}
}

// A lesson without optional content still has every field in the response.
func TestGetLessonSendsEmptyValuesForMissingContent(t *testing.T) {
	f := newContentFixture(t)
	rec := f.get(fmt.Sprintf("/lessons/%d", f.sparseID), true)
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || rec.Code != nethttp.StatusOK {
		t.Fatalf("%d %s %v", rec.Code, rec.Body.String(), err)
	}
	for _, k := range []string{"ownedFiles", "hints"} {
		if v, ok := raw[k].([]any); !ok || len(v) != 0 {
			t.Errorf("%s = %#v, want []", k, raw[k])
		}
	}
	for _, k := range []string{"starterFiles", "testFiles", "failureExplanations"} {
		if v, ok := raw[k].(map[string]any); !ok || len(v) != 0 {
			t.Errorf("%s = %#v, want {}", k, raw[k])
		}
	}
}

func TestListExercises(t *testing.T) {
	f := newContentFixture(t)
	byCourse := decode[[]gen.Exercise](t, f.get(fmt.Sprintf("/exercises?courseId=%d", f.course.ID), true), nethttp.StatusOK)
	if len(byCourse) != 2 || byCourse[0].Id != f.quiz.ID || byCourse[1].Id != f.flash.ID {
		t.Fatalf("by course = %+v", byCourse)
	}
	if q := byCourse[0]; q.Type != gen.ExerciseType("quiz") || q.CourseId != f.course.ID || q.Slug != "q" || q.Title != "Quiz" || q.Payload["questions"] == nil {
		t.Errorf("quiz = %+v", q)
	}

	byType := decode[[]gen.Exercise](t, f.get(fmt.Sprintf("/exercises?courseId=%d&type=flashcard", f.course.ID), true), nethttp.StatusOK)
	if len(byType) != 1 || byType[0].Id != f.flash.ID {
		t.Errorf("by type = %+v", byType)
	}

	all := decode[[]gen.Exercise](t, f.get("/exercises", true), nethttp.StatusOK)
	ids := map[int]bool{}
	for _, e := range all {
		ids[e.Id] = true
	}
	if !ids[f.quiz.ID] || !ids[f.flash.ID] || !ids[f.otherEx.ID] {
		t.Errorf("unfiltered list misses seeded exercises: %v", ids)
	}
}

func TestContentNotFoundAndInvalid(t *testing.T) {
	f := newContentFixture(t)
	for _, tt := range []struct {
		path string
		code int
		err  gen.ErrorError
	}{
		{"/lessons/2147483647", nethttp.StatusNotFound, gen.NotFound},
		{"/courses/2147483647/lessons", nethttp.StatusNotFound, gen.NotFound},
		{"/lessons/abc", nethttp.StatusBadRequest, ""},
		{"/exercises?type=poem", nethttp.StatusBadRequest, ""},
	} {
		t.Run(tt.path, func(t *testing.T) {
			rec := f.get(tt.path, true)
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.code, rec.Body.String())
			}
			if tt.err != "" {
				if got := decode[gen.Error](t, rec, tt.code); got.Error != tt.err {
					t.Errorf("error = %q, want %q", got.Error, tt.err)
				}
			}
		})
	}
}

func TestContentRequiresAuth(t *testing.T) {
	f := newContentFixture(t)
	for _, path := range []string{"/courses", "/courses/1/lessons", "/lessons/1", "/exercises"} {
		if rec := f.get(path, false); rec.Code != nethttp.StatusUnauthorized {
			t.Errorf("GET %s without token = %d, want 401", path, rec.Code)
		}
	}
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}
