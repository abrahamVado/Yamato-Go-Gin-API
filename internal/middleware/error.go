package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// 1.- ErrorHandler centralizes rendering of structured errors emitted by handlers.
func ErrorHandler() gin.HandlerFunc {
	// 2.- Return the Gin middleware responsible for post-processing handler errors.
	return func(ctx *gin.Context) {
		// 3.- Allow handlers and downstream middleware to execute first.
		ctx.Next()

		// 4.- Skip rendering when a response has already been written or no errors exist.
		if ctx.Writer.Written() || len(ctx.Errors) == 0 {
			return
		}

		// 5.- Locate the first structured EnvelopeError attached to the context.
		for _, ginErr := range ctx.Errors {
			var envErr respond.EnvelopeError
			if errors.As(ginErr.Err, &envErr) {
				respond.WriteError(ctx, envErr)
				return
			}
		}

		// 6.- Fallback to a generic internal error when no structured error is present.
		respond.WriteError(ctx, respond.NewError(http.StatusInternalServerError, "internal server error", map[string]interface{}{}))
	}
}
