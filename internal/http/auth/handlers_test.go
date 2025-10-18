package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/config"
	authpkg "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/middleware"
)

// 1.- memoryUserStore stores users in memory for deterministic handler tests.
type memoryUserStore struct {
	users   map[string]authpkg.User
	byEmail map[string]string
}

// 1.- stubVerificationService records verification and resend calls for assertions.
type stubVerificationService struct {
	verifyCalls [][2]string
	resendCalls []string
	verifyErr   error
	resendErr   error
}

// 1.- Verify records the call and returns the configured error.
func (s *stubVerificationService) Verify(_ context.Context, userID string, hash string) error {
	s.verifyCalls = append(s.verifyCalls, [2]string{userID, hash})
	return s.verifyErr
}

// 1.- Resend records the subject and returns the configured error.
func (s *stubVerificationService) Resend(_ context.Context, userID string) error {
	s.resendCalls = append(s.resendCalls, userID)
	return s.resendErr
}

// 1.- HashForUser returns a deterministic hash for assertions.
func (s *stubVerificationService) HashForUser(userID string) string {
	return "hash-" + userID
}

// 1.- newMemoryUserStore constructs a thread-safe in-memory repository.
func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{users: map[string]authpkg.User{}, byEmail: map[string]string{}}
}

// 1.- Create inserts a new user into the in-memory maps.
func (m *memoryUserStore) Create(_ context.Context, user authpkg.User) (authpkg.User, error) {
	if _, exists := m.byEmail[user.Email]; exists {
		return authpkg.User{}, errors.New("duplicate email")
	}
	copied := user
	m.users[user.ID] = copied
	m.byEmail[user.Email] = user.ID
	return copied, nil
}

// 1.- FindByEmail returns the stored user or authpkg.ErrUserNotFound.
func (m *memoryUserStore) FindByEmail(_ context.Context, email string) (authpkg.User, error) {
	id, ok := m.byEmail[email]
	if !ok {
		return authpkg.User{}, authpkg.ErrUserNotFound
	}
	return m.users[id], nil
}

// 1.- FindByID returns the stored user using the identifier index.
func (m *memoryUserStore) FindByID(_ context.Context, id string) (authpkg.User, error) {
	user, ok := m.users[id]
	if !ok {
		return authpkg.User{}, authpkg.ErrUserNotFound
	}
	return user, nil
}

// 1.- setupHandler constructs a handler with real token service dependencies.
func setupHandler(t *testing.T) (authpkg.Handler, *memoryUserStore, *stubVerificationService, func()) {
	gin.SetMode(gin.TestMode)

	// 2.- Boot an in-memory Redis instance for token workflows.
	mini, err := miniredis.Run()
	require.NoError(t, err)

	// 3.- Create a Redis client configured to talk to the miniredis server.
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})

	// 4.- Configure short-lived tokens to keep tests fast and deterministic.
	svc, err := internalauth.NewService(config.JWTConfig{
		Secret:            "test-secret",
		Issuer:            "yamato-test",
		AccessExpiration:  time.Minute,
		RefreshExpiration: time.Hour,
	}, client)
	require.NoError(t, err)

	store := newMemoryUserStore()
	verifier := &stubVerificationService{}
	handler := authpkg.NewHandler(svc, store, verifier)
	cleanup := func() {
		_ = client.Close()
		mini.Close()
	}

	return handler, store, verifier, cleanup
}

// 1.- successPayload generically decodes success envelopes in tests.
type successPayload[T any] struct {
	Status string         `json:"status"`
	Data   T              `json:"data"`
	Meta   map[string]any `json:"meta"`
}

// 1.- errorPayload captures the standard error envelope emitted by handlers.
type errorPayload struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- tokenPayload mirrors the JSON structure of token responses for assertions.
type tokenPayload struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// 1.- registerPayload captures the register endpoint response schema used in tests.
type registerPayload struct {
	User             authpkg.User `json:"user"`
	Tokens           tokenPayload `json:"tokens"`
	Notice           string       `json:"notice"`
	VerificationHash string       `json:"verification_hash"`
}

