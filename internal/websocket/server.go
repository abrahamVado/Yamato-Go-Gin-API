package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- ErrUnauthorized indicates that the WebSocket handshake could not authenticate the client.
var ErrUnauthorized = errors.New("websocket: unauthorized")

// 1.- Authenticator resolves a principal from a bearer token supplied during the handshake.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (internalauth.Principal, error)
}

// 1.- Broker exposes publish/subscribe capabilities for forwarding domain events to clients.
type Broker interface {
	Subscribe(ctx context.Context, channels ...string) (Subscription, error)
}

// 1.- Subscription provides a stream of messages originating from the brokered channels.
type Subscription interface {
	Messages() <-chan Message
	Close() error
}

// 1.- Message captures the channel name alongside the delivered payload bytes.
type Message struct {
	Channel string
	Payload []byte
}

// 1.- EventEnvelope is serialized to the WebSocket connection for each brokered event.
type EventEnvelope struct {
	Channel string `json:"channel"`
	Payload string `json:"payload"`
}

// 1.- Server validates WebSocket connections and relays broker messages to authenticated clients.
type Server struct {
	auth         Authenticator
	broker       Broker
	writeTimeout time.Duration
}

// 1.- NewServer constructs a Server with sane defaults for write deadlines and dependency checks.
func NewServer(auth Authenticator, broker Broker) (*Server, error) {
	if auth == nil {
		return nil, fmt.Errorf("websocket: authenticator must not be nil")
	}
	if broker == nil {
		return nil, fmt.Errorf("websocket: broker must not be nil")
	}

	return &Server{auth: auth, broker: broker, writeTimeout: 5 * time.Second}, nil
}

// 1.- Handle upgrades HTTP requests to WebSocket connections once authentication succeeds.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	// 2.- Extract and validate the bearer token from the Sec-WebSocket-Protocol header.
	token, err := extractBearerToken(r.Header.Values("Sec-WebSocket-Protocol"))
	if err != nil {
		http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	// 3.- Resolve the authenticated principal using the configured authenticator.
	principal, err := s.auth.Authenticate(r.Context(), token)
	if err != nil {
		http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	// 4.- Accept the WebSocket handshake while echoing the bearer subprotocol for compliance.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"bearer"}})
	if err != nil {
		http.Error(w, "failed to upgrade connection", http.StatusBadRequest)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing connection")

	// 5.- Build the list of broker channels for the authenticated principal.
	channels := []string{fmt.Sprintf("notifications:%s", principal.Subject)}
	if principal.HasRole("admin") {
		channels = append(channels, "admin:events")
	}

	// 6.- Subscribe to the selected channels via the configured broker implementation.
	sub, err := s.broker.Subscribe(r.Context(), channels...)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "subscription failed")
		return
	}
	defer sub.Close()

	// 7.- Spawn a goroutine to monitor client initiated closes to terminate gracefully.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		for {
			if _, _, readErr := conn.Read(ctx); readErr != nil {
				cancel()
				return
			}
		}
	}()

	// 8.- Forward brokered messages to the client until the context signals termination.
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.Messages():
			if !ok {
				return
			}

			envelope := EventEnvelope{Channel: msg.Channel, Payload: string(msg.Payload)}
			payload, marshalErr := json.Marshal(envelope)
			if marshalErr != nil {
				conn.Close(websocket.StatusInternalError, "marshal error")
				return
			}

			writeCtx, writeCancel := context.WithTimeout(ctx, s.writeTimeout)
			if writeErr := conn.Write(writeCtx, websocket.MessageText, payload); writeErr != nil {
				writeCancel()
				cancel()
				return
			}
			writeCancel()
		}
	}
}

// 1.- extractBearerToken scans the Sec-WebSocket-Protocol entries for a bearer token pair.
func extractBearerToken(protocols []string) (string, error) {
	// 2.- Iterate over every header value to normalize and inspect the advertised subprotocols.
	for _, headerValue := range protocols {
		entries := strings.Split(headerValue, ",")
		for idx := 0; idx < len(entries); idx++ {
			entry := strings.TrimSpace(entries[idx])
			if !strings.EqualFold(entry, "bearer") {
				continue
			}

			if idx+1 >= len(entries) {
				return "", ErrUnauthorized
			}

			token := strings.TrimSpace(entries[idx+1])
			if token == "" {
				return "", ErrUnauthorized
			}
			return token, nil
		}
	}
	return "", ErrUnauthorized
}
