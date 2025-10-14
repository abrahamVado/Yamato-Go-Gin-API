package websocket

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// 1.- RedisBroker bridges Redis Pub/Sub streams with the WebSocket subscription abstraction.
type RedisBroker struct {
	client *redis.Client
}

// 1.- NewRedisBroker constructs a Redis-backed broker using the supplied client instance.
func NewRedisBroker(client *redis.Client) (*RedisBroker, error) {
	if client == nil {
		return nil, fmt.Errorf("websocket: redis client must not be nil")
	}
	return &RedisBroker{client: client}, nil
}

// 1.- Subscribe wires Redis Pub/Sub messages into the Server's subscription contract.
func (b *RedisBroker) Subscribe(ctx context.Context, channels ...string) (Subscription, error) {
	pubsub := b.client.Subscribe(ctx, channels...)
	if _, err := pubsub.Receive(ctx); err != nil {
		pubsub.Close()
		return nil, err
	}
	sub := &redisSubscription{pubsub: pubsub, messages: make(chan Message, 16)}
	go sub.consume(ctx)
	return sub, nil
}

type redisSubscription struct {
	pubsub   *redis.PubSub
	messages chan Message
	once     sync.Once
}

// 1.- Messages exposes the downstream channel of brokered events.
func (s *redisSubscription) Messages() <-chan Message {
	return s.messages
}

// 1.- Close terminates the Redis subscription and stops the relay goroutine.
func (s *redisSubscription) Close() error {
	s.once.Do(func() {
		s.pubsub.Close()
		close(s.messages)
	})
	return nil
}

func (s *redisSubscription) consume(ctx context.Context) {
	defer s.Close()
	for {
		msg, err := s.pubsub.ReceiveMessage(ctx)
		if err != nil {
			return
		}

		select {
		case s.messages <- Message{Channel: msg.Channel, Payload: []byte(msg.Payload)}:
		default:
		}
	}
}
