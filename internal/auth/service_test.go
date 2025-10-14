package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/example/Yamato-Go-Gin-API/internal/config"
)

func newTestService(t *testing.T) (*Service, *redis.Client) {
	t.Helper()

	//1.- Launch a miniredis server for deterministic unit testing.
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	cfg := config.JWTConfig{
		Secret:            "unit-test-secret",
		Issuer:            "unit-test-issuer",
		Audience:          "unit-test-audience",
		AccessExpiration:  2 * time.Minute,
		RefreshExpiration: 10 * time.Minute,
	}

	svc, err := NewService(cfg, client)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	return svc, client
}

func TestLoginIssuesTokens(t *testing.T) {
	//1.- Prepare the service under test and request initial credentials.
	ctx := context.Background()
	svc, client := newTestService(t)

	pair, err := svc.Login(ctx, "user-123")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens: %#v", pair)
	}

	claims, err := svc.ValidateAccessToken(ctx, pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken returned error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}

	stored, err := client.Get(ctx, refreshFamilyKey(claims.FamilyID)).Result()
	if err != nil {
		t.Fatalf("expected refresh family entry: %v", err)
	}
	if stored == "" {
		t.Fatalf("refresh family value should not be empty")
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	//1.- Issue a baseline token pair to exercise rotation logic.
	ctx := context.Background()
	svc, _ := newTestService(t)

	initial, err := svc.Login(ctx, "user-456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	rotated, err := svc.Refresh(ctx, initial.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if rotated.RefreshToken == initial.RefreshToken {
		t.Fatalf("expected rotated refresh token to differ")
	}

	if _, err := svc.Refresh(ctx, initial.RefreshToken); !errors.Is(err, ErrReuseDetected) {
		t.Fatalf("expected reuse detection error, got %v", err)
	}

	if _, err := svc.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, ErrReuseDetected) {
		t.Fatalf("expected family to remain blacklisted, got %v", err)
	}
}

func TestLogoutBlacklistsTokens(t *testing.T) {
	//1.- Acquire tokens and then invoke logout for revocation coverage.
	ctx := context.Background()
	svc, _ := newTestService(t)

	pair, err := svc.Login(ctx, "user-789")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	if err := svc.Logout(ctx, pair.RefreshToken, pair.AccessToken); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}

	if _, err := svc.ValidateAccessToken(ctx, pair.AccessToken); !errors.Is(err, ErrBlacklisted) {
		t.Fatalf("expected access token to be blacklisted, got %v", err)
	}

	if _, err := svc.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ErrReuseDetected) {
		t.Fatalf("expected family blacklist after logout, got %v", err)
	}
}
