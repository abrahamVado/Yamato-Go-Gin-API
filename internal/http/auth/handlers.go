package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- ErrUserNotFound is returned by repositories when an email or identifier is missing.
var ErrUserNotFound = errors.New("http/auth: user not found")

// 1.- ErrInvalidVerification indicates that the supplied verification hash is not valid.
var ErrInvalidVerification = errors.New("http/auth: invalid verification link")

// 1.- ErrVerificationThrottled signals that verification resends are temporarily rate limited.
var ErrVerificationThrottled = errors.New("http/auth: verification request throttled")

// 1.- User represents the minimal information stored for authentication flows.
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

// 1.- UserStore abstracts persistence for registering and retrieving users.
type UserStore interface {
	// 2.- Create persists a new user and returns the stored representation.
	Create(ctx context.Context, user User) (User, error)
	// 3.- FindByEmail retrieves a user by e-mail or returns ErrUserNotFound.
	FindByEmail(ctx context.Context, email string) (User, error)
}

// 1.- AuthService defines the subset of the core auth service used by handlers.
type AuthService interface {
	HashPassword(password string) (string, error)
	CheckPassword(hash string, password string) error
	Login(ctx context.Context, subject string) (internalauth.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (internalauth.TokenPair, error)
	Logout(ctx context.Context, refreshToken string, accessToken string) error
}

// 1.- Handler wires HTTP requests to the auth service and user store dependencies.
type Handler struct {
	auth         AuthService
	users        UserStore
	verification EmailVerificationService
}

// 1.- NewHandler constructs a Handler with the supplied dependencies.
func NewHandler(auth AuthService, users UserStore, verification EmailVerificationService) Handler {
	return Handler{auth: auth, users: users, verification: verification}
}

// 1.- EmailVerificationService defines verification and resend workflows used by handlers.
type EmailVerificationService interface {
	Verify(ctx context.Context, userID string, hash string) error
	Resend(ctx context.Context, userID string) error
}

// 1.- registerRequest models the expected payload for account creation.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// 1.- loginRequest models credential-based authentication attempts.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// 1.- refreshRequest captures the refresh token rotation payload.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// 1.- logoutRequest requires both refresh and access tokens for revocation.
type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

// 1.- tokenEnvelope exposes tokens alongside expiration metadata.
type tokenEnvelope struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// 1.- registerResponse bundles the created user and issued tokens.
type registerResponse struct {
	User   User          `json:"user"`
	Tokens tokenEnvelope `json:"tokens"`
}

// 1.- loginResponse mirrors registerResponse for successful logins.
type loginResponse struct {
	User   User          `json:"user"`
	Tokens tokenEnvelope `json:"tokens"`
}

// 1.- principalResponse mirrors auth.Principal details for the /user endpoint.
type principalResponse struct {
	Subject     string   `json:"subject"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// 1.- successEnvelope conforms to ADR-003 success payload layout.
type successEnvelope struct {
	Data interface{}    `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- errorEnvelope conforms to ADR-003 failure payload layout.
type errorEnvelope struct {
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- newMeta creates a meta object ensuring the field exists on success responses.
func newMeta() map[string]any {
	return map[string]any{}
}

// 1.- newErrors creates the error map container for failure envelopes.
func newErrors() map[string]interface{} {
	return map[string]interface{}{}
}

// 1.- writeSuccess standardizes success responses with the correct envelope.
func writeSuccess(ctx *gin.Context, status int, data interface{}) {
	ctx.JSON(status, successEnvelope{Data: data, Meta: newMeta()})
}

// 1.- writeError standardizes error responses using ADR-003 envelopes.
func writeError(ctx *gin.Context, status int, message string, errs map[string]interface{}) {
	if errs == nil {
		errs = newErrors()
	}
	ctx.JSON(status, errorEnvelope{Message: message, Errors: errs})
}

// 1.- pairToEnvelope maps TokenPair instances to the JSON structure returned to clients.
func pairToEnvelope(pair internalauth.TokenPair) tokenEnvelope {
	return tokenEnvelope{
		AccessToken:      pair.AccessToken,
		RefreshToken:     pair.RefreshToken,
		AccessExpiresAt:  pair.AccessExpiresAt,
		RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}

// 1.- Register creates a user account and issues an initial token pair.
func (h Handler) Register(ctx *gin.Context) {
	// 1.- Bind the incoming JSON payload and surface validation errors.
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	// 2.- Normalize and validate required fields.
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	if req.Email == "" || req.Password == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"validation": map[string][]map[string]any{}})
		return
	}

	// 3.- Guard against duplicate registrations for the same email address.
	if _, err := h.users.FindByEmail(ctx.Request.Context(), req.Email); err == nil {
		writeError(ctx, http.StatusConflict, "account already exists", nil)
		return
	} else if !errors.Is(err, ErrUserNotFound) {
		writeError(ctx, http.StatusInternalServerError, "failed to query users", nil)
		return
	}

	// 4.- Hash the provided password prior to persistence.
	hashed, err := h.auth.HashPassword(req.Password)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "failed to hash password", nil)
		return
	}

	// 5.- Persist the new user record via the store abstraction.
	user := User{ID: uuid.NewString(), Email: req.Email, PasswordHash: hashed}
	created, err := h.users.Create(ctx.Request.Context(), user)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "failed to create user", nil)
		return
	}

	// 6.- Issue a fresh token pair for the registered user.
	pair, err := h.auth.Login(ctx.Request.Context(), created.ID)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "failed to issue tokens", nil)
		return
	}

	// 7.- Return the success envelope with the new user and tokens.
	writeSuccess(ctx, http.StatusCreated, registerResponse{User: User{ID: created.ID, Email: created.Email}, Tokens: pairToEnvelope(pair)})
}

