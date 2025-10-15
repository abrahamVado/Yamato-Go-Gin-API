package memory

import (
	"context"
	"sync"
	"time"

	taskhttp "github.com/example/Yamato-Go-Gin-API/internal/http/tasks"
)

// 1.- TaskService delivers a static slice of tasks for the Next.js dashboard integration.
type TaskService struct {
	mu    sync.RWMutex
	tasks []taskhttp.Task
}

// 1.- NewTaskService stores a defensive copy of the supplied tasks.
func NewTaskService(tasks []taskhttp.Task) *TaskService {
	cloned := make([]taskhttp.Task, len(tasks))
	copy(cloned, tasks)
	return &TaskService{tasks: cloned}
}

// 1.- List returns the known task collection; callers receive a fresh copy on each request.
func (s *TaskService) List(_ context.Context) ([]taskhttp.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cloned := make([]taskhttp.Task, len(s.tasks))
	copy(cloned, s.tasks)
	return cloned, nil
}

// 1.- DefaultTasks produces a curated set of ten dashboard tasks.
func DefaultTasks() []taskhttp.Task {
	due := func(days int) string {
		return time.Now().AddDate(0, 0, days).Format(time.RFC3339)
	}
	return []taskhttp.Task{
		{ID: "TASK-510", Title: "Refine billing onboarding", Status: "In Progress", Priority: "High", Assignee: "Olivia Martin", DueDate: due(3)},
		{ID: "TASK-511", Title: "Document AI guardrails", Status: "Todo", Priority: "Medium", Assignee: "Isabella Nguyen", DueDate: due(5)},
		{ID: "TASK-512", Title: "Ship analytics export", Status: "Blocked", Priority: "High", Assignee: "Jackson Lee", DueDate: due(2)},
		{ID: "TASK-513", Title: "Update policy meshes", Status: "Done", Priority: "Low", Assignee: "William Kim", DueDate: due(-1)},
		{ID: "TASK-514", Title: "QA autopilot", Status: "In Review", Priority: "Medium", Assignee: "Sofia Davis", DueDate: due(1)},
		{ID: "TASK-515", Title: "Prototype webhooks", Status: "Todo", Priority: "Medium", Assignee: "Mia Chen", DueDate: due(7)},
		{ID: "TASK-516", Title: "Scale infra agents", Status: "In Progress", Priority: "High", Assignee: "Noah Patel", DueDate: due(4)},
		{ID: "TASK-517", Title: "Localize cockpit copy", Status: "Todo", Priority: "Low", Assignee: "Ava Rodríguez", DueDate: due(9)},
		{ID: "TASK-518", Title: "Audit role assignments", Status: "In Review", Priority: "High", Assignee: "Ethan Brooks", DueDate: due(6)},
		{ID: "TASK-519", Title: "Benchmark data lake", Status: "Blocked", Priority: "Critical", Assignee: "Liam Hughes", DueDate: due(0)},
	}
}
