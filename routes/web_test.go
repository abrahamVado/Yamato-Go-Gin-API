package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/routes"
)

// 1.- TestRegisterRoutesServesLaragoBanner verifies the welcome route advertises the Larago branding.
func TestRegisterRoutesServesLaragoBanner(t *testing.T) {
	// 1.- Place Gin in test mode to silence logging during the request lifecycle.
	gin.SetMode(gin.TestMode)
	// 2.- Create a fresh router and register the Larago routes under test.
	router := gin.New()
	routes.RegisterRoutes(router)

	// 3.- Issue a GET request against the root path served by RegisterRoutes.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	// 4.- Exercise the router and capture the emitted response for assertions.
	router.ServeHTTP(recorder, req)

	// 5.- Confirm the handler returns HTTP 200 with the Larago banner text.
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "Welcome to the Larago API", recorder.Body.String())
}
