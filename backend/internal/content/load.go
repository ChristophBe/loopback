package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ChristophBe/loopback/backend/gen"
)

// requiredBodySections mirrors definitions.requiredBodySections in
// api/schemas/lesson.schema.json; a test keeps the two in sync.
var requiredBodySections = []string{"Objective", "Instructions"}

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Course identifies one project in one language.
type Course struct {
	Language string
	Project  string
}

// Title is the display title derived from the project name, for example
// "redis-server" becomes "Redis Server". Content has no course title field.
func (c Course) Title() string {
	words := strings.Split(c.Project, "-")
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// Lesson is a validated lesson file.
type Lesson struct {
	File                string
	Course              Course
	Slug                string
	Order               int
	Title               string
	Minutes             int
	XP                  int
	Body                string
	OwnedFiles          []string
	Hints               []string
	FailureExplanations map[string]string
}

// Exercise is a validated quiz, flashcard or review file.
type Exercise struct {
	File    string
	Course  Course
	Slug    string
	Type    string
	Title   string
	Payload map[string]any
}

// Bundle is everything found under a content directory.
type Bundle struct {
	Lessons   []Lesson
	Exercises []Exercise
}

// Load reads and validates every lesson (*.md) and exercise (*.json) under
// root. Top-level template-* directories are skipped. When any file is
// invalid, the error is a Problems value naming every file and field.
func Load(root string) (*Bundle, error) {
	var b Bundle
	var problems Problems
	fsys := os.DirFS(root)
	err := fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), "template-") && path.Dir(rel) == "." {
				return fs.SkipDir
			}
			return nil
		}
		ext := path.Ext(rel)
		if ext != ".md" && ext != ".json" {
			return nil
		}
		data, err := fs.ReadFile(fsys, rel)
		if err != nil {
			return err
		}
		f := &file{name: rel}
		parts := strings.Split(rel, "/")
		if len(parts) != 3 {
			f.add("path", "must be <language>/<project>/<file>")
		} else if ext == ".md" {
			if l, ok := parseLesson(f, data, parts[0], parts[1]); ok {
				b.Lessons = append(b.Lessons, l)
			}
		} else if e, ok := parseExercise(f, data, parts[0], parts[1]); ok {
			b.Exercises = append(b.Exercises, e)
		}
		problems = append(problems, f.problems...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	problems = append(problems, duplicates(&b)...)
	if len(problems) > 0 {
		return nil, problems
	}
	return &b, nil
}

// duplicates reports slugs and lesson orders that occur twice in one course.
func duplicates(b *Bundle) Problems {
	var ps Problems
	slugs := map[string]string{}
	orders := map[string]string{}
	for _, l := range b.Lessons {
		if prev, ok := slugs[l.key()]; ok {
			ps = append(ps, Problem{l.File, "id", "is already used by " + prev})
		}
		slugs[l.key()] = l.File
		ordKey := fmt.Sprintf("%s/%d", l.Course.key(), l.Order)
		if prev, found := orders[ordKey]; found {
			ps = append(ps, Problem{l.File, "order", fmt.Sprintf("%d is already used by %s", l.Order, prev)})
		}
		orders[ordKey] = l.File
	}
	for _, e := range b.Exercises {
		k := e.Course.key() + "/" + e.Slug
		if prev, ok := slugs[k]; ok {
			ps = append(ps, Problem{e.File, "id", "is already used by " + prev})
		}
		slugs[k] = e.File
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].File < ps[j].File })
	return ps
}

func (c Course) key() string { return c.Language + "/" + c.Project }

func (l Lesson) key() string { return l.Course.key() + "/" + l.Slug }

// file collects the problems of one content file.
type file struct {
	name     string
	problems Problems
}

func (f *file) add(field, msg string) {
	f.problems = append(f.problems, Problem{File: f.name, Field: field, Message: msg})
}

func (f *file) addf(field, format string, args ...any) {
	f.add(field, fmt.Sprintf(format, args...))
}

// requireString reports an empty string.
func (f *file) requireString(field, v string) {
	if v == "" {
		f.add(field, "must not be empty")
	}
}

// common checks the fields every content type shares.
func (f *file) common(id, typ, wantType, language, project, title, dirLang, dirProject string) {
	if !kebab.MatchString(id) {
		f.add("id", "must be kebab-case: lowercase letters and digits separated by single -")
	}
	if typ != wantType {
		f.addf("type", "must be %q", wantType)
	}
	if language != "go" && language != "rust" {
		f.add("language", "must be go or rust")
	} else if language != dirLang {
		f.addf("language", "is %q but the directory says %q", language, dirLang)
	}
	if !kebab.MatchString(project) {
		f.add("project", "must be kebab-case")
	} else if project != dirProject {
		f.addf("project", "is %q but the directory says %q", project, dirProject)
	}
	f.requireString("title", title)
}

