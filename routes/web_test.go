package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/routes"
)

// 1.- TestRegisterRoutesServesLaragoBanner verifies the welcome route advertises the Larago branding.
func TestRegisterRoutesServesLaragoBanner(t *testing.T) {
	// 2.- Place Gin in test mode to silence logging during the request lifecycle.
	gin.SetMode(gin.TestMode)
	// 3.- Create a fresh router and register the Larago routes under test.
	router := gin.New()
	routes.RegisterRoutes(router)

	// 4.- Issue a GET request against the root path served by RegisterRoutes.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	// 5.- Exercise the router and capture the emitted response for assertions.
	router.ServeHTTP(recorder, req)

	// 6.- Confirm the handler returns HTTP 200 with the Larago banner text.
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "Welcome to the Larago API", recorder.Body.String())
}

// 1.- TestRegisterRoutesExposesDashboardTasks ensures the unauthenticated tasks endpoint returns the curated dataset.
func TestRegisterRoutesExposesDashboardTasks(t *testing.T) {
	// 2.- Configure Gin for deterministic unit testing and register application routes.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.RegisterRoutes(router)

	// 3.- Issue a request against the public tasks endpoint expected by the Next.js frontend.
	request := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	// 4.- Validate that the response succeeded with the canonical envelope structure.
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}

	// 5.- Decode the JSON payload so the curated task collection can be validated.
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "success", payload.Status)
	require.Len(t, payload.Data.Items, 10)
}