// 1.- loginPayload captures the login response schema used in tests.
type loginPayload struct {
	User   authpkg.User `json:"user"`
	Tokens tokenPayload `json:"tokens"`
}

// 1.- principalPayload mirrors the JSON envelope returned by the current user endpoint.
type principalPayload struct {
	Subject     string   `json:"subject"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// 1.- newTestEngine returns a Gin engine configured with the shared error middleware.
func newTestEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	return engine
}

// 1.- performRequest issues an HTTP request against the supplied engine and returns the recorder.
func performRequest(engine *gin.Engine, method string, path string, body string, contentType string) *httptest.ResponseRecorder {
	reader := strings.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

// 1.- TestRegisterLoginRefreshLogoutFlow covers the complete authentication lifecycle.
func TestRegisterLoginRefreshLogoutFlow(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/v1/auth/register", handler.Register)
	engine.POST("/v1/auth/login", handler.Login)
	engine.POST("/v1/auth/refresh", handler.Refresh)
	engine.POST("/v1/auth/logout", handler.Logout)

	// 2.- Exercise the registration endpoint.
	registerRecorder := performRequest(engine, http.MethodPost, "/v1/auth/register", `{"email":"user@example.com","password":"secret"}`, "application/json")
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registerBody successPayload[registerPayload]
	require.NoError(t, json.Unmarshal(registerRecorder.Body.Bytes(), &registerBody))
	require.Equal(t, "success", registerBody.Status)
	require.NotNil(t, registerBody.Meta)
	require.NotEmpty(t, registerBody.Data.User.ID)
	require.Equal(t, "user@example.com", registerBody.Data.User.Email)
	require.NotEmpty(t, registerBody.Data.Tokens.AccessToken)
	require.NotEmpty(t, registerBody.Data.Tokens.RefreshToken)
	require.NotEmpty(t, registerBody.Data.User.Name)
	require.NotEmpty(t, registerBody.Data.Notice)
	require.Equal(t, "hash-"+registerBody.Data.User.ID, registerBody.Data.VerificationHash)

	// 3.- Attempt a login with the same credentials; tokens must rotate.
	loginRecorder := performRequest(engine, http.MethodPost, "/v1/auth/login", `{"email":"user@example.com","password":"secret"}`, "application/json")
	require.Equal(t, http.StatusOK, loginRecorder.Code)

	var loginBody successPayload[loginPayload]
	require.NoError(t, json.Unmarshal(loginRecorder.Body.Bytes(), &loginBody))
	require.Equal(t, "success", loginBody.Status)
	require.NotNil(t, loginBody.Meta)
	require.NotEqual(t, registerBody.Data.Tokens.AccessToken, loginBody.Data.Tokens.AccessToken)
	require.NotEqual(t, registerBody.Data.Tokens.RefreshToken, loginBody.Data.Tokens.RefreshToken)
	require.Equal(t, registerBody.Data.User.Name, loginBody.Data.User.Name)

	// 4.- Refresh the issued tokens and ensure rotation occurs.
	refreshRecorder := performRequest(engine, http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"`+loginBody.Data.Tokens.RefreshToken+`"}`, "application/json")
	require.Equal(t, http.StatusOK, refreshRecorder.Code)

	var refreshBody successPayload[tokenPayload]
	require.NoError(t, json.Unmarshal(refreshRecorder.Body.Bytes(), &refreshBody))
	require.Equal(t, "success", refreshBody.Status)
	require.NotNil(t, refreshBody.Meta)
	require.NotEqual(t, loginBody.Data.Tokens.RefreshToken, refreshBody.Data.RefreshToken)
	require.NotEqual(t, loginBody.Data.Tokens.AccessToken, refreshBody.Data.AccessToken)

	// 5.- Ensure the previous refresh token is now invalid (reuse detection).
	staleRecorder := performRequest(engine, http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"`+loginBody.Data.Tokens.RefreshToken+`"}`, "application/json")
	require.Equal(t, http.StatusUnauthorized, staleRecorder.Code)

	var staleError errorPayload
	require.NoError(t, json.Unmarshal(staleRecorder.Body.Bytes(), &staleError))
	require.Equal(t, "error", staleError.Status)

	// 6.- Logout using the newest token pair.
	logoutPayload := `{"refresh_token":"` + refreshBody.Data.RefreshToken + `","access_token":"` + refreshBody.Data.AccessToken + `"}`
	logoutRecorder := performRequest(engine, http.MethodPost, "/v1/auth/logout", logoutPayload, "application/json")
	require.Equal(t, http.StatusOK, logoutRecorder.Code)

	var logoutBody successPayload[map[string]bool]
	require.NoError(t, json.Unmarshal(logoutRecorder.Body.Bytes(), &logoutBody))
	require.Equal(t, "success", logoutBody.Status)
	require.NotNil(t, logoutBody.Meta)
	require.True(t, logoutBody.Data["revoked"])

	// 7.- Refresh attempts after logout must fail due to blacklisting.
	blockedRecorder := performRequest(engine, http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"`+refreshBody.Data.RefreshToken+`"}`, "application/json")
	require.Equal(t, http.StatusUnauthorized, blockedRecorder.Code)

	var blockedError errorPayload
	require.NoError(t, json.Unmarshal(blockedRecorder.Body.Bytes(), &blockedError))
	require.Equal(t, "error", blockedError.Status)
}

