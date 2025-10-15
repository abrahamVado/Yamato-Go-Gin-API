package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
)

// 1.- stubAuthService implements authhttp.AuthService while recording invocations for assertions.
type stubAuthService struct {
	// 2.- loginSubjects stores subjects passed to Login for later verification.
	loginSubjects []string
	// 3.- refreshTokens stores refresh tokens observed by Refresh.
	refreshTokens []string
	// 4.- logoutPairs tracks the token combinations received by Logout.
	logoutPairs [][2]string
}

// 1.- HashPassword returns a deterministic hash string for testing.
func (s *stubAuthService) HashPassword(password string) (string, error) {
	return "hashed:" + password, nil
}

// 1.- CheckPassword accepts any password during tests to avoid crypto dependencies.
func (s *stubAuthService) CheckPassword(hash string, password string) error {
	return nil
}

// 1.- Login records the subject and returns a predictable token pair.
func (s *stubAuthService) Login(_ context.Context, subject string) (internalauth.TokenPair, error) {
	s.loginSubjects = append(s.loginSubjects, subject)
	now := time.Unix(0, 0).UTC()
	return internalauth.TokenPair{
		AccessToken:      "access-" + subject,
		RefreshToken:     "refresh-" + subject,
		AccessExpiresAt:  now.Add(time.Hour),
		RefreshExpiresAt: now.Add(24 * time.Hour),
	}, nil
}

// 1.- Refresh records the supplied token and returns a rotated pair.
func (s *stubAuthService) Refresh(_ context.Context, refreshToken string) (internalauth.TokenPair, error) {
	s.refreshTokens = append(s.refreshTokens, refreshToken)
	now := time.Unix(0, 0).UTC()
	return internalauth.TokenPair{
		AccessToken:      "access-rotated",
		RefreshToken:     "refresh-rotated",
		AccessExpiresAt:  now.Add(time.Hour),
		RefreshExpiresAt: now.Add(24 * time.Hour),
	}, nil
}

// 1.- Logout records the refresh/access combination received during the call.
func (s *stubAuthService) Logout(_ context.Context, refreshToken string, accessToken string) error {
	s.logoutPairs = append(s.logoutPairs, [2]string{refreshToken, accessToken})
	return nil
}

// 1.- stubUserStore implements authhttp.UserStore backed by an in-memory map.
type stubUserStore struct {
	// 2.- users indexes stored users by e-mail for quick lookups.
	users map[string]authhttp.User
}

// 1.- stubVerificationService captures verification and resend invocations.
type stubVerificationService struct {
	verifyCalls [][2]string
	resendCalls []string
}

// 1.- Verify records the provided identifiers for later assertions.
func (s *stubVerificationService) Verify(_ context.Context, userID string, hash string) error {
	s.verifyCalls = append(s.verifyCalls, [2]string{userID, hash})
	return nil
}

// 1.- Resend records the subject triggering the resend flow.
func (s *stubVerificationService) Resend(_ context.Context, userID string) error {
	s.resendCalls = append(s.resendCalls, userID)
	return nil
}

// 1.- HashForUser returns a deterministic hash to emulate verification tokens.
func (s *stubVerificationService) HashForUser(userID string) string {
	return "hash-" + userID
}

// 1.- newStubUserStore prepares an empty store ready for use in tests.
func newStubUserStore() *stubUserStore {
	return &stubUserStore{users: map[string]authhttp.User{}}
}

// 1.- Create persists the provided user and returns it unchanged.
func (s *stubUserStore) Create(_ context.Context, user authhttp.User) (authhttp.User, error) {
	s.users[user.Email] = user
	return user, nil
}

// 1.- FindByEmail retrieves a stored user or returns the sentinel error.
func (s *stubUserStore) FindByEmail(_ context.Context, email string) (authhttp.User, error) {
	if user, ok := s.users[email]; ok {
		return user, nil
	}
	return authhttp.User{}, authhttp.ErrUserNotFound
}

// 1.- FindByID returns the user matching the supplied identifier.
func (s *stubUserStore) FindByID(_ context.Context, id string) (authhttp.User, error) {
	for _, user := range s.users {
		if user.ID == id {
			return user, nil
		}
	}
	return authhttp.User{}, authhttp.ErrUserNotFound
}

