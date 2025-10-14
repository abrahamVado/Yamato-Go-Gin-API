package joinrequests

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- Status represents the lifecycle state of a join request.
type Status string

const (
	// 1.- StatusPending indicates the request is awaiting a decision.
	StatusPending Status = "pending"
	// 1.- StatusApproved indicates the request has been accepted.
	StatusApproved Status = "approved"
	// 1.- StatusDeclined indicates the request has been rejected.
	StatusDeclined Status = "declined"
)

// 1.- ErrJoinRequestNotFound signals the target join request is missing.
var ErrJoinRequestNotFound = errors.New("http/joinrequests: join request not found")

// 1.- ErrInvalidStatusTransition is returned when applying an invalid status change.
var ErrInvalidStatusTransition = errors.New("http/joinrequests: invalid status transition")

// 1.- Submission captures the data required to create a join request.
type Submission struct {
	User    string         `json:"user"`
	Email   string         `json:"email"`
	Payload map[string]any `json:"payload"`
}

// 1.- Filter specifies the allowed filters for listing join requests.
type Filter struct {
	Status Status
}

// 1.- Decision represents an administrative decision and its audit metadata.
type Decision struct {
	ActorID string
	Note    string
}

// 1.- AuditEntry records lifecycle changes for a join request.
type AuditEntry struct {
	ActorID    string    `json:"actor_id"`
	Action     string    `json:"action"`
	Note       string    `json:"note,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// 1.- JoinRequest models the API representation returned to clients.
type JoinRequest struct {
	ID         string         `json:"id"`
	User       string         `json:"user"`
	Email      string         `json:"email"`
	Status     Status         `json:"status"`
	Payload    map[string]any `json:"payload"`
	AuditTrail []AuditEntry   `json:"audit_trail"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// 1.- Service defines the persistence contract required by the handlers.
type Service interface {
	// 2.- Submit persists a new join request from a public submission.
	Submit(ctx context.Context, submission Submission) (JoinRequest, error)
	// 3.- List retrieves join requests matching the provided filter.
	List(ctx context.Context, filter Filter) ([]JoinRequest, error)
	// 4.- Approve transitions a join request into the approved state.
	Approve(ctx context.Context, id string, decision Decision) (JoinRequest, error)
	// 5.- Decline transitions a join request into the declined state.
	Decline(ctx context.Context, id string, decision Decision) (JoinRequest, error)
}

// 1.- Handler wires HTTP requests into the join request service.
type Handler struct {
	service Service
}

// 1.- NewHandler constructs a Handler bound to the provided service.
func NewHandler(service Service) Handler {
	return Handler{service: service}
}

// 1.- successEnvelope standardizes success responses per ADR-003.
type successEnvelope struct {
	Data interface{}    `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- errorEnvelope standardizes error responses per ADR-003.
type errorEnvelope struct {
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- writeSuccess serializes success responses with optional metadata.
func writeSuccess(ctx *gin.Context, status int, data interface{}, meta map[string]any) {
	if meta == nil {
		meta = map[string]any{}
	}
	ctx.JSON(status, successEnvelope{Data: data, Meta: meta})
}

// 1.- writeError serializes error responses with structured errors.
func writeError(ctx *gin.Context, status int, message string, errs map[string]interface{}) {
	if errs == nil {
		errs = map[string]interface{}{}
	}
	ctx.JSON(status, errorEnvelope{Message: message, Errors: errs})
}

// 1.- requirePrincipal extracts the authenticated principal or writes 401.
func requirePrincipal(ctx *gin.Context) (internalauth.Principal, bool) {
	// 2.- Attempt to read the principal from the Gin context bag.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "missing principal", nil)
		return internalauth.Principal{}, false
	}
	return principal, true
}

// 1.- contextFromGin safely obtains a context from the Gin request.
func contextFromGin(ctx *gin.Context) context.Context {
	// 2.- Use the request context when it exists.
	if ctx.Request != nil {
		return ctx.Request.Context()
	}
	// 3.- Fall back to background when tests omit the request.
	return context.Background()
}

// 1.- Submit handles public join request submissions.
func (h Handler) Submit(ctx *gin.Context) {
	// 2.- Decode the incoming payload into a structured request.
	var req Submission
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid join request payload", map[string]interface{}{"details": err.Error()})
		return
	}

	// 3.- Validate the user name is provided.
	trimmedUser := strings.TrimSpace(req.User)
	if trimmedUser == "" {
		writeError(ctx, http.StatusUnprocessableEntity, "validation error", map[string]interface{}{"user": "user is required"})
		return
	}

	// 4.- Validate the email conforms to RFC 5322 format.
	trimmedEmail := strings.TrimSpace(req.Email)
	if trimmedEmail == "" {
		writeError(ctx, http.StatusUnprocessableEntity, "validation error", map[string]interface{}{"email": "email is required"})
		return
	}
	if _, err := mail.ParseAddress(trimmedEmail); err != nil {
		writeError(ctx, http.StatusUnprocessableEntity, "validation error", map[string]interface{}{"email": "email must be valid"})
		return
	}

	// 5.- Normalize fields before handing off to the service.
	req.User = trimmedUser
	req.Email = strings.ToLower(trimmedEmail)

	// 6.- Persist the join request via the service.
	stored, err := h.service.Submit(contextFromGin(ctx), req)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "unable to store join request", map[string]interface{}{"details": err.Error()})
		return
	}

	// 7.- Respond with the stored join request representation.
	writeSuccess(ctx, http.StatusCreated, stored, nil)
}

// 1.- List exposes administrative join request listings with optional filtering.
func (h Handler) List(ctx *gin.Context) {
	// 2.- Ensure the caller is authenticated.
	if _, ok := requirePrincipal(ctx); !ok {
		return
	}

	// 3.- Parse the optional status filter from query parameters.
	filter := Filter{}
	if rawStatus := strings.TrimSpace(ctx.Query("status")); rawStatus != "" {
		switch Status(rawStatus) {
		case StatusPending, StatusApproved, StatusDeclined:
			filter.Status = Status(rawStatus)
		default:
			writeError(ctx, http.StatusBadRequest, "invalid status filter", map[string]interface{}{"status": "unsupported status value"})
			return
		}
	}

	// 4.- Fetch join requests matching the filter.
	items, err := h.service.List(contextFromGin(ctx), filter)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "unable to list join requests", map[string]interface{}{"details": err.Error()})
		return
	}

	// 5.- Return the collection to the caller.
	writeSuccess(ctx, http.StatusOK, items, nil)
}

// 1.- Approve transitions a join request into the approved state with audit logging.
func (h Handler) Approve(ctx *gin.Context) {
	// 2.- Ensure the caller is authenticated.
	principal, ok := requirePrincipal(ctx)
	if !ok {
		return
	}

	// 3.- Extract the join request identifier from the path.
	joinRequestID := ctx.Param("id")
	if joinRequestID == "" {
		writeError(ctx, http.StatusBadRequest, "missing join request id", nil)
		return
	}

	// 4.- Decode the optional decision body for audit context.
	var payload struct {
		Note string `json:"note"`
	}
	if ctx.Request != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			writeError(ctx, http.StatusBadRequest, "invalid decision payload", map[string]interface{}{"details": err.Error()})
			return
		}
	}

	// 5.- Delegate to the service for the status transition.
	updated, err := h.service.Approve(contextFromGin(ctx), joinRequestID, Decision{ActorID: principal.Subject, Note: strings.TrimSpace(payload.Note)})
	if err != nil {
		if errors.Is(err, ErrJoinRequestNotFound) {
			writeError(ctx, http.StatusNotFound, "join request not found", nil)
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			writeError(ctx, http.StatusConflict, "invalid status transition", nil)
			return
		}
		writeError(ctx, http.StatusInternalServerError, "unable to approve join request", map[string]interface{}{"details": err.Error()})
		return
	}

	// 6.- Return the updated resource to the client.
	writeSuccess(ctx, http.StatusOK, updated, nil)
}

// 1.- Decline transitions a join request into the declined state with audit logging.
func (h Handler) Decline(ctx *gin.Context) {
	// 2.- Ensure the caller is authenticated.
	principal, ok := requirePrincipal(ctx)
	if !ok {
		return
	}

	// 3.- Extract the join request identifier from the path.
	joinRequestID := ctx.Param("id")
	if joinRequestID == "" {
		writeError(ctx, http.StatusBadRequest, "missing join request id", nil)
		return
	}

	// 4.- Decode the optional decision body for audit context.
	var payload struct {
		Note string `json:"note"`
	}
	if ctx.Request != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			writeError(ctx, http.StatusBadRequest, "invalid decision payload", map[string]interface{}{"details": err.Error()})
			return
		}
	}

	// 5.- Delegate to the service for the status transition.
	updated, err := h.service.Decline(contextFromGin(ctx), joinRequestID, Decision{ActorID: principal.Subject, Note: strings.TrimSpace(payload.Note)})
	if err != nil {
		if errors.Is(err, ErrJoinRequestNotFound) {
			writeError(ctx, http.StatusNotFound, "join request not found", nil)
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			writeError(ctx, http.StatusConflict, "invalid status transition", nil)
			return
		}
		writeError(ctx, http.StatusInternalServerError, "unable to decline join request", map[string]interface{}{"details": err.Error()})
		return
	}

	// 6.- Return the updated resource to the client.
	writeSuccess(ctx, http.StatusOK, updated, nil)
}
