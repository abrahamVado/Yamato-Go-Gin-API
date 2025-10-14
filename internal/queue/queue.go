package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

// Message represents a serialized job payload.
type Message struct {
	ID         string                 `json:"id"`
	Job        string                 `json:"job"`
	Payload    map[string]any         `json:"payload"`
	Attempts   int                    `json:"attempts"`
	MaxRetries int                    `json:"max_retries"`
	EnqueuedAt time.Time              `json:"enqueued_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	LastError  string                 `json:"last_error,omitempty"`
}

// RedisQueue coordinates producers and consumers using Redis lists.
type RedisQueue struct {
	client   *redis.Client
	registry *jobRegistry
	queueKey string
	dlqKey   string
	waitTime time.Duration
}

// NewRedisQueue constructs the queue with sensible defaults.
func NewRedisQueue(client *redis.Client, namespace string) *RedisQueue {
	// 1.- Provide fallbacks for namespace and blocking duration.
	if namespace == "" {
		namespace = "jobs"
	}
	// 2.- Instantiate the queue with derived Redis keys.
	return &RedisQueue{
		client:   client,
		registry: newJobRegistry(),
		queueKey: fmt.Sprintf("%s:main", namespace),
		dlqKey:   fmt.Sprintf("%s:dlq", namespace),
		waitTime: 5 * time.Second,
	}
}

// Register exposes the job registration helper.
func (q *RedisQueue) Register(job RegisteredJob) error {
	// 1.- Delegate to the underlying registry to keep responsibilities narrow.
	return q.registry.register(job)
}

// Enqueue pushes a new job into Redis for later processing.
func (q *RedisQueue) Enqueue(ctx context.Context, jobName string, payload map[string]any) (Message, error) {
	// 1.- Validate the job exists before serializing the message.
	jobDef, ok := q.registry.get(jobName)
	if !ok {
		return Message{}, fmt.Errorf("job %s is not registered", jobName)
	}
	// 2.- Prepare the message envelope with tracking fields.
	message := Message{
		ID:         uuid.NewString(),
		Job:        jobName,
		Payload:    payload,
		MaxRetries: jobDef.MaxRetries,
		EnqueuedAt: time.Now().UTC(),
	}
	// 3.- Convert message to JSON for storage in Redis.
	encoded, err := json.Marshal(message)
	if err != nil {
		return Message{}, err
	}
	// 4.- Append the message to the queue list.
	if err := q.client.RPush(ctx, q.queueKey, encoded).Err(); err != nil {
		return Message{}, err
	}
	return message, nil
}

// StartConsumer begins processing jobs until the context is cancelled.
func (q *RedisQueue) StartConsumer(ctx context.Context) error {
	// 1.- Continuously pop jobs while respecting cancellation signals.
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// 2.- Block waiting for the next job from Redis.
		result, err := q.client.BLPop(ctx, q.waitTime, q.queueKey).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, redis.Nil) {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if len(result) < 2 {
			continue
		}
		// 3.- Decode the JSON message payload.
		var message Message
		if err := json.Unmarshal([]byte(result[1]), &message); err != nil {
			message.LastError = err.Error()
			_ = q.moveToDLQ(ctx, message)
			continue
		}
		// 4.- Lookup the job handler for execution.
		jobDef, ok := q.registry.get(message.Job)
		if !ok {
			message.LastError = "unregistered job"
			_ = q.moveToDLQ(ctx, message)
			continue
		}
		// 5.- Enforce per-job timeout around the handler execution.
		jobCtx, cancel := context.WithTimeout(ctx, jobDef.Timeout)
		err = jobDef.Handler(jobCtx, &message)
		cancel()
		if err != nil {
			// 6.- Retry failed jobs until the attempt budget is exhausted.
			message.Attempts++
			message.LastError = err.Error()
			if message.Attempts >= message.MaxRetries {
				_ = q.moveToDLQ(ctx, message)
				continue
			}
			if requeueErr := q.requeue(ctx, message); requeueErr != nil {
				message.LastError = fmt.Sprintf("requeue failed: %v", requeueErr)
				_ = q.moveToDLQ(ctx, message)
			}
			continue
		}
		// 7.- Successful executions require no further action.
	}
}

// requeue pushes a previously dequeued message back for another attempt.
func (q *RedisQueue) requeue(ctx context.Context, message Message) error {
	// 1.- Re-marshal the message with updated attempt counters.
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	// 2.- Insert the message at the tail of the list for retry handling.
	return q.client.RPush(ctx, q.queueKey, encoded).Err()
}

// moveToDLQ persists the message in a dead-letter list for later inspection.
func (q *RedisQueue) moveToDLQ(ctx context.Context, message Message) error {
	// 1.- Preserve diagnostic context for operators by encoding as JSON.
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	// 2.- Store the message in the DLQ to prevent infinite retry loops.
	return q.client.RPush(ctx, q.dlqKey, encoded).Err()
}

// ReadDLQ returns up to limit messages from the DLQ without removing them.
func (q *RedisQueue) ReadDLQ(ctx context.Context, limit int64) ([]Message, error) {
	// 1.- Pull the requested range from Redis while bounding the slice.
	raw, err := q.client.LRange(ctx, q.dlqKey, 0, limit-1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	// 2.- Decode each JSON entry back into a Message value.
	messages := make([]Message, 0, len(raw))
	for _, item := range raw {
		var msg Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// Jobs exposes the registered jobs which helps the worker bootstrap runtime features.
func (q *RedisQueue) Jobs() []RegisteredJob {
	// 1.- Delegate to the registry to maintain a single source of truth.
	return q.registry.list()
}
