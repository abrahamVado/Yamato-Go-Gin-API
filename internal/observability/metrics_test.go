package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/internal/observability"
)

// 1.- TestMetricsRegistrationAndScrape validates that collectors register and expose samples.
func TestMetricsRegistrationAndScrape(t *testing.T) {
	// 2.- Build the metrics registry used throughout the application.
	metrics, err := observability.NewMetrics("yamato-test")
	require.NoError(t, err)

	// 3.- Record representative samples across every subsystem.
	metrics.RecordHTTP(context.Background(), "GET", "/api/health", http.StatusOK, 45*time.Millisecond)
	metrics.RecordDBQuery(context.Background(), "select", "users", true, 5*time.Millisecond)
	metrics.RecordRedisOperation(context.Background(), "get", true, 2*time.Millisecond)
	metrics.IncrementWebsocketConnections(context.Background(), "notifications")
	metrics.RecordWebsocketMessage(context.Background(), "notifications", "outbound")
	metrics.DecrementWebsocketConnections(context.Background(), "notifications")
	metrics.RecordJobRun(context.Background(), "send_digest", "success", 1500*time.Millisecond)

	// 4.- Serve the /metrics endpoint using httptest to capture the Prometheus exposition.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metrics.Handler().ServeHTTP(recorder, request)

	// 5.- Assert the handler responds successfully with the expected content type.
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")

	// 6.- Inspect the body to ensure each collector exposed a sample.
	body := recorder.Body.String()
	require.Contains(t, body, `http_requests_total{method="GET",route="/api/health",service="yamato-test",status="200"} 1`)
	require.Contains(t, body, `db_queries_total{operation="SELECT",resource="users",service="yamato-test",success="true"} 1`)
	require.Contains(t, body, `redis_commands_total{command="GET",service="yamato-test",success="true"} 1`)
	require.Contains(t, body, `websocket_messages_total{direction="outbound",scope="notifications",service="yamato-test"} 1`)
	require.Contains(t, body, `job_runs_total{name="send_digest",service="yamato-test",status="success"} 1`)

	// 7.- Verify that histograms exported latency summaries.
	require.True(t, strings.Contains(body, `http_request_duration_seconds_sum{method="GET",route="/api/health",service="yamato-test"}`))
	require.True(t, strings.Contains(body, `db_query_duration_seconds_sum{operation="SELECT",resource="users",service="yamato-test"}`))
	require.True(t, strings.Contains(body, `redis_command_duration_seconds_sum{command="GET",service="yamato-test"}`))
	require.True(t, strings.Contains(body, `job_duration_seconds_sum{name="send_digest",service="yamato-test"}`))
}
