package tests

import (
	"net/http"
	"net/http/httptest"
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

	// 5.- Confirm that the response body contains the expected JSON snippet.
	expected := "\"status\":\"ok\""
	if body := recorder.Body.String(); !contains(body, expected) {
		t.Fatalf("expected body to contain %s, got %s", expected, body)
	}
}

// contains is a helper mirroring strings.Contains without importing the full package.
func contains(haystack, needle string) bool {
	// 1.- Iterate over the haystack to locate the needle manually.
	hLen := len(haystack)
	nLen := len(needle)

	// 2.- Reject impossible matches quickly.
	if nLen == 0 || nLen > hLen {
		return false
	}

	// 3.- Slide across the haystack to search for the needle.
	for i := 0; i <= hLen-nLen; i++ {
		if haystack[i:i+nLen] == needle {
			return true
		}
	}

	// 4.- Report failure when the substring is not found.
	return false
}
