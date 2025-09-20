package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/abrahamVado/Yamato-Go-Gin-API/internal/http/response"
)

// NotImplemented is a placeholder for all contract endpoints.
func NotImplemented(c *gin.Context) {
    response.Error(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", "This endpoint is not implemented yet.")
}
