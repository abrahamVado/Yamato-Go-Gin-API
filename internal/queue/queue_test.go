package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/example/Yamato-Go-Gin-API/internal/queue"
)

// setupRedis starts an in-memory Redis instance that mimics production behavior.
func setupRedis(t *testing.T) (*redis.Client, func()) {
	// 1.- Boot a miniredis server which provides a lightweight Redis substitute.
	srv, err := miniredis.Run()
	require.NoError(t, err)

	// 2.- Connect a go-redis client to the miniredis server for exercising the queue.
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	// 3.- Return cleanup that tears down the client and in-memory server.
	cleanup := func() {
		_ = client.Close()
		srv.Close()
	}
	return client, cleanup
}

func TestRedisQueueProcessesFanout(t *testing.T) {
	client, cleanup := setupRedis(t)
	defer cleanup()

	q := queue.NewRedisQueue(client, "itest")
	notifier := &mockNotifier{}
	require.NoError(t, q.Register(queue.NewNotificationFanoutJob(notifier)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = q.StartConsumer(ctx) }()

	_, err := q.Enqueue(ctx, "notification_fanout", map[string]any{
		"user_ids": []string{"alice", "bob"},
		"message":  "welcome",
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return notifier.Count() == 2
	}, 10*time.Second, 50*time.Millisecond)
}

func TestRedisQueueRetriesOnFailure(t *testing.T) {
	client, cleanup := setupRedis(t)
	defer cleanup()

	q := queue.NewRedisQueue(client, "itest")
	sender := &flakyEmailSender{failures: 1}
	require.NoError(t, q.Register(queue.NewEmailSendJob(sender)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = q.StartConsumer(ctx) }()

	_, err := q.Enqueue(ctx, "email_send", map[string]any{
		"to":      "user@example.com",
		"subject": "retry",
		"body":    "hello",
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		successes, attempts := sender.Metrics()
		return successes == 1 && attempts >= 2
	}, 10*time.Second, 50*time.Millisecond)
}

func TestRedisQueueMovesToDLQ(t *testing.T) {
	client, cleanup := setupRedis(t)
	defer cleanup()

	q := queue.NewRedisQueue(client, "itest")
	job := queue.RegisteredJob{
		Name:       "always_fail",
		MaxRetries: 2,
		Handler: func(ctx context.Context, message *queue.Message) error {
			return assertError
		},
	}
	require.NoError(t, q.Register(job))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = q.StartConsumer(ctx) }()

	_, err := q.Enqueue(ctx, "always_fail", map[string]any{"value": "test"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		messages, err := q.ReadDLQ(context.Background(), 10)
		require.NoError(t, err)
		if len(messages) == 0 {
			return false
		}
		return messages[0].LastError != ""
	}, 10*time.Second, 50*time.Millisecond)
}

type mockNotifier struct {
	mu    sync.Mutex
	users []string
	count int
}

func (m *mockNotifier) SendToUser(ctx context.Context, userID string, message string) error {
	// 1.- Record each delivered user for later assertions.
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = append(m.users, userID)
	m.count++
	return nil
}

func (m *mockNotifier) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

type flakyEmailSender struct {
	mu        sync.Mutex
	failures  int
	attempts  int
	successes int
}

func (f *flakyEmailSender) Send(ctx context.Context, to string, subject string, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 1.- Increment attempts so tests can assert retry behavior.
	f.attempts++
	if f.failures > 0 {
		f.failures--
		return assertError
	}
	f.successes++
	return nil
}

func (f *flakyEmailSender) Metrics() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.successes, f.attempts
}

var assertError = queueErr("intentional failure")

type queueErr string

func (e queueErr) Error() string { return string(e) }
