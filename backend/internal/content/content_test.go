package content_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/contentversion"
	"github.com/ChristophBe/loopback/backend/ent/enttest"
	"github.com/ChristophBe/loopback/backend/internal/content"
)

const sampleDir = "../../../content"

const quizJSON = `{"id":"go-slices-quiz","type":"quiz","language":"go","project":"redis-server","title":"Slices",
"questions":[{"prompt":"p","choices":["a","b"],"answer":1,"explanation":"e"}]}`

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	err := os.WriteFile(p, []byte(text), 0o600) //nolint:gosec // test helper writes into t.TempDir
	if err != nil {
		t.Fatal(err)
	}
}

// sampleCopy makes a temp copy of the real sample lesson with its starter and
// test files.
func sampleCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	lessonDir := filepath.Join(sampleDir, "go/redis-server")
	for _, rel := range []string{"01-parse-simple-string.md", "starter/01-parse-simple-string/resp/parse.go", "tests/01-parse-simple-string/resp/parse_test.go"} {
		data, err := os.ReadFile(filepath.Join(lessonDir, rel)) //nolint:gosec // test path
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, "go/redis-server/"+rel, string(data))
	}
	return root
}

func TestLoadSampleContent(t *testing.T) {
	b, err := content.Load(sampleDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Lessons) != 1 || b.Lessons[0].Slug != "go-redis-01-parse-simple-string" || b.Lessons[0].XP != 50 {
		t.Errorf("lessons = %+v", b.Lessons)
	}
	if !strings.HasPrefix(b.Lessons[0].Body, "## Objective") {
		t.Errorf("body = %q", b.Lessons[0].Body)
	}
	l := b.Lessons[0]
	if !strings.Contains(l.StarterFiles["resp/parse.go"], "func Parse") || len(l.StarterFiles) != 1 {
		t.Errorf("starter files = %v", l.StarterFiles)
	}
	if !strings.Contains(l.TestFiles["resp/parse_test.go"], "TestParseSimpleStringMissingPrefix") || len(l.TestFiles) != 1 {
		t.Errorf("test files = %v", l.TestFiles)
	}
	if got := b.Lessons[0].Course.Title(); got != "Redis Server" {
		t.Errorf("title = %q", got)
	}
}

func TestLoadSkipsTemplates(t *testing.T) {
	root := sampleCopy(t)
	for _, d := range []string{"solutions", "starter", "tests"} {
		write(t, root, "go/redis-server/"+d+"/02-other/notes.md", "not a lesson")
		write(t, root, "go/redis-server/"+d+"/02-other/data.json", "not json")
	}
	write(t, root, "template-go/bad.md", "not a lesson")
	if _, err := content.Load(root); err != nil {
		t.Fatal(err)
	}
}

func TestLoadExercise(t *testing.T) {
	root := sampleCopy(t)
	write(t, root, "go/redis-server/02-slices.json", quizJSON)
	b, err := content.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Exercises) != 1 || b.Exercises[0].Type != "quiz" || b.Exercises[0].Payload["questions"] == nil {
		t.Errorf("exercises = %+v", b.Exercises)
	}
}

