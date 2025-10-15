package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
)

// 1.- VerificationService offers a deterministic email verification workflow for development.
type VerificationService struct {
	users    authhttp.UserStore
	secret   string
	throttle time.Duration

	mu       sync.Mutex
	lastSend map[string]time.Time
}

// 1.- NewVerificationService constructs a verification helper using the supplied user store.
func NewVerificationService(users authhttp.UserStore, secret string, throttle time.Duration) *VerificationService {
	if strings.TrimSpace(secret) == "" {
		secret = "development-verification-secret"
	}
	if throttle <= 0 {
		throttle = 30 * time.Second
	}
	return &VerificationService{users: users, secret: secret, throttle: throttle, lastSend: map[string]time.Time{}}
}

// 1.- Verify ensures the provided hash matches the deterministic value for the user ID.
func (s *VerificationService) Verify(ctx context.Context, userID string, hash string) error {
	trimmedID := strings.TrimSpace(userID)
	if trimmedID == "" {
		return authhttp.ErrUserNotFound
	}
	if _, err := s.users.FindByID(ctx, trimmedID); err != nil {
		return err
	}
	expected := s.HashForUser(trimmedID)
	if subtleCompare(expected, strings.TrimSpace(hash)) {
		return nil
	}
	return authhttp.ErrInvalidVerification
}

// 1.- Resend enforces a basic throttle while signalling that a new notification would be dispatched.
func (s *VerificationService) Resend(ctx context.Context, userID string) error {
	trimmedID := strings.TrimSpace(userID)
	if trimmedID == "" {
		return authhttp.ErrUserNotFound
	}
	if _, err := s.users.FindByID(ctx, trimmedID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if last, ok := s.lastSend[trimmedID]; ok {
		if now.Sub(last) < s.throttle {
			return authhttp.ErrVerificationThrottled
		}
	}
	s.lastSend[trimmedID] = now
	return nil
}

// 1.- HashForUser exposes the deterministic verification hash for UI flows.
func (s *VerificationService) HashForUser(userID string) string {
	normalized := strings.TrimSpace(userID)
	sum := sha256.Sum256([]byte(normalized + "|" + s.secret))
	return hex.EncodeToString(sum[:])
}

// 1.- subtleCompare performs a constant-time comparison between two strings.
func subtleCompare(expected string, candidate string) bool {
	if len(expected) != len(candidate) {
		return false
	}
	var diff byte
	for i := range expected {
		diff |= expected[i] ^ candidate[i]
	}
	return diff == 0
}