// shape checks that obj is an object with exactly the given required and
// optional keys. It returns false when obj is not an object.
func (f *file) shape(field string, obj any, required []string, optional ...string) bool {
	m, ok := obj.(map[string]any)
	if !ok {
		f.add(orRoot(field), "must be an object")
		return false
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			f.add(join(field, k), "is required")
		}
	}
	for k := range m {
		if !slices.Contains(required, k) && !slices.Contains(optional, k) {
			f.add(join(field, k), "is not allowed")
		}
	}
	return true
}

// items returns the elements of a list field, or nil when it is not a list.
func items(m map[string]any, key string) []any {
	l, _ := m[key].([]any)
	return l
}

func join(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func orRoot(field string) string {
	if field == "" {
		return "(root)"
	}
	return field
}

// decode converts the generic document into the generated type. A type
// mismatch is reported with the offending field.
func (f *file) decode(doc map[string]any, v any) bool {
	data, err := json.Marshal(doc)
	if err != nil {
		f.add("(root)", err.Error())
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			f.addf(te.Field, "must be of type %s, not %s", te.Type, te.Value)
		} else {
			f.add("(root)", err.Error())
		}
		return false
	}
	return true
}

func parseLesson(f *file, data []byte, dirLang, dirProject string) (Lesson, bool) {
	front, body, ok := splitFrontMatter(string(data))
	if !ok {
		f.add("(front matter)", "is missing: the file must start with a --- block")
		return Lesson{}, false
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(front), &doc); err != nil {
		f.add("(front matter)", "is not valid YAML: "+err.Error())
		return Lesson{}, false
	}
	required := []string{"id", "type", "language", "project", "order", "title", "minutes", "xp", "ownedFiles"}
	if !f.shape("", doc, required, "hints", "failureExplanations") {
		return Lesson{}, false
	}
	var fm gen.LessonSchemaJson
	if !f.decode(doc, &fm) || len(f.problems) > 0 {
		return Lesson{}, false
	}
	f.common(fm.ID, string(fm.Type), "lesson", string(fm.Language), fm.Project, fm.Title, dirLang, dirProject)
	if fm.Order < 1 {
		f.add("order", "must be 1 or more")
	}
	if fm.Minutes < 1 || fm.Minutes > 15 {
		f.add("minutes", "must be between 1 and 15")
	}
	if fm.Xp < 1 {
		f.add("xp", "must be 1 or more")
	}
	checkStrings(f, "ownedFiles", fm.OwnedFiles, true)
	checkStrings(f, "hints", fm.Hints, false)
	for _, k := range sortedKeys(fm.FailureExplanations) {
		f.requireString("failureExplanations."+k, fm.FailureExplanations[k])
	}
	checkBody(f, body)
	if len(f.problems) > 0 {
		return Lesson{}, false
	}
	return Lesson{
		File:                f.name,
		Course:              Course{dirLang, dirProject},
		Slug:                fm.ID,
		Order:               fm.Order,
		Title:               fm.Title,
		Minutes:             fm.Minutes,
		XP:                  fm.Xp,
		Body:                strings.TrimSpace(body),
		OwnedFiles:          fm.OwnedFiles,
		Hints:               fm.Hints,
		FailureExplanations: fm.FailureExplanations,
	}, true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkStrings reports empty entries and, when unique is set, duplicates.
func checkStrings(f *file, field string, vs []string, unique bool) {
	seen := map[string]bool{}
	for i, v := range vs {
		if v == "" {
			f.add(fmt.Sprintf("%s[%d]", field, i), "must not be empty")
		}
		if unique && seen[v] {
			f.addf(fmt.Sprintf("%s[%d]", field, i), "duplicates %q", v)
		}
		seen[v] = true
	}
}

// splitFrontMatter splits a file that starts with a --- delimited YAML block.
func splitFrontMatter(text string) (front, body string, ok bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rest, found := strings.CutPrefix(text, "---\n")
	if !found {
		return "", "", false
	}
	if front, body, found = strings.Cut(rest, "\n---\n"); found {
		return front, body, true
	}
	if front, found = strings.CutSuffix(rest, "\n---"); found {
		return front, "", true
	}
	return "", "", false
}

// checkBody requires the level-2 headings in order, ignoring fenced code.
func checkBody(f *file, body string) {
	var found []string
	fence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			fence = !fence
		case !fence && strings.HasPrefix(line, "## "):
			found = append(found, strings.TrimSpace(line[3:]))
		}
	}
	missing := false
	for _, s := range requiredBodySections {
		if !slices.Contains(found, s) {
			f.addf("body", "is missing the section %q", "## "+s)
			missing = true
		}
	}
	if missing {
		return
	}
	var order []string
	for _, h := range found {
		if slices.Contains(requiredBodySections, h) {
			order = append(order, h)
		}
	}
	if !slices.Equal(order, requiredBodySections) {
		f.addf("body", "must contain the sections %q in this order", requiredBodySections)
	}
}

