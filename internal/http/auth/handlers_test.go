package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/config"
)

// 1.- memoryUserStore stores users in memory for deterministic handler tests.
type memoryUserStore struct {
	users   map[string]User
	byEmail map[string]string
}

// 1.- newMemoryUserStore constructs a thread-safe in-memory repository.
func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{users: map[string]User{}, byEmail: map[string]string{}}
}

// 1.- Create inserts a new user into the in-memory maps.
func (m *memoryUserStore) Create(_ context.Context, user User) (User, error) {
	if _, exists := m.byEmail[user.Email]; exists {
		return User{}, errors.New("duplicate email")
	}
	copied := user
	m.users[user.ID] = copied
	m.byEmail[user.Email] = user.ID
	return copied, nil
}

// 1.- FindByEmail returns the stored user or ErrUserNotFound.
func (m *memoryUserStore) FindByEmail(_ context.Context, email string) (User, error) {
	id, ok := m.byEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return m.users[id], nil
}

// 1.- setupHandler constructs a handler with real token service dependencies.
func setupHandler(t *testing.T) (Handler, *memoryUserStore, func()) {
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
	handler := NewHandler(svc, store)
	cleanup := func() {
		_ = client.Close()
		mini.Close()
	}

	return handler, store, cleanup
}

// 1.- successPayload generically decodes success envelopes in tests.
type successPayload[T any] struct {
	Data T              `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- TestRegisterLoginRefreshLogoutFlow covers the complete authentication lifecycle.
func TestRegisterLoginRefreshLogoutFlow(t *testing.T) {
	handler, _, cleanup := setupHandler(t)
	defer cleanup()

	// 2.- Exercise the registration endpoint.
	registerRecorder := httptest.NewRecorder()
	registerCtx, _ := gin.CreateTestContext(registerRecorder)
	registerCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewBufferString(`{"email":"user@example.com","password":"secret"}`))
	registerCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Register(registerCtx)
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registerBody successPayload[registerResponse]
	require.NoError(t, json.Unmarshal(registerRecorder.Body.Bytes(), &registerBody))
	require.NotNil(t, registerBody.Meta)
	require.NotEmpty(t, registerBody.Data.User.ID)
	require.Equal(t, "user@example.com", registerBody.Data.User.Email)
	require.NotEmpty(t, registerBody.Data.Tokens.AccessToken)
	require.NotEmpty(t, registerBody.Data.Tokens.RefreshToken)

	// 3.- Attempt a login with the same credentials; tokens must rotate.
	loginRecorder := httptest.NewRecorder()
	loginCtx, _ := gin.CreateTestContext(loginRecorder)
	loginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBufferString(`{"email":"user@example.com","password":"secret"}`))
	loginCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Login(loginCtx)
	require.Equal(t, http.StatusOK, loginRecorder.Code)

	var loginBody successPayload[loginResponse]
	require.NoError(t, json.Unmarshal(loginRecorder.Body.Bytes(), &loginBody))
	require.NotNil(t, loginBody.Meta)
	require.NotEqual(t, registerBody.Data.Tokens.AccessToken, loginBody.Data.Tokens.AccessToken)
	require.NotEqual(t, registerBody.Data.Tokens.RefreshToken, loginBody.Data.Tokens.RefreshToken)

	// 4.- Refresh the issued tokens and ensure rotation occurs.
	refreshRecorder := httptest.NewRecorder()
	refreshCtx, _ := gin.CreateTestContext(refreshRecorder)
	refreshPayload := successPayload[tokenEnvelope]{}
	refreshCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+loginBody.Data.Tokens.RefreshToken+`"}`))
	refreshCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Refresh(refreshCtx)
	require.Equal(t, http.StatusOK, refreshRecorder.Code)

	require.NoError(t, json.Unmarshal(refreshRecorder.Body.Bytes(), &refreshPayload))
	require.NotNil(t, refreshPayload.Meta)
	require.NotEqual(t, loginBody.Data.Tokens.RefreshToken, refreshPayload.Data.RefreshToken)
	require.NotEqual(t, loginBody.Data.Tokens.AccessToken, refreshPayload.Data.AccessToken)

	// 5.- Ensure the previous refresh token is now invalid (reuse detection).
	staleRecorder := httptest.NewRecorder()
	staleCtx, _ := gin.CreateTestContext(staleRecorder)
	staleCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+loginBody.Data.Tokens.RefreshToken+`"}`))
	staleCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Refresh(staleCtx)
	require.Equal(t, http.StatusUnauthorized, staleRecorder.Code)

	// 6.- Logout using the newest token pair.
	logoutRecorder := httptest.NewRecorder()
	logoutCtx, _ := gin.CreateTestContext(logoutRecorder)
	logoutPayload := `{"refresh_token":"` + refreshPayload.Data.RefreshToken + `","access_token":"` + refreshPayload.Data.AccessToken + `"}`
	logoutCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/logout", bytes.NewBufferString(logoutPayload))
	logoutCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Logout(logoutCtx)
	require.Equal(t, http.StatusOK, logoutRecorder.Code)

	var logoutBody successPayload[map[string]bool]
	require.NoError(t, json.Unmarshal(logoutRecorder.Body.Bytes(), &logoutBody))
	require.NotNil(t, logoutBody.Meta)
	require.True(t, logoutBody.Data["revoked"])

	// 7.- Refresh attempts after logout must fail due to blacklisting.
	blockedRecorder := httptest.NewRecorder()
	blockedCtx, _ := gin.CreateTestContext(blockedRecorder)
	blockedCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+refreshPayload.Data.RefreshToken+`"}`))
	blockedCtx.Request.Header.Set("Content-Type", "application/json")
	handler.Refresh(blockedCtx)
	require.Equal(t, http.StatusUnauthorized, blockedRecorder.Code)
}

// 1.- TestCurrentUserEndpoint returns the principal envelope when context is populated.
func TestCurrentUserEndpoint(t *testing.T) {
	handler, _, cleanup := setupHandler(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/user", nil)

	internalauth.SetPrincipal(ctx, internalauth.Principal{
		Subject:     "user-123",
		Roles:       []string{"member"},
		Permissions: []string{"read"},
	})

	handler.CurrentUser(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body successPayload[principalResponse]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotNil(t, body.Meta)
	require.Equal(t, "user-123", body.Data.Subject)
	require.Equal(t, []string{"member"}, body.Data.Roles)
	require.Equal(t, []string{"read"}, body.Data.Permissions)
}

// 1.- TestCurrentUserUnauthorized ensures missing principals trigger an error envelope.
func TestCurrentUserUnauthorized(t *testing.T) {
	handler, _, cleanup := setupHandler(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/user", nil)

	handler.CurrentUser(ctx)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	var body errorEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotEmpty(t, body.Message)
	require.NotNil(t, body.Errors)
}
