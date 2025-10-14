package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// 1.- TestErrorHandlerRendersEnvelope validates middleware renders structured errors.
func TestErrorHandlerRendersEnvelope(t *testing.T) {
	// 2.- Configure Gin test mode to mute logging during the run.
	gin.SetMode(gin.TestMode)

	// 3.- Register a route that emits an envelope error to be handled by the middleware.
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/boom", func(ctx *gin.Context) {
		respond.Error(ctx, http.StatusBadRequest, "boom", map[string]interface{}{"field": "invalid"})
	})

	// 4.- Dispatch the request and capture the emitted response.
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	router.ServeHTTP(recorder, req)

	// 5.- Assert the middleware surfaced the intended status code.
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	// 6.- Verify the response body matches the canonical envelope schema.
	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("expected status error, got %v", payload["status"])
	}
	if payload["message"] != "boom" {
		t.Fatalf("expected message boom, got %v", payload["message"])
	}
	errorsField, ok := payload["errors"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected errors map, got %T", payload["errors"])
	}
	if errorsField["field"] != "invalid" {
		t.Fatalf("expected field error invalid, got %v", errorsField["field"])
	}
}
