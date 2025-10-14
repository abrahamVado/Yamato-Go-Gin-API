package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/example/Yamato-Go-Gin-API/internal/queue"
)

// stdoutNotifier is a demo implementation that writes fan-out results to stdout.
type stdoutNotifier struct{}

func (stdoutNotifier) SendToUser(ctx context.Context, userID string, message string) error {
	fmt.Printf("fanout -> user=%s message=%s\n", userID, message)
	return nil
}

// stdoutEmailSender prints outgoing emails instead of sending them.
type stdoutEmailSender struct{}

func (stdoutEmailSender) Send(ctx context.Context, to string, subject string, body string) error {
	fmt.Printf("email -> to=%s subject=%s\n", to, subject)
	return nil
}

// stdoutWebhookDispatcher mimics HTTP dispatch for local testing.
type stdoutWebhookDispatcher struct{}

func (stdoutWebhookDispatcher) Post(ctx context.Context, url string, headers map[string]string, body []byte) error {
	fmt.Printf("webhook -> url=%s headers=%v body=%s\n", url, headers, string(body))
	return nil
}

func main() {
	// 1.- Prepare cancellation context reacting to OS signals.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// 2.- Connect to Redis using environment-provided configuration.
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer client.Close()

	// 3.- Build the queue and register all job handlers.
	q := queue.NewRedisQueue(client, "jobs")
	cronEngine := cron.New()
	_ = q.Register(queue.NewNotificationFanoutJob(stdoutNotifier{}))
	_ = q.Register(queue.NewEmailSendJob(stdoutEmailSender{}))
	_ = q.Register(queue.NewWebhookDispatchJob(stdoutWebhookDispatcher{}))
	_ = q.Register(queue.NewSchedulerBootstrapJob(cronEngine, nil, q.Enqueue))

	// 4.- Enqueue the scheduler bootstrapper so cron entries are loaded.
	if _, err := q.Enqueue(ctx, "scheduler_bootstrap", map[string]any{}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to enqueue scheduler bootstrap: %v\n", err)
	}

	// 5.- Start the cron engine and queue consumer concurrently.
	cronEngine.Start()
	go func() {
		if err := q.StartConsumer(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "worker consumer error: %v\n", err)
			cancel()
		}
	}()

	// 6.- Keep the process alive until cancellation is requested.
	<-ctx.Done()
	cronEngine.Stop()
	time.Sleep(500 * time.Millisecond)
}
