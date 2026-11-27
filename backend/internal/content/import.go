package content

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/contentversion"
	entcourse "github.com/ChristophBe/loopback/backend/ent/course"
	"github.com/ChristophBe/loopback/backend/ent/exercise"
	"github.com/ChristophBe/loopback/backend/ent/lesson"
)

// Result counts what an import did.
type Result struct {
	// VersionCreated is true when the git SHA was not yet recorded.
	VersionCreated bool
	Created        int
	Updated        int
	Unchanged      int
}

// GitSHA returns the git commit of the content: `git rev-parse HEAD` in dir,
// or the CONTENT_SHA environment variable when git cannot answer (a container
// without git or without the .git directory).
func GitSHA(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output() // #nosec G204 -- fixed command, dir is an argument
	if sha := strings.TrimSpace(string(out)); err == nil && sha != "" {
		return sha, nil
	}
	if sha := strings.TrimSpace(os.Getenv("CONTENT_SHA")); sha != "" {
		return sha, nil
	}
	return "", errors.New("cannot determine the content git SHA: git rev-parse HEAD failed and CONTENT_SHA is not set")
}

// Import upserts the bundle in one transaction and records sha as a
// ContentVersion. A lesson or exercise that already equals the bundle is not
// written, so importing unchanged content again changes nothing. Rows that
// are no longer in the bundle are left alone.
func Import(ctx context.Context, client *ent.Client, b *Bundle, sha string) (res Result, err error) {
	if sha == "" {
		return res, errors.New("content version checksum must not be empty")
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return res, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
			return
		}
		err = tx.Commit()
	}()

	cv, err := tx.ContentVersion.Query().Where(contentversion.Checksum(sha)).Only(ctx)
	switch {
	case ent.IsNotFound(err):
		if cv, err = tx.ContentVersion.Create().SetChecksum(sha).Save(ctx); err != nil {
			return res, fmt.Errorf("record content version: %w", err)
		}
		res.VersionCreated = true
	case err != nil:
		return res, err
	}

	courses := map[Course]*ent.Course{}
	getCourse := func(c Course) (*ent.Course, error) {
		if ec, ok := courses[c]; ok {
			return ec, nil
		}
		ec, err := upsertCourse(ctx, tx, c)
		courses[c] = ec
		return ec, err
	}
	for _, l := range b.Lessons {
		ec, err := getCourse(l.Course)
		if err != nil {
			return res, err
		}
		if err := upsertLesson(ctx, tx, ec, cv, l, &res); err != nil {
			return res, fmt.Errorf("%s: %w", l.File, err)
		}
	}
	for _, e := range b.Exercises {
		ec, err := getCourse(e.Course)
		if err != nil {
			return res, err
		}
		if err := upsertExercise(ctx, tx, ec, cv, e, &res); err != nil {
			return res, fmt.Errorf("%s: %w", e.File, err)
		}
	}
	return res, nil
}

func upsertCourse(ctx context.Context, tx *ent.Tx, c Course) (*ent.Course, error) {
	ec, err := tx.Course.Query().Where(entcourse.LanguageEQ(entcourse.Language(c.Language)), entcourse.Project(c.Project)).Only(ctx)
	switch {
	case ent.IsNotFound(err):
		return tx.Course.Create().SetLanguage(entcourse.Language(c.Language)).SetProject(c.Project).SetTitle(c.Title()).Save(ctx)
	case err != nil:
		return nil, err
	case ec.Title != c.Title():
		return ec.Update().SetTitle(c.Title()).Save(ctx)
	}
	return ec, nil
}

func upsertLesson(ctx context.Context, tx *ent.Tx, c *ent.Course, cv *ent.ContentVersion, l Lesson, res *Result) error {
	cur, err := tx.Lesson.Query().Where(lesson.Slug(l.Slug), lesson.HasCourseWith(entcourse.ID(c.ID))).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = tx.Lesson.Create().
			SetSlug(l.Slug).SetOrder(l.Order).SetTitle(l.Title).SetMinutes(l.Minutes).SetXp(l.XP).SetBody(l.Body).
			SetOwnedFiles(l.OwnedFiles).SetStarterFiles(l.StarterFiles).SetTestFiles(l.TestFiles).SetHints(l.Hints).SetFailureExplanations(l.FailureExplanations).
			SetCourse(c).SetContentVersion(cv).Save(ctx)
		res.Created++
		return err
	}
	if err != nil {
		return err
	}
	if cur.Order == l.Order && cur.Title == l.Title && cur.Minutes == l.Minutes && cur.Xp == l.XP && cur.Body == l.Body &&
		same(cur.OwnedFiles, l.OwnedFiles) && same(cur.StarterFiles, l.StarterFiles) && same(cur.TestFiles, l.TestFiles) && same(cur.Hints, l.Hints) && same(cur.FailureExplanations, l.FailureExplanations) {
		res.Unchanged++
		return nil
	}
	res.Updated++
	return cur.Update().
		SetOrder(l.Order).SetTitle(l.Title).SetMinutes(l.Minutes).SetXp(l.XP).SetBody(l.Body).
		SetOwnedFiles(l.OwnedFiles).SetStarterFiles(l.StarterFiles).SetTestFiles(l.TestFiles).SetHints(l.Hints).SetFailureExplanations(l.FailureExplanations).
		SetContentVersion(cv).Exec(ctx)
}

func upsertExercise(ctx context.Context, tx *ent.Tx, c *ent.Course, cv *ent.ContentVersion, e Exercise, res *Result) error {
	cur, err := tx.Exercise.Query().Where(exercise.Slug(e.Slug), exercise.HasCourseWith(entcourse.ID(c.ID))).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = tx.Exercise.Create().
			SetSlug(e.Slug).SetType(exercise.Type(e.Type)).SetTitle(e.Title).SetPayload(e.Payload).
			SetCourse(c).SetContentVersion(cv).Save(ctx)
		res.Created++
		return err
	}
	if err != nil {
		return err
	}
	if string(cur.Type) == e.Type && cur.Title == e.Title && same(cur.Payload, e.Payload) {
		res.Unchanged++
		return nil
	}
	res.Updated++
	return cur.Update().SetType(exercise.Type(e.Type)).SetTitle(e.Title).SetPayload(e.Payload).SetContentVersion(cv).Exec(ctx)
}

// same compares two values, treating a nil and an empty slice or map as equal
// because the database does not tell them apart.
func same(a, b any) bool {
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func normalize(v any) any {
	switch x := v.(type) {
	case []string:
		if len(x) == 0 {
			return []string{}
		}
	case map[string]string:
		if len(x) == 0 {
			return map[string]string{}
		}
	}
	return v
}
