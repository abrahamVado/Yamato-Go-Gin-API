package queue

import (
	"context"
	"errors"
	"time"
)

// EmailSender abstracts the delivery mechanism for email notifications.
type EmailSender interface {
	Send(ctx context.Context, to string, subject string, body string) error
}

// NewEmailSendJob registers a retry-aware email sending job.
func NewEmailSendJob(sender EmailSender) RegisteredJob {
	return RegisteredJob{
		Name:       "email_send",
		MaxRetries: 5,
		Timeout:    45 * time.Second,
		Handler: func(ctx context.Context, message *Message) error {
			// 1.- Extract payload fields with strong validation.
			to, _ := message.Payload["to"].(string)
			subject, _ := message.Payload["subject"].(string)
			body, _ := message.Payload["body"].(string)
			if to == "" || subject == "" {
				return errors.New("missing email fields")
			}
			// 2.- Forward the email to the sender dependency.
			if err := sender.Send(ctx, to, subject, body); err != nil {
				return err
			}
			// 3.- Record metadata for traceability across retries.
			message.Metadata = map[string]interface{}{
				"to":      to,
				"subject": subject,
				"status":  "sent",
			}
			return nil
		},
	}
}