// exerciseKeys are the fields of an exercise file that are not payload.
var exerciseKeys = []string{"id", "type", "language", "project", "title"}

func parseExercise(f *file, data []byte, dirLang, dirProject string) (Exercise, bool) {
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil {
		f.add("(root)", "is not valid JSON: "+err.Error())
		return Exercise{}, false
	}
	typ, _ := doc["type"].(string)
	var (
		payloadKeys []string
		check       func()
		common      func() (id, title, lang, project string)
	)
	switch typ {
	case "quiz":
		var q gen.QuizSchemaJson
		payloadKeys = []string{"questions"}
		if !f.exerciseShape(doc, payloadKeys) || !f.decode(doc, &q) {
			return Exercise{}, false
		}
		common = func() (string, string, string, string) { return q.ID, q.Title, string(q.Language), q.Project }
		check = func() { checkQuiz(f, doc, q) }
	case "flashcard":
		var c gen.FlashcardSchemaJson
		payloadKeys = []string{"cards"}
		if !f.exerciseShape(doc, payloadKeys) || !f.decode(doc, &c) {
			return Exercise{}, false
		}
		common = func() (string, string, string, string) { return c.ID, c.Title, string(c.Language), c.Project }
		check = func() { checkFlashcard(f, doc, c) }
	case "review":
		var r gen.ReviewSchemaJson
		payloadKeys = []string{"code", "flaggedLines"}
		if !f.exerciseShape(doc, payloadKeys) || !f.decode(doc, &r) {
			return Exercise{}, false
		}
		common = func() (string, string, string, string) { return r.ID, r.Title, string(r.Language), r.Project }
		check = func() { checkReview(f, doc, r) }
	default:
		f.add("type", "must be one of quiz, flashcard, review")
		return Exercise{}, false
	}
	if len(f.problems) > 0 {
		return Exercise{}, false
	}
	id, title, lang, project := common()
	f.common(id, typ, typ, lang, project, title, dirLang, dirProject)
	check()
	if len(f.problems) > 0 {
		return Exercise{}, false
	}
	payload := map[string]any{}
	for _, k := range payloadKeys {
		payload[k] = doc[k]
	}
	return Exercise{File: f.name, Course: Course{dirLang, dirProject}, Slug: id, Type: typ, Title: title, Payload: payload}, true
}

// exerciseShape checks the top-level keys of an exercise file.
func (f *file) exerciseShape(doc map[string]any, payloadKeys []string) bool {
	f.shape("", doc, append(slices.Clone(exerciseKeys), payloadKeys...))
	return len(f.problems) == 0
}

func checkQuiz(f *file, doc map[string]any, q gen.QuizSchemaJson) {
	if len(q.Questions) == 0 {
		f.add("questions", "must not be empty")
	}
	for i, raw := range items(doc, "questions") {
		field := fmt.Sprintf("questions[%d]", i)
		if !f.shape(field, raw, []string{"prompt", "choices", "answer", "explanation"}) {
			continue
		}
		qu := q.Questions[i]
		f.requireString(field+".prompt", qu.Prompt)
		f.requireString(field+".explanation", qu.Explanation)
		if len(qu.Choices) < 2 {
			f.add(field+".choices", "must have at least 2 entries")
		}
		checkStrings(f, field+".choices", qu.Choices, true)
		if qu.Answer < 0 || qu.Answer >= len(qu.Choices) {
			f.add(field+".answer", "must be an index into choices")
		}
	}
}

func checkFlashcard(f *file, doc map[string]any, c gen.FlashcardSchemaJson) {
	if len(c.Cards) == 0 {
		f.add("cards", "must not be empty")
	}
	for i, raw := range items(doc, "cards") {
		field := fmt.Sprintf("cards[%d]", i)
		if !f.shape(field, raw, []string{"front", "back"}) {
			continue
		}
		f.requireString(field+".front", c.Cards[i].Front)
		f.requireString(field+".back", c.Cards[i].Back)
	}
}

func checkReview(f *file, doc map[string]any, r gen.ReviewSchemaJson) {
	if f.shape("code", doc["code"], []string{"language", "source"}) {
		if r.Code.Language != gen.ReviewSchemaJsonCodeLanguageGo && r.Code.Language != gen.ReviewSchemaJsonCodeLanguageRust {
			f.add("code.language", "must be go or rust")
		}
		f.requireString("code.source", r.Code.Source)
	}
	if len(r.FlaggedLines) == 0 {
		f.add("flaggedLines", "must not be empty")
	}
	for i, raw := range items(doc, "flaggedLines") {
		field := fmt.Sprintf("flaggedLines[%d]", i)
		if !f.shape(field, raw, []string{"line", "note"}) {
			continue
		}
		fl := r.FlaggedLines[i]
		if fl.Line < 1 {
			f.add(field+".line", "must be 1 or more")
		}
		f.requireString(field+".note", fl.Note)
	}
}