// 1.- TestRegisterValidationErrors ensures missing fields produce structured feedback.
func TestRegisterValidationErrors(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/v1/auth/register", handler.Register)

	recorder := performRequest(engine, http.MethodPost, "/v1/auth/register", `{"email":" ","password":" "}`, "application/json")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var payload errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "error", payload.Status)
	fields, ok := payload.Errors["fields"].(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, fields, "email")
	require.Contains(t, fields, "password")
}

// 1.- TestLoginValidationErrors ensures invalid credentials trigger field errors before lookup.
func TestLoginValidationErrors(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/v1/auth/login", handler.Login)

	recorder := performRequest(engine, http.MethodPost, "/v1/auth/login", `{"email":"invalid","password":" "}`, "application/json")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var payload errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "error", payload.Status)
	fields := payload.Errors["fields"].(map[string]interface{})
	require.Contains(t, fields, "email")
	require.Contains(t, fields, "password")
}

// 1.- TestRefreshValidationError verifies blank refresh tokens fail validation.
func TestRefreshValidationError(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/v1/auth/refresh", handler.Refresh)

	recorder := performRequest(engine, http.MethodPost, "/v1/auth/refresh", `{"refresh_token":" "}`, "application/json")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var payload errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "error", payload.Status)
	fields := payload.Errors["fields"].(map[string]interface{})
	require.Contains(t, fields, "refresh_token")
}

// 1.- TestLogoutValidationError verifies both tokens must be supplied.
func TestLogoutValidationError(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/v1/auth/logout", handler.Logout)

	recorder := performRequest(engine, http.MethodPost, "/v1/auth/logout", `{"refresh_token":"","access_token":""}`, "application/json")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var payload errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "error", payload.Status)
	fields := payload.Errors["fields"].(map[string]interface{})
	require.Contains(t, fields, "refresh_token")
	require.Contains(t, fields, "access_token")
}

// 1.- TestCurrentUserEndpoint returns the principal envelope when context is populated.
func TestCurrentUserEndpoint(t *testing.T) {
	handler, store, _, cleanup := setupHandler(t)
	defer cleanup()

	_, err := store.Create(context.Background(), authpkg.User{ID: "user-123", Email: "principal@example.com", Name: "Principal"})
	require.NoError(t, err)

	engine := newTestEngine()
	engine.GET("/v1/user", func(ctx *gin.Context) {
		internalauth.SetPrincipal(ctx, internalauth.Principal{
			Subject:     "user-123",
			Roles:       []string{"member"},
			Permissions: []string{"read"},
		})
		handler.CurrentUser(ctx)
	})

	recorder := performRequest(engine, http.MethodGet, "/v1/user", "", "")
	require.Equal(t, http.StatusOK, recorder.Code)

	var body successPayload[principalPayload]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "success", body.Status)
	require.NotNil(t, body.Meta)
	require.Equal(t, "user-123", body.Data.Subject)
	require.Equal(t, "principal@example.com", body.Data.Email)
	require.Equal(t, "Principal", body.Data.Name)
	require.Equal(t, []string{"member"}, body.Data.Roles)
	require.Equal(t, []string{"read"}, body.Data.Permissions)
}

