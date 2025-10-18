package tasks

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/internal/storage"
	"github.com/example/Yamato-Go-Gin-API/internal/testutil"
)

// 1.- TestRepositoryList exercises the Postgres-backed task repository end-to-end.
func TestRepositoryList(t *testing.T) {
	container := testutil.RunPostgresContainer(t)
	if container == nil {
		t.Skip("postgres container unavailable")
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

	// 2.- Seed a pair of tasks spanning different due dates.
	_, err = db.ExecContext(ctx, `
INSERT INTO tasks (id, title, status, priority, assignee, due_date)
VALUES
  ($1, $2, $3, $4, $5, $6),
  ($7, $8, $9, $10, $11, $12);
`,
		"TASK-200", "Calibrate warehouse drones", "In Progress", "High", "Alex Kim", time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC),
		"TASK-150", "Draft compliance policy", "Todo", "Medium", "Jordan Blake", time.Date(2024, 1, 5, 9, 30, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	repo, repoErr := NewRepository(db)
	require.NoError(t, repoErr)

	tasks, listErr := repo.List(ctx)
	require.NoError(t, listErr)
	require.Len(t, tasks, 2)

	// 3.- Verify tasks are ordered by due date and serialized using RFC3339 strings.
	require.Equal(t, "TASK-150", tasks[0].ID)
	require.Equal(t, "Draft compliance policy", tasks[0].Title)
	require.Equal(t, time.Date(2024, 1, 5, 9, 30, 0, 0, time.UTC).Format(time.RFC3339), tasks[0].DueDate)
	require.Equal(t, "TASK-200", tasks[1].ID)
	require.Equal(t, "Calibrate warehouse drones", tasks[1].Title)
	require.Equal(t, time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339), tasks[1].DueDate)
}
