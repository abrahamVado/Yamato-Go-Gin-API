package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- stubAuthenticator allows tests to control authentication outcomes without touching Redis or JWTs.
type stubAuthenticator struct {
	principal internalauth.Principal
	err       error
	calls     int
	mu        sync.Mutex
}

// 1.- Authenticate records invocations and either returns the configured principal or error.
func (s *stubAuthenticator) Authenticate(ctx context.Context, token string) (internalauth.Principal, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.err != nil {
		return internalauth.Principal{}, s.err
	}
	return s.principal, nil
}

// 1.- callCount reports how many times the authenticator was invoked.
func (s *stubAuthenticator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// 1.- memoryBroker fans out published events to in-memory subscribers for deterministic testing.
type memoryBroker struct {
	mu          sync.RWMutex
	subscribers map[string]map[*memorySubscription]struct{}
}

// 1.- newMemoryBroker constructs the broker with initialized maps.
func newMemoryBroker() *memoryBroker {
	return &memoryBroker{subscribers: make(map[string]map[*memorySubscription]struct{})}
}

// 1.- Subscribe registers the subscriber for all requested channels and observes context cancellation.
func (b *memoryBroker) Subscribe(ctx context.Context, channels ...string) (Subscription, error) {
	sub := &memorySubscription{broker: b, channels: append([]string(nil), channels...), messages: make(chan Message, 4)}

	b.mu.Lock()
	for _, ch := range channels {
		if _, ok := b.subscribers[ch]; !ok {
			b.subscribers[ch] = make(map[*memorySubscription]struct{})
		}
		b.subscribers[ch][sub] = struct{}{}
	}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		sub.Close()
	}()

	return sub, nil
}

// 1.- publish delivers a payload to all subscribers of the provided channel.
func (b *memoryBroker) publish(channel string, payload []byte) {
	b.mu.RLock()
	subs := b.subscribers[channel]
	for sub := range subs {
		select {
		case sub.messages <- Message{Channel: channel, Payload: payload}:
		default:
		}
	}
	b.mu.RUnlock()
}

type memorySubscription struct {
	broker   *memoryBroker
	channels []string
	messages chan Message
	once     sync.Once
}

// 1.- Messages exposes the buffered channel of broadcast events.
func (s *memorySubscription) Messages() <-chan Message {
	return s.messages
}

// 1.- Close unregisters the subscription and closes the backing channel exactly once.
func (s *memorySubscription) Close() error {
	s.once.Do(func() {
		s.broker.mu.Lock()
		for _, ch := range s.channels {
			subs := s.broker.subscribers[ch]
			delete(subs, s)
			if len(subs) == 0 {
				delete(s.broker.subscribers, ch)
			}
		}
		s.broker.mu.Unlock()
		close(s.messages)
	})
	return nil
}

func TestHandleRejectsMissingBearerToken(t *testing.T) {
	broker := newMemoryBroker()
	auth := &stubAuthenticator{}

	server, err := NewServer(auth, broker)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(server.Handle))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1)

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected handshake to fail without bearer token")
	}

	if resp == nil {
		t.Fatalf("expected HTTP response on handshake failure")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", resp.StatusCode)
	}
	if auth.callCount() != 0 {
		t.Fatalf("expected authenticator to remain unused, got %d calls", auth.callCount())
	}
}

func TestHandleRejectsInvalidToken(t *testing.T) {
	broker := newMemoryBroker()
	auth := &stubAuthenticator{err: errors.New("invalid token")}

	server, err := NewServer(auth, broker)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(server.Handle))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1)

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "bearer,bad-token")

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}

	conn, resp, err := dialer.Dial(wsURL, header)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected handshake to fail with invalid token")
	}

	if resp == nil {
		t.Fatalf("expected HTTP response on handshake failure")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", resp.StatusCode)
	}
	if auth.callCount() != 1 {
		t.Fatalf("expected authenticator call count 1, got %d", auth.callCount())
	}
}

func TestHandleFanoutToAuthorizedChannels(t *testing.T) {
	broker := newMemoryBroker()
	auth := &stubAuthenticator{principal: internalauth.Principal{Subject: "user-1"}}

	server, err := NewServer(auth, broker)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(server.Handle))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1)

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "bearer,valid-token")

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}

	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 response, got %+v", resp)
	}
	defer conn.Close()

	time.Sleep(25 * time.Millisecond)

	broker.publish("notifications:user-1", []byte(`{"message":"hello"}`))
	broker.publish("admin:events", []byte(`{"message":"admin"}`))

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}

	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	if envelope.Channel != "notifications:user-1" {
		t.Fatalf("expected notifications channel, got %s", envelope.Channel)
	}
	if envelope.Payload != `{"message":"hello"}` {
		t.Fatalf("unexpected payload: %s", envelope.Payload)
	}

	if err := conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set short read deadline: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return
		}
		t.Fatalf("expected timeout waiting for admin broadcast, got %v", err)
	} else {
		t.Fatalf("expected to time out waiting for admin broadcast")
	}
}

func TestHandleIncludesAdminEventsForAdminPrincipal(t *testing.T) {
	broker := newMemoryBroker()
	auth := &stubAuthenticator{principal: internalauth.Principal{Subject: "admin-1", Roles: []string{"admin"}}}

	server, err := NewServer(auth, broker)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(server.Handle))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1)

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "bearer,valid-token")

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}

	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 response, got %+v", resp)
	}
	defer conn.Close()

	time.Sleep(25 * time.Millisecond)

	broker.publish("notifications:admin-1", []byte(`{"message":"hello"}`))
	broker.publish("admin:events", []byte(`{"message":"admin"}`))

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}
	_, first, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set second read deadline: %v", err)
	}
	_, second, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("second Read returned error: %v", err)
	}

	var firstEnvelope EventEnvelope
	if err := json.Unmarshal(first, &firstEnvelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	var secondEnvelope EventEnvelope
	if err := json.Unmarshal(second, &secondEnvelope); err != nil {
		t.Fatalf("failed to unmarshal second envelope: %v", err)
	}

	received := map[string]bool{firstEnvelope.Channel: true, secondEnvelope.Channel: true}
	if !received["notifications:admin-1"] || !received["admin:events"] {
		t.Fatalf("expected to receive notifications and admin events, got %+v", received)
	}
}
