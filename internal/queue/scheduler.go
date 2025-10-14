package queue

import (
	"context"
	"errors"

	"github.com/robfig/cron/v3"

	"github.com/example/Yamato-Go-Gin-API/config"
)

// EnqueueFunc abstracts queue interactions to keep the job decoupled during tests.
type EnqueueFunc func(ctx context.Context, jobName string, payload map[string]any) (Message, error)

// NewSchedulerBootstrapJob loads cron entries from configuration into the scheduler.
func NewSchedulerBootstrapJob(cronEngine *cron.Cron, loader func() config.JobsConfig, enqueue EnqueueFunc) RegisteredJob {
	return RegisteredJob{
		Name:       "scheduler_bootstrap",
		MaxRetries: 1,
		Handler: func(ctx context.Context, message *Message) error {
			// 1.- Validate dependencies to avoid panics at runtime.
			if cronEngine == nil || enqueue == nil {
				return errors.New("scheduler dependencies missing")
			}
			if loader == nil {
				loader = config.LoadJobsConfig
			}
			// 2.- Load cron entries from configuration.
			cfg := loader()
			count := 0
			for _, entry := range cfg.CronEntries {
				entry := entry
				if entry.Spec == "" || entry.Job == "" {
					continue
				}
				// 3.- Register a closure that enqueues the configured job when triggered.
				_, err := cronEngine.AddFunc(entry.Spec, func() {
					_, _ = enqueue(context.Background(), entry.Job, entry.Payload)
				})
				if err != nil {
					return err
				}
				count++
			}
			// 4.- Record how many schedules were installed for observability.
			message.Metadata = map[string]interface{}{"scheduled": count}
			return nil
		},
	}
}
