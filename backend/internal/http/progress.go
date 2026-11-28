package http

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/ChristophBe/loopback/backend/gen"
)

// CompleteLesson is not implemented yet.
func (s *Server) CompleteLesson(context.Context, gen.CompleteLessonRequestObject) (gen.CompleteLessonResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// CreateExerciseAttempt is not implemented yet.
func (s *Server) CreateExerciseAttempt(context.Context, gen.CreateExerciseAttemptRequestObject) (gen.CreateExerciseAttemptResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// GetMe is not implemented yet.
func (s *Server) GetMe(context.Context, gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	return nil, echo.ErrNotImplemented
}

// GetMyProgress is not implemented yet.
func (s *Server) GetMyProgress(context.Context, gen.GetMyProgressRequestObject) (gen.GetMyProgressResponseObject, error) {
	return nil, echo.ErrNotImplemented
}
