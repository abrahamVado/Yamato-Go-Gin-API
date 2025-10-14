package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/Yamato-Go-Gin-API/bootstrap"
)

// TestHealthRoute ensures that the health endpoint returns the expected payload.
func TestHealthRoute(t *testing.T) {
	// 1.- Boot the Gin router with application routes.
	router, _ := bootstrap.SetupRouter()

	// 2.- Prepare an HTTP request targeting the health endpoint.
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// 3.- Record the response emitted by the router.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 4.- Validate that the response code indicates success.
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	// 5.- Parse the response body and ensure the status flag reports ok.
	var envelope map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %T", envelope["data"])
	}
	if status, ok := data["status"].(string); !ok || status != "ok" {
		t.Fatalf("expected status ok, got %v (%t)", status, ok)
	}
}

// TestMetricsRoute exposes the Prometheus endpoint when metrics are configured.
func TestMetricsRoute(t *testing.T) {
	// 1.- Boot the Gin router with application routes.
	router, _ := bootstrap.SetupRouter()

	// 2.- Prepare an HTTP request targeting the Prometheus metrics endpoint.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	// 3.- Record the response emitted by the router.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 4.- Validate that the response code indicates success.
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	// 5.- Confirm the response advertises the Prometheus exposition media type.
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/plain") {
		t.Fatalf("expected metrics content type, got %q", contentType)
	}
}