// 1.- TestCurrentUserUnauthorized ensures missing principals trigger an error envelope.
func TestCurrentUserUnauthorized(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.GET("/v1/user", handler.CurrentUser)

	recorder := performRequest(engine, http.MethodGet, "/v1/user", "", "")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	var body errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "error", body.Status)
	require.Equal(t, "authentication required", body.Message)
}

// 1.- TestVerifyEmailHappyPath ensures verification delegates to the service and returns success.
func TestVerifyEmailHappyPath(t *testing.T) {
	handler, _, verifier, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.GET("/email/verify/:id/:hash", handler.VerifyEmail)

	recorder := performRequest(engine, http.MethodGet, "/email/verify/123/hash-value", "", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, verifier.verifyCalls, 1)
	require.Equal(t, [2]string{"123", "hash-value"}, verifier.verifyCalls[0])

	var body successPayload[map[string]bool]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "success", body.Status)
	require.True(t, body.Data["verified"])
}

// 1.- TestVerifyEmailValidationError returns a bad request when parameters are missing.
func TestVerifyEmailValidationError(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.GET("/email/verify", handler.VerifyEmail)

	recorder := performRequest(engine, http.MethodGet, "/email/verify", "", "")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var body errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "error", body.Status)
	require.Equal(t, "invalid verification link", body.Message)
}

// 1.- TestVerifyEmailInvalidHash maps service errors to a 400 response.
func TestVerifyEmailInvalidHash(t *testing.T) {
	handler, _, verifier, cleanup := setupHandler(t)
	defer cleanup()

        verifier.verifyErr = authpkg.ErrInvalidVerification

	engine := newTestEngine()
	engine.GET("/email/verify/:id/:hash", handler.VerifyEmail)

	recorder := performRequest(engine, http.MethodGet, "/email/verify/123/hash-value", "", "")
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var body errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "error", body.Status)
	require.Equal(t, "invalid verification link", body.Message)
}

// 1.- TestResendVerificationRequiresPrincipal returns unauthorized when no subject exists.
func TestResendVerificationRequiresPrincipal(t *testing.T) {
	handler, _, _, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/email/verification-notification", handler.ResendVerification)

	recorder := performRequest(engine, http.MethodPost, "/email/verification-notification", "", "")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// 1.- TestResendVerificationThrottled surfaces throttling information with HTTP 429.
func TestResendVerificationThrottled(t *testing.T) {
	handler, _, verifier, cleanup := setupHandler(t)
	defer cleanup()

        verifier.resendErr = authpkg.ErrVerificationThrottled

	engine := newTestEngine()
	engine.POST("/email/verification-notification", func(ctx *gin.Context) {
		internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: "user-123"})
		handler.ResendVerification(ctx)
	})

	recorder := performRequest(engine, http.MethodPost, "/email/verification-notification", "", "")
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)

	var body errorPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "error", body.Status)
	require.Equal(t, "verification resend throttled", body.Message)
}

// 1.- TestResendVerificationSuccess triggers the resend workflow successfully.
func TestResendVerificationSuccess(t *testing.T) {
	handler, _, verifier, cleanup := setupHandler(t)
	defer cleanup()

	engine := newTestEngine()
	engine.POST("/email/verification-notification", func(ctx *gin.Context) {
		internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: "user-123"})
		handler.ResendVerification(ctx)
	})

	recorder := performRequest(engine, http.MethodPost, "/email/verification-notification", "", "")
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, []string{"user-123"}, verifier.resendCalls)

	var body successPayload[map[string]bool]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "success", body.Status)
	require.True(t, body.Data["resent"])
}
