package tasks

import (
	"context"
	"database/sql"
	"errors"
	"time"

	taskhttp "github.com/example/Yamato-Go-Gin-API/internal/http/tasks"
)

// 1.- Repository provides a Postgres-backed implementation of the tasks.Service interface.
type Repository struct {
	db *sql.DB
}

// 1.- NewRepository validates the database handle and prepares the repository.
func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("tasks repository requires a database connection")
	}
	return &Repository{db: db}, nil
}

// 1.- List returns the persisted tasks ordered by due date and identifier.
func (r *Repository) List(ctx context.Context) ([]taskhttp.Task, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("tasks repository is not initialized")
	}

	const query = `
SELECT id, title, status, priority, assignee, due_date
FROM tasks
ORDER BY due_date ASC, id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskhttp.Task, 0)
	for rows.Next() {
		var (
			id       string
			title    string
			status   string
			priority string
			assignee string
			dueDate  time.Time
		)
		if scanErr := rows.Scan(&id, &title, &status, &priority, &assignee, &dueDate); scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, taskhttp.Task{
			ID:       id,
			Title:    title,
			Status:   status,
			Priority: priority,
			Assignee: assignee,
			DueDate:  dueDate.UTC().Format(time.RFC3339),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}
