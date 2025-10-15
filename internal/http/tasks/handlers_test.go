package tasks

import (
        "context"
        "errors"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/gin-gonic/gin"
        "github.com/stretchr/testify/require"

        "github.com/example/Yamato-Go-Gin-API/internal/middleware"
)

// 1.- stubService implements the Service interface for deterministic unit tests.
type stubService struct {
	tasks []Task
	err   error
}

// 1.- List returns either the configured tasks or propagates the configured error.
func (s *stubService) List(_ context.Context) ([]Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tasks, nil
}

// 1.- TestListReturnsTasks verifies that the handler responds with the task collection.
func TestListReturnsTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&stubService{tasks: []Task{{ID: "TASK-1", Title: "Bootstrap"}}})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks", nil)

	handler.List(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "TASK-1")
	require.Contains(t, recorder.Body.String(), "items")
}

// 1.- TestListHandlesErrors ensures service failures propagate as 500 responses.
func TestListHandlesErrors(t *testing.T) {
        gin.SetMode(gin.TestMode)
        handler := NewHandler(&stubService{err: errors.New("boom")})

        //1.- Exercise the handler through a Gin engine so the error middleware renders the structured response.
        engine := gin.New()
        engine.Use(middleware.ErrorHandler())
        engine.GET("/v1/tasks", handler.List)

        recorder := httptest.NewRecorder()
        engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/tasks", nil))

        require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
