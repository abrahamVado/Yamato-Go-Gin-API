package queue

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// WebhookDispatcher abstracts HTTP transport so jobs remain testable.
type WebhookDispatcher interface {
	Post(ctx context.Context, url string, headers map[string]string, body []byte) error
}

// NewWebhookDispatchJob creates a job that posts signed payloads to webhooks.
func NewWebhookDispatchJob(dispatcher WebhookDispatcher) RegisteredJob {
	return RegisteredJob{
		Name:       "webhook_dispatch",
		MaxRetries: 4,
		Handler: func(ctx context.Context, message *Message) error {
			// 1.- Parse the required fields from the payload map.
			url, _ := message.Payload["url"].(string)
			secret, _ := message.Payload["secret"].(string)
			bodyValue, hasBody := message.Payload["body"]
			if url == "" || secret == "" || !hasBody {
				return errors.New("invalid webhook payload")
			}
			// 2.- Normalize the body to a JSON byte slice when needed.
			var bodyBytes []byte
			switch body := bodyValue.(type) {
			case string:
				bodyBytes = []byte(body)
			default:
				encoded, err := json.Marshal(body)
				if err != nil {
					return err
				}
				bodyBytes = encoded
			}
			// 3.- Sign the body with HMAC-SHA256 to authenticate the request.
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			signature := hex.EncodeToString(mac.Sum(nil))
			headers := map[string]string{
				"Content-Type": "application/json",
				"X-Signature":  signature,
			}
			// 4.- Dispatch the webhook through the provided transport layer.
			if err := dispatcher.Post(ctx, url, headers, bodyBytes); err != nil {
				return err
			}
			// 5.- Attach metadata for auditability.
			message.Metadata = map[string]interface{}{
				"url":       url,
				"signature": signature,
			}
			return nil
		},
	}
}
