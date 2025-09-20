package response

import (
    "github.com/gin-gonic/gin"
)

type ErrorPayload struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
}

func Error(c *gin.Context, status int, code, message string) {
    var payload ErrorPayload
    payload.Error.Code = code
    payload.Error.Message = message
    c.JSON(status, payload)
}
