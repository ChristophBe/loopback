package http

import (
	"context"

	"github.com/ChristophBe/loopback/backend/ent"
	"github.com/ChristophBe/loopback/backend/ent/course"
	"github.com/ChristophBe/loopback/backend/ent/exercise"
	"github.com/ChristophBe/loopback/backend/ent/lesson"
	"github.com/ChristophBe/loopback/backend/gen"
)

// ListCourses returns every course, ordered by language and project.
func (s *Server) ListCourses(ctx context.Context, _ gen.ListCoursesRequestObject) (gen.ListCoursesResponseObject, error) {
	rows, err := s.db.Course.Query().Order(ent.Asc(course.FieldLanguage), ent.Asc(course.FieldProject)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListCourses200JSONResponse, len(rows))
	for i, c := range rows {
		out[i] = gen.Course{Id: c.ID, Language: gen.Language(c.Language), Project: c.Project, Title: c.Title}
	}
	return out, nil
}

// ListCourseLessons returns the lessons of a course in path order.
func (s *Server) ListCourseLessons(ctx context.Context, req gen.ListCourseLessonsRequestObject) (gen.ListCourseLessonsResponseObject, error) {
	exists, err := s.db.Course.Query().Where(course.ID(req.Id)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return gen.ListCourseLessons404JSONResponse{Error: gen.NotFound}, nil
	}
	rows, err := s.db.Lesson.Query().Where(lesson.HasCourseWith(course.ID(req.Id))).Order(ent.Asc(lesson.FieldOrder)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListCourseLessons200JSONResponse, len(rows))
	for i, l := range rows {
		out[i] = toLesson(l, req.Id)
	}
	return out, nil
}

// GetLesson returns one lesson.
func (s *Server) GetLesson(ctx context.Context, req gen.GetLessonRequestObject) (gen.GetLessonResponseObject, error) {
	l, err := s.db.Lesson.Query().Where(lesson.ID(req.Id)).WithCourse().Only(ctx)
	if ent.IsNotFound(err) {
		return gen.GetLesson404JSONResponse{Error: gen.NotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetLesson200JSONResponse(toLesson(l, l.Edges.Course.ID)), nil
}

// ListExercises returns the exercises, optionally filtered by course or type.
func (s *Server) ListExercises(ctx context.Context, req gen.ListExercisesRequestObject) (gen.ListExercisesResponseObject, error) {
	q := s.db.Exercise.Query().WithCourse().Order(ent.Asc(exercise.FieldID))
	if req.Params.CourseId != nil {
		q = q.Where(exercise.HasCourseWith(course.ID(*req.Params.CourseId)))
	}
	if t := req.Params.Type; t != nil {
		if err := exercise.TypeValidator(exercise.Type(*t)); err != nil {
			return gen.ListExercises400JSONResponse{ErrorJSONResponse: gen.ErrorJSONResponse{Error: gen.InvalidRequest}}, nil
		}
		q = q.Where(exercise.TypeEQ(exercise.Type(*t)))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListExercises200JSONResponse, len(rows))
	for i, e := range rows {
		payload := e.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		out[i] = gen.Exercise{
			Id: e.ID, CourseId: e.Edges.Course.ID, Slug: e.Slug,
			Type: gen.ExerciseType(e.Type), Title: e.Title, Payload: payload,
		}
	}
	return out, nil
}

// toLesson maps a stored lesson to the API type. Optional JSON columns are
// null for lessons without them, but the API always sends empty values.
func toLesson(l *ent.Lesson, courseID int) gen.Lesson {
	out := gen.Lesson{
		Id: l.ID, CourseId: courseID, Slug: l.Slug, Order: l.Order, Title: l.Title,
		Minutes: l.Minutes, Xp: l.Xp, Body: l.Body,
		OwnedFiles: l.OwnedFiles, StarterFiles: l.StarterFiles, TestFiles: l.TestFiles,
		Hints: l.Hints, FailureExplanations: l.FailureExplanations,
	}
	if out.OwnedFiles == nil {
		out.OwnedFiles = []string{}
	}
	if out.StarterFiles == nil {
		out.StarterFiles = map[string]string{}
	}
	if out.TestFiles == nil {
		out.TestFiles = map[string]string{}
	}
	if out.Hints == nil {
		out.Hints = []string{}
	}
	if out.FailureExplanations == nil {
		out.FailureExplanations = map[string]string{}
	}
	return out
}