// 1.- Login authenticates an existing user and returns a rotated token pair.
func (h Handler) Login(ctx *gin.Context) {
	// 1.- Bind the request payload and validate structure.
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	// 2.- Normalize credentials before lookup.
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	if req.Email == "" || req.Password == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"validation": map[string][]map[string]any{}})
		return
	}

	// 3.- Retrieve the user from storage.
	user, err := h.users.FindByEmail(ctx.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(ctx, http.StatusUnauthorized, "invalid credentials", nil)
			return
		}
		writeError(ctx, http.StatusInternalServerError, "failed to query users", nil)
		return
	}

	// 4.- Compare the provided password with the stored hash.
	if err := h.auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(ctx, http.StatusUnauthorized, "invalid credentials", nil)
		return
	}

	// 5.- Issue a new token pair for the authenticated subject.
	pair, err := h.auth.Login(ctx.Request.Context(), user.ID)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "failed to issue tokens", nil)
		return
	}

	// 6.- Return the authenticated user and token envelope.
	writeSuccess(ctx, http.StatusOK, loginResponse{User: User{ID: user.ID, Email: user.Email}, Tokens: pairToEnvelope(pair)})
}

// 1.- Refresh rotates refresh tokens and returns a new token pair.
func (h Handler) Refresh(ctx *gin.Context) {
	// 1.- Bind the refresh token payload.
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	// 2.- Ensure the refresh token is present before invoking the service.
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"validation": map[string][]map[string]any{}})
		return
	}

	// 3.- Delegate rotation to the auth service while translating domain errors.
	pair, err := h.auth.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, internalauth.ErrInvalidToken):
			status = http.StatusUnauthorized
		case errors.Is(err, internalauth.ErrBlacklisted), errors.Is(err, internalauth.ErrReuseDetected):
			status = http.StatusUnauthorized
		}
		writeError(ctx, status, "failed to refresh token", nil)
		return
	}

	// 4.- Return the rotated token pair to the client.
	writeSuccess(ctx, http.StatusOK, pairToEnvelope(pair))
}

// 1.- Logout revokes the supplied tokens to terminate the session.
func (h Handler) Logout(ctx *gin.Context) {
	// 1.- Bind the refresh and access tokens from the payload.
	var req logoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	// 2.- Validate that both tokens are present.
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	if req.RefreshToken == "" || req.AccessToken == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"validation": map[string][]map[string]any{}})
		return
	}

	// 3.- Attempt to revoke the provided tokens using the auth service.
	if err := h.auth.Logout(ctx.Request.Context(), req.RefreshToken, req.AccessToken); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, internalauth.ErrInvalidToken) {
			status = http.StatusUnauthorized
		}
		writeError(ctx, status, "failed to logout", nil)
		return
	}

	// 4.- Indicate success with an empty data payload.
	writeSuccess(ctx, http.StatusOK, map[string]any{"revoked": true})
}

// 1.- CurrentUser returns the authenticated principal captured by middleware.
func (h Handler) CurrentUser(ctx *gin.Context) {
	// 1.- Extract the principal from the Gin context.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	// 2.- Return the principal details as a success envelope.
	writeSuccess(ctx, http.StatusOK, principalResponse{
		Subject:     principal.Subject,
		Roles:       principal.Roles,
		Permissions: principal.Permissions,
	})
}

// 1.- VerifyEmail confirms a user's email address via Laravel-compatible parameters.
func (h Handler) VerifyEmail(ctx *gin.Context) {
	// 1.- Guard against missing verification dependencies to surface clear errors.
	if h.verification == nil {
		writeError(ctx, http.StatusServiceUnavailable, "verification service unavailable", nil)
		return
	}

	// 2.- Normalize and validate required path parameters.
	userID := strings.TrimSpace(ctx.Param("id"))
	hash := strings.TrimSpace(ctx.Param("hash"))
	if userID == "" || hash == "" {
		writeError(ctx, http.StatusBadRequest, "invalid verification link", map[string]interface{}{"verification": "missing id or hash"})
		return
	}

	// 3.- Delegate the verification logic to the backing service.
	if err := h.verification.Verify(ctx.Request.Context(), userID, hash); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			writeError(ctx, http.StatusNotFound, "user not found", nil)
		case errors.Is(err, ErrInvalidVerification):
			writeError(ctx, http.StatusBadRequest, "invalid verification link", nil)
		default:
			writeError(ctx, http.StatusInternalServerError, "failed to verify email", nil)
		}
		return
	}

	// 4.- Respond with a Laravel-compatible success payload signalling verification completion.
	writeSuccess(ctx, http.StatusOK, map[string]any{"verified": true})
}

// 1.- ResendVerification triggers a new verification email for the authenticated user.
func (h Handler) ResendVerification(ctx *gin.Context) {
	// 1.- Ensure the verification dependency is configured.
	if h.verification == nil {
		writeError(ctx, http.StatusServiceUnavailable, "verification service unavailable", nil)
		return
	}

	// 2.- Require an authenticated principal mirroring Laravel's middleware behavior.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	// 3.- Ask the service to deliver a new verification notification for the subject.
	if err := h.verification.Resend(ctx.Request.Context(), principal.Subject); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			writeError(ctx, http.StatusNotFound, "user not found", nil)
		case errors.Is(err, ErrVerificationThrottled):
			writeError(ctx, http.StatusTooManyRequests, "verification resend throttled", nil)
		default:
			writeError(ctx, http.StatusInternalServerError, "failed to resend verification", nil)
		}
		return
	}

	// 4.- Return an accepted response to align with Laravel's resend semantics.
	writeSuccess(ctx, http.StatusAccepted, map[string]any{"resent": true})
}
