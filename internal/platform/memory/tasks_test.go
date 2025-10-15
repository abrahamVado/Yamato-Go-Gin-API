package memory

import (
	"context"
	"testing"

	taskhttp "github.com/example/Yamato-Go-Gin-API/internal/http/tasks"
)

// 1.- TestDefaultTasksProvidesTenItems ensures the curated list powers the dashboard with enough sample data.
func TestDefaultTasksProvidesTenItems(t *testing.T) {
	tasks := DefaultTasks()
	if len(tasks) != 10 {
		t.Fatalf("expected 10 tasks, got %d", len(tasks))
	}
	for index, task := range tasks {
		//1.- Validate that each task exposes the key fields consumed by the Next.js table.
		if task.ID == "" {
			t.Fatalf("task at index %d is missing an ID", index)
		}
		if task.Title == "" {
			t.Fatalf("task at index %d is missing a title", index)
		}
		if task.Status == "" {
			t.Fatalf("task at index %d is missing a status", index)
		}
	}
}

// 1.- TestTaskServiceListReturnsDefensiveCopy verifies consumers receive isolated slices each time List is invoked.
func TestTaskServiceListReturnsDefensiveCopy(t *testing.T) {
	original := []taskhttp.Task{{ID: "TASK-100", Title: "Draft roadmap"}}
	svc := NewTaskService(original)

	first, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 task, got %d", len(first))
	}

	//1.- Mutate the returned slice and ensure the service keeps its internal copy intact.
	first[0].Title = "Mutated"

	second, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if second[0].Title != "Draft roadmap" {
		t.Fatalf("expected original title, got %s", second[0].Title)
	}
}
