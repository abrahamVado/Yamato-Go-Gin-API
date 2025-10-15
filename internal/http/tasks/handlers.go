package tasks

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// 1.- Task models the payload consumed by the Next.js dashboard.
type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Assignee string `json:"assignee"`
	DueDate  string `json:"due_date"`
}

// 1.- Service defines the behaviour required to surface task collections.
type Service interface {
	List(ctx context.Context) ([]Task, error)
}

// 1.- Handler wires the service implementation to Gin routes.
type Handler struct {
	service Service
}

// 1.- NewHandler constructs a handler with the supplied service dependency.
func NewHandler(service Service) Handler {
	return Handler{service: service}
}

// 1.- List responds with the curated task collection for the operator dashboard.
func (h Handler) List(ctx *gin.Context) {
	var reqCtx context.Context
	if ctx.Request != nil {
		reqCtx = ctx.Request.Context()
	} else {
		reqCtx = context.Background()
	}
	tasks, err := h.service.List(reqCtx)
	if err != nil {
		respond.InternalError(ctx, err)
		return
	}
	payload := map[string]interface{}{"items": tasks}
	respond.Success(ctx, http.StatusOK, payload, map[string]interface{}{})
}
