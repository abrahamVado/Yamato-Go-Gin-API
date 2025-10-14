package queue

import "context"

// FanoutNotifier defines the dependency used to broadcast notifications.
type FanoutNotifier interface {
	SendToUser(ctx context.Context, userID string, message string) error
}

// NewNotificationFanoutJob wires the fan-out handler into the queue registry.
func NewNotificationFanoutJob(notifier FanoutNotifier) RegisteredJob {
	return RegisteredJob{
		Name:       "notification_fanout",
		MaxRetries: 2,
		Handler: func(ctx context.Context, message *Message) error {
			// 1.- Pull fan-out parameters from the job payload.
			users, _ := message.Payload["user_ids"].([]any)
			text, _ := message.Payload["message"].(string)
			// 2.- Invoke the notifier for each targeted user sequentially.
			for _, userValue := range users {
				userID, _ := userValue.(string)
				if userID == "" {
					continue
				}
				if err := notifier.SendToUser(ctx, userID, text); err != nil {
					return err
				}
			}
			// 3.- Record diagnostic information for tests and observability.
			message.Metadata = map[string]interface{}{
				"delivered": len(users),
				"text":      text,
			}
			return nil
		},
	}
}