func TestLoadInvalid(t *testing.T) {
	lesson, err := os.ReadFile(filepath.Join(sampleDir, "go/redis-server/01-parse-simple-string.md"))
	if err != nil {
		t.Fatal(err)
	}
	sample := string(lesson)
	cases := []struct {
		name, rel, text, file, field string
	}{
		{"no front matter", "go/redis-server/a.md", "# hi", "go/redis-server/a.md", "(front matter)"},
		{"minutes too high", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "minutes: 5", "minutes: 99", 1), "go/redis-server/01-parse-simple-string.md", "minutes"},
		{"missing field", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "xp: 50\n", "", 1), "go/redis-server/01-parse-simple-string.md", "xp"},
		{"unknown field", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "xp: 50\n", "xp: 50\nfoo: 1\n", 1), "go/redis-server/01-parse-simple-string.md", "foo"},
		{"wrong type", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "order: 1", "order: first", 1), "go/redis-server/01-parse-simple-string.md", "order"},
		{"missing heading", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "## Instructions", "## Steps", 1), "go/redis-server/01-parse-simple-string.md", "body"},
		{"headings in wrong order", "go/redis-server/01-parse-simple-string.md", strings.NewReplacer("## Objective", "## TMP", "## Instructions", "## Objective", "## TMP", "## Instructions").Replace(sample), "go/redis-server/01-parse-simple-string.md", "body"},
		{"language mismatch", "rust/redis-server/01-parse-simple-string.md", sample, "rust/redis-server/01-parse-simple-string.md", "language"},
		{"bad reference solution", "go/redis-server/01-parse-simple-string.md", strings.Replace(sample, "referenceSolution: solutions/01-parse-simple-string", "referenceSolution: ../x", 1), "go/redis-server/01-parse-simple-string.md", "referenceSolution"},
		{"bad depth", "x.json", quizJSON, "x.json", "path"},
		{"bad json", "go/redis-server/q.json", "{", "go/redis-server/q.json", "(root)"},
		{"unknown exercise type", "go/redis-server/q.json", `{"type":"poll"}`, "go/redis-server/q.json", "type"},
		{"quiz answer out of range", "go/redis-server/q.json", strings.Replace(quizJSON, `"answer":1`, `"answer":5`, 1), "go/redis-server/q.json", "questions[0].answer"},
		{"quiz missing answer", "go/redis-server/q.json", strings.Replace(quizJSON, `"answer":1,`, ``, 1), "go/redis-server/q.json", "questions[0].answer"},
		{"quiz no questions", "go/redis-server/q.json", strings.Replace(quizJSON, `[{"prompt":"p","choices":["a","b"],"answer":1,"explanation":"e"}]`, `[]`, 1), "go/redis-server/q.json", "questions"},
		{"flashcard empty side", "go/redis-server/c.json", `{"id":"c","type":"flashcard","language":"go","project":"redis-server","title":"t","cards":[{"front":"f","back":""}]}`, "go/redis-server/c.json", "cards[0].back"},
		{"review bad language", "go/redis-server/r.json", `{"id":"r","type":"review","language":"go","project":"redis-server","title":"t","code":{"language":"c","source":"x"},"flaggedLines":[{"line":1,"note":"n"}]}`, "go/redis-server/r.json", "code.language"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tc.rel, tc.text)
			_, err := content.Load(root)
			ps, ok := err.(content.Problems)
			if !ok {
				t.Fatalf("err = %v, want Problems", err)
			}
			for _, p := range ps {
				if p.File == tc.file && p.Field == tc.field {
					return
				}
			}
			t.Errorf("want a problem for %s %s, got %v", tc.file, tc.field, ps)
		})
	}
}

func TestLoadLessonFiles(t *testing.T) {
	const lesson = "go/redis-server/01-parse-simple-string.md"
	const starter = "go/redis-server/starter/01-parse-simple-string/"
	const tests = "go/redis-server/tests/01-parse-simple-string/"
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		field string
		want  string // must appear in the message, with the path
	}{
		{"test file missing", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, tests, "resp/parse_test.go")); err != nil {
				t.Fatal(err)
			}
		}, "ownedFiles", `"resp/parse_test.go"`},
		{"starter file missing", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, starter, "resp/parse.go")); err != nil {
				t.Fatal(err)
			}
		}, "ownedFiles", `"resp/parse.go"`},
		{"extra starter file", func(t *testing.T, root string) { write(t, root, starter+"resp/extra.go", "package resp") }, "starterFiles", `"resp/extra.go"`},
		{"extra test file", func(t *testing.T, root string) { write(t, root, tests+"resp/extra_test.go", "package resp") }, "testFiles", `"resp/extra_test.go"`},
		{"starter directory missing", func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, starter)); err != nil {
				t.Fatal(err)
			}
		}, "starterFiles", "starter/01-parse-simple-string"},
		{"symlink", func(t *testing.T, root string) {
			if err := os.Symlink("parse.go", filepath.Join(root, starter, "resp/link.go")); err != nil {
				t.Skip(err)
			}
		}, "starterFiles", `"resp/link.go"`},
		{"only test directory declared", func(t *testing.T, root string) {
			data, err := os.ReadFile(filepath.Join(root, lesson)) //nolint:gosec // test path
			if err != nil {
				t.Fatal(err)
			}
			write(t, root, lesson, strings.Replace(string(data), "starterFiles: starter/01-parse-simple-string\n", "", 1))
		}, "ownedFiles", "testFiles directory"},
		{"file is not text", func(t *testing.T, root string) { write(t, root, starter+"resp/parse.go", "a\x00b") }, "starterFiles", `"resp/parse.go"`},
		{"escaping directory", func(t *testing.T, root string) {
			data, err := os.ReadFile(filepath.Join(root, lesson)) //nolint:gosec // test path
			if err != nil {
				t.Fatal(err)
			}
			write(t, root, lesson, strings.Replace(string(data), "testFiles: tests/01-parse-simple-string", "testFiles: ../tests", 1))
		}, "testFiles", "relative directory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := sampleCopy(t)
			tc.setup(t, root)
			_, err := content.Load(root)
			ps, ok := err.(content.Problems)
			if !ok {
				t.Fatalf("err = %v, want Problems", err)
			}
			for _, p := range ps {
				if p.File == lesson && p.Field == tc.field && strings.Contains(p.Message, tc.want) {
					return
				}
			}
			t.Errorf("want a problem for %s %s mentioning %s, got %v", lesson, tc.field, tc.want, ps)
		})
	}
}

