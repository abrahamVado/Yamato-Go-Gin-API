package queue

import (
	"context"
	"errors"
	"sync"
	"time"
)

// JobHandler defines the signature workers must implement.
type JobHandler func(ctx context.Context, message *Message) error

// RegisteredJob keeps metadata for a job type.
type RegisteredJob struct {
	Name       string
	Handler    JobHandler
	MaxRetries int
	Timeout    time.Duration
}

// jobRegistry stores job definitions and provides safe access.
type jobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]RegisteredJob
}

// newJobRegistry constructs an empty registry.
func newJobRegistry() *jobRegistry {
	return &jobRegistry{jobs: make(map[string]RegisteredJob)}
}

// register adds or replaces a job handler within the registry.
func (r *jobRegistry) register(job RegisteredJob) error {
	// 1.- Ensure job names are provided so lookups succeed.
	if job.Name == "" {
		return errors.New("job name is required")
	}
	// 2.- Validate a handler exists before storing the job.
	if job.Handler == nil {
		return errors.New("job handler cannot be nil")
	}
	// 3.- Set fallback retry and timeout values when omitted.
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}
	if job.Timeout == 0 {
		job.Timeout = 30 * time.Second
	}
	// 4.- Persist the job definition with a write lock.
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.Name] = job
	return nil
}

// get fetches a job definition by name if it exists.
func (r *jobRegistry) get(name string) (RegisteredJob, bool) {
	// 1.- Use a read lock because lookups occur frequently at runtime.
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[name]
	return job, ok
}

// list returns a snapshot of registered jobs.
func (r *jobRegistry) list() []RegisteredJob {
	// 1.- Capture a slice copy so callers cannot mutate internal state.
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := make([]RegisteredJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
