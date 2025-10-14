package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
	"github.com/example/Yamato-Go-Gin-API/internal/observability"
)

const requestIDHeader = "X-Request-ID"

// JSONOnly rejects non-JSON payloads for mutating HTTP verbs to keep handlers consistent.
func JSONOnly() gin.HandlerFunc {
	// 1.- Return the Gin middleware responsible for enforcing the content-type contract.
	return func(ctx *gin.Context) {
		// 2.- Skip validation for safe verbs that typically avoid request bodies.
		switch ctx.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
			ctx.Next()
			return
		}

		// 3.- Allow empty bodies so health checks or intent-style requests are not rejected.
		if ctx.Request.ContentLength == 0 {
			ctx.Next()
			return
		}

		// 4.- Inspect the declared content type to ensure JSON payloads reach the handlers.
		contentType := strings.ToLower(strings.TrimSpace(ctx.GetHeader("Content-Type")))
		if !strings.HasPrefix(contentType, "application/json") {
			env := respond.NewError(http.StatusUnsupportedMediaType, "content type must be application/json", map[string]interface{}{"content_type": contentType})
			respond.WriteError(ctx, env)
			ctx.Abort()
			return
		}

		// 5.- Continue down the middleware chain once the contract has been satisfied.
		ctx.Next()
	}
}

// RequestID attaches a unique identifier to every HTTP request for traceability.
func RequestID() gin.HandlerFunc {
	// 1.- Return the Gin middleware that seeds the request ID if absent.
	return func(ctx *gin.Context) {
		// 2.- Prefer inbound identifiers to support distributed tracing scenarios.
		requestID := strings.TrimSpace(ctx.GetHeader(requestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		// 3.- Surface the identifier in both the response headers and request context.
		ctx.Writer.Header().Set(requestIDHeader, requestID)
		ctx.Set("request_id", requestID)

		// 4.- Mirror the request ID into the standard context so observability packages can correlate events.
		if ctx.Request != nil {
			ctx.Request = ctx.Request.WithContext(observability.ContextWithRequestID(ctx.Request.Context(), requestID))
		}

		// 5.- Continue processing downstream middleware and handlers.
		ctx.Next()
	}
}
