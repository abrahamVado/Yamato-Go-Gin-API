package notifications

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- ErrNotificationNotFound signals that the requested notification is missing.
var ErrNotificationNotFound = errors.New("http/notifications: notification not found")

// 1.- Notification models the data exposed by the notifications API.
type Notification struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// 1.- Page encapsulates a slice of notifications alongside pagination metadata.
type Page struct {
	Items   []Notification
	Total   int
	Page    int
	PerPage int
}

// 1.- Service coordinates notification storage operations required by handlers.
type Service interface {
	// 2.- List retrieves a page of notifications for the given user.
	List(ctx context.Context, userID string, page int, perPage int) (Page, error)
	// 3.- MarkRead marks a notification as read and records the timestamp.
	MarkRead(ctx context.Context, userID string, notificationID string) error
}

// 1.- Handler wires Gin routes to the notification service implementation.
type Handler struct {
	service Service
}

// 1.- NewHandler creates a Handler configured with the provided service.
func NewHandler(service Service) Handler {
	return Handler{service: service}
}

// 1.- successEnvelope conforms to ADR-003 response format.
type successEnvelope struct {
	Data interface{}    `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- errorEnvelope aligns with ADR-003 error formatting guidelines.
type errorEnvelope struct {
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- newMeta initializes an empty meta map for success envelopes.
func newMeta() map[string]any {
	return map[string]any{}
}

// 1.- newErrors initializes an empty error collection for failure responses.
func newErrors() map[string]interface{} {
	return map[string]interface{}{}
}

// 1.- writeSuccess standardizes success responses across handlers.
func writeSuccess(ctx *gin.Context, status int, data interface{}, meta map[string]any) {
	ctx.JSON(status, successEnvelope{Data: data, Meta: meta})
}

// 1.- writeError standardizes error responses following ADR-003.
func writeError(ctx *gin.Context, status int, message string, errs map[string]interface{}) {
	if errs == nil {
		errs = newErrors()
	}
	ctx.JSON(status, errorEnvelope{Message: message, Errors: errs})
}

// 1.- List retrieves paginated notifications for the authenticated user.
func (h Handler) List(ctx *gin.Context) {
	// 2.- Ensure the request is authenticated and extract the subject.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "missing principal", nil)
		return
	}

	// 3.- Parse pagination parameters with sane defaults.
	page, perPage, err := parsePagination(ctx)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid pagination", map[string]interface{}{"details": err.Error()})
		return
	}

	// 4.- Query the notification service for the requested page.
	listing, err := h.service.List(contextFromGin(ctx), principal.Subject, page, perPage)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "unable to list notifications", map[string]interface{}{"details": err.Error()})
		return
	}

	// 5.- Compute pagination metadata including total pages.
	paginationMeta := map[string]any{
		"page":        listing.Page,
		"per_page":    listing.PerPage,
		"total":       listing.Total,
		"total_pages": totalPages(listing.Total, listing.PerPage),
	}

	// 6.- Deliver the notifications alongside pagination metadata.
	meta := newMeta()
	meta["pagination"] = paginationMeta
	writeSuccess(ctx, http.StatusOK, listing.Items, meta)
}

// 1.- MarkRead marks a notification as read for the authenticated user.
func (h Handler) MarkRead(ctx *gin.Context) {
	// 2.- Ensure the request is authenticated.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "missing principal", nil)
		return
	}

	// 3.- Extract the notification identifier from the path parameters.
	notificationID := ctx.Param("id")
	if notificationID == "" {
		writeError(ctx, http.StatusBadRequest, "missing notification id", nil)
		return
	}

	// 4.- Delegate to the service layer and handle domain errors.
	if err := h.service.MarkRead(contextFromGin(ctx), principal.Subject, notificationID); err != nil {
		if errors.Is(err, ErrNotificationNotFound) {
			writeError(ctx, http.StatusNotFound, "notification not found", nil)
			return
		}
		writeError(ctx, http.StatusInternalServerError, "unable to mark notification read", map[string]interface{}{"details": err.Error()})
		return
	}

	// 5.- Return a 204 status to indicate the update succeeded without body content.
	ctx.Status(http.StatusNoContent)
	ctx.Writer.WriteHeaderNow()
}

// 1.- parsePagination converts query parameters to integers with defaults.
func parsePagination(ctx *gin.Context) (int, int, error) {
	// 2.- Default to page 1 and 20 items per page as sensible defaults.
	page := 1
	perPage := 20

	// 3.- Parse the optional page parameter.
	if raw := ctx.Query("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New("page must be a positive integer")
		}
		page = parsed
	}

	// 4.- Parse the optional per_page parameter enforcing positive values.
	if raw := ctx.Query("per_page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New("per_page must be a positive integer")
		}
		perPage = parsed
	}

	return page, perPage, nil
}

// 1.- totalPages computes the number of pages available for the given totals.
func totalPages(total int, perPage int) int {
	// 2.- Protect against division by zero by returning zero pages when perPage is invalid.
	if perPage <= 0 {
		return 0
	}
	// 3.- Apply ceiling division to determine page count.
	return (total + perPage - 1) / perPage
}

// 1.- contextFromGin extracts a context.Context from the Gin request when available.
func contextFromGin(ctx *gin.Context) context.Context {
	// 2.- Use the request context when the request is present.
	if ctx.Request != nil {
		return ctx.Request.Context()
	}
	// 3.- Fall back to Background when tests omit a request.
	return context.Background()
}