// 1.- TestRegisterAuthRoutesWiresEndpoints verifies that the router exposes the expected auth endpoints.
func TestRegisterAuthRoutesWiresEndpoints(t *testing.T) {
	// 2.- Configure Gin for deterministic testing and prepare dependencies.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := newStubUserStore()
	store.users["seed@example.com"] = authhttp.User{ID: "user-123", Email: "seed@example.com"}
	authSvc := &stubAuthService{}
	verificationSvc := &stubVerificationService{}
	handler := authhttp.NewHandler(authSvc, store, verificationSvc)

	// 3.- Configure the authentication middleware to populate the principal context.
	authMiddleware := func(ctx *gin.Context) {
		internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: "user-123", Roles: []string{"member"}, Permissions: []string{"read"}})
		ctx.Next()
	}

	// 4.- Register the routes under test using the helper.
	RegisterAuthRoutes(router, handler, authMiddleware)

	// 5.- Exercise the registration endpoint and ensure persistence was invoked.
	registerBody, _ := json.Marshal(map[string]string{"email": "new@example.com", "password": "secretpass"})
	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("expected register to return %d, got %d", http.StatusCreated, registerRec.Code)
	}
	if _, exists := store.users["new@example.com"]; !exists {
		t.Fatalf("expected user to be stored after registration")
	}

	// 6.- Authenticate using the login endpoint and ensure token issuance is attempted.
	loginBody, _ := json.Marshal(map[string]string{"email": "new@example.com", "password": "secretpass"})
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login to return %d, got %d", http.StatusOK, loginRec.Code)
	}
	if len(authSvc.loginSubjects) == 0 {
		t.Fatalf("expected login to invoke auth service")
	}

	// 7.- Rotate tokens using the refresh endpoint and check that the token was recorded.
	refreshBody, _ := json.Marshal(map[string]string{"refresh_token": "refresh-token"})
	refreshReq := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected refresh to return %d, got %d", http.StatusOK, refreshRec.Code)
	}
	if len(authSvc.refreshTokens) == 0 {
		t.Fatalf("expected refresh token to be recorded")
	}

	// 8.- Revoke tokens through the logout endpoint and ensure the payload was captured.
	logoutBody, _ := json.Marshal(map[string]string{"refresh_token": "refresh-token", "access_token": "access-token"})
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", bytes.NewReader(logoutBody))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected logout to return %d, got %d", http.StatusOK, logoutRec.Code)
	}
	if len(authSvc.logoutPairs) == 0 {
		t.Fatalf("expected logout tokens to be tracked")
	}

	// 9.- Fetch the current principal via the user endpoint to confirm the route exists.
	userReq := httptest.NewRequest(http.MethodGet, "/v1/user", nil)
	userRec := httptest.NewRecorder()
	router.ServeHTTP(userRec, userReq)
	if userRec.Code != http.StatusOK {
		t.Fatalf("expected user endpoint to return %d, got %d", http.StatusOK, userRec.Code)
	}

	// 10.- Exercise the verification endpoint to ensure compatibility routes are mounted.
	verifyReq := httptest.NewRequest(http.MethodGet, "/email/verify/user-123/hash-token", nil)
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected verification endpoint to return %d, got %d", http.StatusOK, verifyRec.Code)
	}
	if len(verificationSvc.verifyCalls) != 1 {
		t.Fatalf("expected verification service to record a single call")
	}

	// 11.- Trigger the resend endpoint and ensure the service receives the subject.
	resendReq := httptest.NewRequest(http.MethodPost, "/email/verification-notification", nil)
	resendRec := httptest.NewRecorder()
	router.ServeHTTP(resendRec, resendReq)
	if resendRec.Code != http.StatusAccepted {
		t.Fatalf("expected resend endpoint to return %d, got %d", http.StatusAccepted, resendRec.Code)
	}
	if len(verificationSvc.resendCalls) != 1 || verificationSvc.resendCalls[0] != "user-123" {
		t.Fatalf("expected resend to capture the authenticated subject")
	}
}