func TestLoadLessonWithoutFileDirectories(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join(sampleDir, "go/redis-server/01-parse-simple-string.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.NewReplacer("starterFiles: starter/01-parse-simple-string\n", "", "testFiles: tests/01-parse-simple-string\n", "").Replace(string(data))
	write(t, root, "go/redis-server/01-parse-simple-string.md", text)
	b := load(t, root)
	if b.Lessons[0].StarterFiles != nil || b.Lessons[0].TestFiles != nil {
		t.Errorf("lesson = %+v", b.Lessons[0])
	}
}

func TestLoadDuplicates(t *testing.T) {
	root := sampleCopy(t)
	data, _ := os.ReadFile(filepath.Join(root, "go/redis-server/01-parse-simple-string.md")) //nolint:gosec // test path
	write(t, root, "go/redis-server/02-copy.md", string(data))
	_, err := content.Load(root)
	ps, ok := err.(content.Problems)
	if !ok || len(ps) != 2 {
		t.Fatalf("err = %v, want 2 duplicate problems (id and order)", err)
	}
}

func TestRequiredSectionsMatchSchema(t *testing.T) {
	data, err := os.ReadFile("../../../api/schemas/lesson.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Definitions struct {
			Sections struct {
				Const []string `json:"const"`
			} `json:"requiredBodySections"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var body strings.Builder
	for _, h := range s.Definitions.Sections.Const {
		body.WriteString("## " + h + "\n\ntext\n\n")
	}
	lesson, _ := os.ReadFile(filepath.Join(sampleDir, "go/redis-server/01-parse-simple-string.md"))
	front, _, _ := strings.Cut(string(lesson), "\n---\n")
	front = strings.NewReplacer("starterFiles: starter/01-parse-simple-string\n", "", "testFiles: tests/01-parse-simple-string\n", "").Replace(front)
	write(t, root, "go/redis-server/01-a.md", front+"\n---\n"+body.String())
	if _, err := content.Load(root); err != nil {
		t.Errorf("sections from the schema are rejected: %v", err)
	}
}

func TestGitSHAFallsBackToEnv(t *testing.T) {
	t.Setenv("CONTENT_SHA", "abc123")
	sha, err := content.GitSHA(context.Background(), t.TempDir())
	if err != nil || sha != "abc123" {
		t.Errorf("sha = %q, err = %v", sha, err)
	}
	t.Setenv("CONTENT_SHA", "")
	if _, err := content.GitSHA(context.Background(), t.TempDir()); err == nil {
		t.Error("want an error without git and CONTENT_SHA")
	}
}

func TestGitSHAFromRepo(t *testing.T) {
	sha, err := content.GitSHA(context.Background(), sampleDir)
	if err != nil || len(sha) != 40 {
		t.Errorf("sha = %q, err = %v", sha, err)
	}
}

// testClient opens a client on a private schema so parallel packages do not
// share rows.
func testClient(t *testing.T) *ent.Client {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	schema := fmt.Sprintf("import_%s", strings.ToLower(strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())))
	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE SCHEMA IF NOT EXISTS ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = db.Close()
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	client := enttest.Open(t, "postgres", u.String())
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func load(t *testing.T, root string) *content.Bundle {
	t.Helper()
	b, err := content.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestImportIsIdempotent(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	root := sampleCopy(t)
	write(t, root, "go/redis-server/02-slices.json", quizJSON)
	b := load(t, root)

	res, err := content.Import(ctx, client, b, "sha-1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.VersionCreated || res.Created != 2 || res.Updated != 0 {
		t.Errorf("first run = %+v", res)
	}
	if n := client.ContentVersion.Query().Where(contentversion.Checksum("sha-1")).CountX(ctx); n != 1 {
		t.Errorf("versions = %d", n)
	}
	l := client.Lesson.Query().OnlyX(ctx)
	if l.Slug != "go-redis-01-parse-simple-string" || l.Xp != 50 || len(l.OwnedFiles) != 2 || l.FailureExplanations["TestParseSimpleStringMissingPrefix"] == "" {
		t.Errorf("lesson = %+v", l)
	}
	ex := client.Exercise.Query().OnlyX(ctx)
	if ex.Type != "quiz" || ex.Payload["questions"] == nil {
		t.Errorf("exercise = %+v", ex)
	}

	res, err = content.Import(ctx, client, load(t, root), "sha-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.VersionCreated || res.Created != 0 || res.Updated != 0 || res.Unchanged != 2 {
		t.Errorf("second run = %+v", res)
	}
	if n := client.ContentVersion.Query().CountX(ctx); n != 1 {
		t.Errorf("versions after rerun = %d", n)
	}
	// A new SHA with unchanged content records the version but touches no row.
	if _, err = content.Import(ctx, client, load(t, root), "sha-2"); err != nil {
		t.Fatal(err)
	}
	if v := client.Lesson.Query().OnlyX(ctx).QueryContentVersion().OnlyX(ctx).Checksum; v != "sha-1" {
		t.Errorf("lesson version = %q, want sha-1", v)
	}
}

func TestImportStoresStarterAndTestFiles(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	root := sampleCopy(t)
	if _, err := content.Import(ctx, client, load(t, root), "sha-1"); err != nil {
		t.Fatal(err)
	}
	l := client.Lesson.Query().OnlyX(ctx)
	if !strings.Contains(l.StarterFiles["resp/parse.go"], "func Parse") || len(l.StarterFiles) != 1 {
		t.Errorf("starter files = %v", l.StarterFiles)
	}
	if !strings.Contains(l.TestFiles["resp/parse_test.go"], "func TestParseSimpleString") || len(l.TestFiles) != 1 {
		t.Errorf("test files = %v", l.TestFiles)
	}
	// Changing a starter file updates the lesson; the same content again does not.
	write(t, root, "go/redis-server/starter/01-parse-simple-string/resp/parse.go", "package resp\n")
	res, err := content.Import(ctx, client, load(t, root), "sha-2")
	if err != nil || res.Updated != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := client.Lesson.Query().OnlyX(ctx).StarterFiles["resp/parse.go"]; got != "package resp\n" {
		t.Errorf("starter file = %q", got)
	}
	if res, err = content.Import(ctx, client, load(t, root), "sha-3"); err != nil || res.Unchanged != 1 {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestImportUpdatesChangedContent(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	root := sampleCopy(t)
	if _, err := content.Import(ctx, client, load(t, root), "sha-1"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "go/redis-server/01-parse-simple-string.md")
	data, _ := os.ReadFile(p) //nolint:gosec // test path
	write(t, root, "go/redis-server/01-parse-simple-string.md", strings.Replace(string(data), "xp: 50", "xp: 60", 1))
	res, err := content.Import(ctx, client, load(t, root), "sha-2")
	if err != nil {
		t.Fatal(err)
	}
	if !res.VersionCreated || res.Updated != 1 {
		t.Errorf("run = %+v", res)
	}
	l := client.Lesson.Query().OnlyX(ctx)
	if l.Xp != 60 || l.QueryContentVersion().OnlyX(ctx).Checksum != "sha-2" {
		t.Errorf("lesson = %+v", l)
	}
	if n := client.ContentVersion.Query().CountX(ctx); n != 2 {
		t.Errorf("versions = %d", n)
	}
}

func TestImportRollsBackOnFailure(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	b := load(t, sampleCopy(t))
	if _, err := content.Import(ctx, client, b, ""); err == nil {
		t.Error("want an error for an empty sha")
	}
	// A bundle whose second lesson violates the order index fails as a whole.
	dup := *b
	second := b.Lessons[0]
	second.Slug = "other"
	dup.Lessons = append(append([]content.Lesson{}, b.Lessons...), second)
	if _, err := content.Import(ctx, client, &dup, "sha-x"); err == nil {
		t.Fatal("want a constraint error")
	}
	if n := client.Lesson.Query().CountX(ctx) + client.ContentVersion.Query().CountX(ctx); n != 0 {
		t.Errorf("rows after rollback = %d", n)
	}
}
