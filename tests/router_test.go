package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/bootstrap"
	"github.com/example/Yamato-Go-Gin-API/internal/storage"
	"github.com/example/Yamato-Go-Gin-API/internal/testutil"
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

// TestTasksEndpointUsesPostgres ensures the task handler queries Postgres when configured.
func TestTasksEndpointUsesPostgres(t *testing.T) {
	container := testutil.RunPostgresContainer(t)
	if container == nil {
		t.Skip("postgres container not available")
		return
	}

	db, err := sql.Open("postgres", container.DSN)
	require.NoError(t, err)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	migrator, err := storage.NewMigrator(db)
	require.NoError(t, err)
	require.NoError(t, migrator.Apply(ctx))

	_, err = db.ExecContext(ctx, `
INSERT INTO tasks (id, title, status, priority, assignee, due_date)
VALUES
  ($1, $2, $3, $4, $5, $6),
  ($7, $8, $9, $10, $11, $12);
`,
		"TASK-300", "Publish release notes", "In Review", "Medium", "Lee Harper", time.Date(2024, 2, 1, 15, 0, 0, 0, time.UTC),
		"TASK-250", "Migrate audit logs", "Todo", "High", "Morgan Wu", time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	t.Setenv("DATABASE_URL", container.DSN)

	router, _ := bootstrap.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
		Meta map[string]interface{} `json:"meta"`
	}

	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "success", payload.Status)
	require.Len(t, payload.Data.Items, 2)

	first := payload.Data.Items[0]
	require.Equal(t, "TASK-250", first["id"])
	require.Equal(t, "Migrate audit logs", first["title"])
	require.Equal(t, time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC).Format(time.RFC3339), first["due_date"])
}
