package controllers

import "github.com/gin-gonic/gin"

// HealthController exposes diagnostic HTTP handlers.
type HealthController struct{}

// NewHealthController creates a new instance of HealthController.
func NewHealthController() HealthController {
	// 1.- Instantiate a HealthController with no additional state.
	return HealthController{}
}

// Status responds with a simple payload showing service availability.
func (hc HealthController) Status(ctx *gin.Context) {
	// 1.- Build a minimal JSON response with service metadata.
	payload := gin.H{
		"status":  "ok",
		"service": "Larago API",
	}

	// 2.- Send the payload with an HTTP 200 status code.
	ctx.JSON(200, payload)
}
