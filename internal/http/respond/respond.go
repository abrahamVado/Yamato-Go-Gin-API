package respond

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 1.- SuccessEnvelope standardizes successful API payloads with a status indicator.
type SuccessEnvelope struct {
	Status string                 `json:"status"`
	Data   interface{}            `json:"data"`
	Meta   map[string]interface{} `json:"meta"`
}

// 1.- PaginationMeta captures pagination metadata exposed alongside list responses.
type PaginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// 1.- ErrorEnvelope represents the canonical error payload emitted by the API.
type ErrorEnvelope struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- EnvelopeError is propagated through Gin's error stack for centralized handling.
type EnvelopeError struct {
	StatusCode int
	Message    string
	Details    map[string]interface{}
}

// 1.- Error satisfies the error interface so EnvelopeError can be stored in Gin errors.
func (e EnvelopeError) Error() string {
	// 2.- Return the human-readable message to satisfy the error contract.
	return e.Message
}

// 1.- NewError constructs an EnvelopeError with defensive defaults.
func NewError(statusCode int, message string, details map[string]interface{}) EnvelopeError {
	// 2.- Ensure the details map is non-nil for deterministic JSON serialization.
	if details == nil {
		details = map[string]interface{}{}
	}
	// 3.- Return the structured error for downstream middleware consumption.
	return EnvelopeError{StatusCode: statusCode, Message: message, Details: details}
}

// 1.- Success renders a successful response using the canonical envelope structure.
func Success(ctx *gin.Context, statusCode int, data interface{}, meta map[string]interface{}) {
	// 2.- Guarantee meta is always present to simplify client-side handling.
	if meta == nil {
		meta = map[string]interface{}{}
	}
	// 3.- Emit the JSON payload directly to the response writer.
	ctx.JSON(statusCode, SuccessEnvelope{Status: "success", Data: data, Meta: meta})
}

// 1.- Paginated renders a list response enriched with pagination metadata.
func Paginated(ctx *gin.Context, statusCode int, data interface{}, pagination PaginationMeta) {
	// 2.- Compose the meta block ensuring pagination data lives under a dedicated key.
	meta := map[string]interface{}{"pagination": pagination}
	// 3.- Delegate to Success to reuse the base envelope structure.
	Success(ctx, statusCode, data, meta)
}

// 1.- Error records an EnvelopeError on the Gin context for centralized rendering.
func Error(ctx *gin.Context, statusCode int, message string, details map[string]interface{}) {
	// 2.- Attach the structured error to the Gin error stack.
	ctx.Error(NewError(statusCode, message, details))
	// 3.- Abort further handler processing so middleware can render the response.
	ctx.Abort()
}

// 1.- WriteError serializes the provided EnvelopeError using the canonical structure.
func WriteError(ctx *gin.Context, env EnvelopeError) {
	// 2.- Ensure the details map is safe for JSON serialization.
	if env.Details == nil {
		env.Details = map[string]interface{}{}
	}
	// 3.- Render the JSON payload with the encapsulated status code.
	ctx.JSON(env.StatusCode, ErrorEnvelope{Status: "error", Message: env.Message, Errors: env.Details})
}

// 1.- InternalError simplifies emitting a generic 500 response for unexpected failures.
func InternalError(ctx *gin.Context, err error) {
	// 2.- Provide a standard message while surfacing diagnostic information.
	details := map[string]interface{}{}
	if err != nil {
		details["details"] = err.Error()
	}
	// 3.- Push the error through the central middleware pipeline.
	Error(ctx, http.StatusInternalServerError, "internal server error", details)
}
