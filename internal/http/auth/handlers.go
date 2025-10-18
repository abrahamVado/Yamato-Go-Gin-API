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
	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
	"github.com/example/Yamato-Go-Gin-API/internal/http/validation"
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
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
}

// 1.- UserStore abstracts persistence for registering and retrieving users.
type UserStore interface {
	// 2.- Create persists a new user and returns the stored representation.
	Create(ctx context.Context, user User) (User, error)
	// 3.- FindByEmail retrieves a user by e-mail or returns ErrUserNotFound.
	FindByEmail(ctx context.Context, email string) (User, error)
	// 4.- FindByID retrieves a user by identifier when loading principals.
	FindByID(ctx context.Context, id string) (User, error)
}

// 1.- AuthService defines the subset of the core auth service used by handlers.
type AuthService interface {
	HashPassword(password string) (string, error)
	CheckPassword(hash string, password string) error
	Login(ctx context.Context, subject string) (internalauth.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (internalauth.TokenPair, error)
	Logout(ctx context.Context, refreshToken string, accessToken string) error
}

// 1.- Handler wires HTTP requests to the auth service, user store, and validator dependencies.
type Handler struct {
	auth         AuthService
	users        UserStore
	verification EmailVerificationService
	validator    *validation.Validator
}

// 1.- NewHandler constructs a Handler with the supplied dependencies and shared validator.
func NewHandler(auth AuthService, users UserStore, verification EmailVerificationService) Handler {
	validator, err := validation.New()
	if err != nil {
		panic(err)
	}
	return Handler{auth: auth, users: users, verification: verification, validator: validator}
}

// 1.- EmailVerificationService defines verification and resend workflows used by handlers.
type EmailVerificationService interface {
	Verify(ctx context.Context, userID string, hash string) error
	Resend(ctx context.Context, userID string) error
	HashForUser(userID string) string
}

// 1.- registerRequest models the expected payload for account creation.
type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Name     string `json:"name" validate:"omitempty"`
}

// 1.- loginRequest models credential-based authentication attempts.
type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// 1.- refreshRequest captures the refresh token rotation payload.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// 1.- logoutRequest requires both refresh and access tokens for revocation.
type logoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	AccessToken  string `json:"access_token" validate:"required"`
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
	User             User          `json:"user"`
	Tokens           tokenEnvelope `json:"tokens"`
	Notice           string        `json:"notice,omitempty"`
	VerificationHash string        `json:"verification_hash,omitempty"`
}

// 1.- loginResponse mirrors registerResponse for successful logins.
type loginResponse struct {
	User   User          `json:"user"`
	Tokens tokenEnvelope `json:"tokens"`
}

// 1.- principalResponse mirrors auth.Principal details for the /user endpoint.
type principalResponse struct {
	Subject     string   `json:"subject"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
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

// 1.- validatePayload executes structural validation and surfaces consistent error responses.
func (h Handler) validatePayload(ctx *gin.Context, payload interface{}) bool {
	if h.validator == nil {
		respond.Error(ctx, http.StatusInternalServerError, "validation unavailable", map[string]interface{}{"details": "validator is not configured"})
		return false
	}

	errs, err := h.validator.ValidateStruct(payload)
	if err != nil {
		respond.Error(ctx, http.StatusInternalServerError, "validation unavailable", map[string]interface{}{"details": err.Error()})
		return false
	}
	if !errs.Empty() {
		respond.Error(ctx, http.StatusBadRequest, "validation failed", errs.ToMap())
		return false
	}
	return true
}

// 1.- Register creates a user account and issues an initial token pair.
func (h Handler) Register(ctx *gin.Context) {
	// 1.- Bind the incoming JSON payload and surface parsing errors immediately.
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Error(ctx, http.StatusBadRequest, "invalid request payload", map[string]interface{}{"details": err.Error()})
		return
	}

	// 2.- Normalize user supplied fields prior to validation and persistence.
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	req.Name = strings.TrimSpace(req.Name)

	// 3.- Validate the normalized payload with structured field errors.
	if !h.validatePayload(ctx, req) {
		return
	}

	// 4.- Derive a default display name when none is supplied.
	if req.Name == "" {
		if at := strings.Index(req.Email, "@"); at > 0 {
			local := strings.ReplaceAll(req.Email[:at], ".", " ")
			words := strings.Fields(local)
			for i, word := range words {
				if len(word) == 0 {
					continue
				}
				lower := strings.ToLower(word)
				words[i] = strings.ToUpper(lower[:1]) + lower[1:]
			}
			req.Name = strings.Join(words, " ")
		}
		if req.Name == "" {
			req.Name = req.Email
		}
	}

	// 5.- Guard against duplicate registrations for the same email address.
	if _, err := h.users.FindByEmail(ctx.Request.Context(), req.Email); err == nil {
		respond.Error(ctx, http.StatusConflict, "account already exists", map[string]interface{}{"fields": map[string][]validation.FieldError{
			"email": []validation.FieldError{{Field: "email", Rule: "unique", Message: "email already registered"}},
		}})
		return
	} else if !errors.Is(err, ErrUserNotFound) {
		respond.Error(ctx, http.StatusInternalServerError, "failed to query users", map[string]interface{}{"details": err.Error()})
		return
	}

	// 6.- Hash the provided password prior to persistence.
	hashed, err := h.auth.HashPassword(req.Password)
	if err != nil {
		respond.Error(ctx, http.StatusInternalServerError, "failed to hash password", map[string]interface{}{"details": err.Error()})
		return
	}

	// 7.- Persist the new user record via the store abstraction.
	user := User{ID: uuid.NewString(), Email: req.Email, Name: req.Name, PasswordHash: hashed}
	created, err := h.users.Create(ctx.Request.Context(), user)
	if err != nil {
		respond.Error(ctx, http.StatusInternalServerError, "failed to create user", map[string]interface{}{"details": err.Error()})
		return
	}

	// 8.- Issue a fresh token pair for the registered user.
	pair, err := h.auth.Login(ctx.Request.Context(), created.ID)
	if err != nil {
		respond.Error(ctx, http.StatusInternalServerError, "failed to issue tokens", map[string]interface{}{"details": err.Error()})
		return
	}

	// 9.- Compose a verification notice mirroring Laravel's onboarding flow.
	notice := "Please verify your email address for {email}."
	verificationHash := ""
	if h.verification != nil {
		verificationHash = h.verification.HashForUser(created.ID)
		notice = notice + " Use the link in your inbox to complete setup."
	}

	// 10.- Return the success envelope with the new user, tokens, and verification metadata.
	respond.Success(ctx, http.StatusCreated, registerResponse{
		User:             User{ID: created.ID, Email: created.Email, Name: created.Name},
		Tokens:           pairToEnvelope(pair),
		Notice:           notice,
		VerificationHash: verificationHash,
	}, nil)
}

// 1.- Login authenticates an existing user and returns a rotated token pair.
func (h Handler) Login(ctx *gin.Context) {
	// 1.- Bind the request payload and validate structure.
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Error(ctx, http.StatusBadRequest, "invalid request payload", map[string]interface{}{"details": err.Error()})
		return
	}

	// 2.- Normalize credentials before lookup.
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	// 3.- Validate the normalized payload to surface actionable errors.
	if !h.validatePayload(ctx, req) {
		return
	}

	// 4.- Retrieve the user from storage.
	user, err := h.users.FindByEmail(ctx.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			respond.Error(ctx, http.StatusUnauthorized, "invalid credentials", map[string]interface{}{"fields": map[string][]validation.FieldError{
				"email": []validation.FieldError{{Field: "email", Rule: "credentials", Message: "email not found"}},
			}})
			return
		}
		respond.Error(ctx, http.StatusInternalServerError, "failed to query users", map[string]interface{}{"details": err.Error()})
		return
	}

	// 5.- Compare the provided password with the stored hash.
	if err := h.auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		respond.Error(ctx, http.StatusUnauthorized, "invalid credentials", map[string]interface{}{"fields": map[string][]validation.FieldError{
			"password": []validation.FieldError{{Field: "password", Rule: "credentials", Message: "password mismatch"}},
		}})
		return
	}

	// 6.- Issue a new token pair for the authenticated subject.
	pair, err := h.auth.Login(ctx.Request.Context(), user.ID)
	if err != nil {
		respond.Error(ctx, http.StatusInternalServerError, "failed to issue tokens", map[string]interface{}{"details": err.Error()})
		return
	}

	// 7.- Return the authenticated user and token envelope.
	respond.Success(ctx, http.StatusOK, loginResponse{User: User{ID: user.ID, Email: user.Email, Name: user.Name}, Tokens: pairToEnvelope(pair)}, nil)
}

// 1.- Refresh rotates refresh tokens and returns a new token pair.
func (h Handler) Refresh(ctx *gin.Context) {
	// 1.- Bind the refresh token payload.
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Error(ctx, http.StatusBadRequest, "invalid request payload", map[string]interface{}{"details": err.Error()})
		return
	}

	// 2.- Ensure the refresh token is present before invoking the service.
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if !h.validatePayload(ctx, req) {
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
		respond.Error(ctx, status, "failed to refresh token", map[string]interface{}{"details": err.Error()})
		return
	}

	// 4.- Return the rotated token pair to the client.
	respond.Success(ctx, http.StatusOK, pairToEnvelope(pair), nil)
}

// 1.- Logout revokes the supplied tokens to terminate the session.
func (h Handler) Logout(ctx *gin.Context) {
	// 1.- Bind the refresh and access tokens from the payload.
	var req logoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Error(ctx, http.StatusBadRequest, "invalid request payload", map[string]interface{}{"details": err.Error()})
		return
	}

	// 2.- Validate that both tokens are present.
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	if !h.validatePayload(ctx, req) {
		return
	}

	// 3.- Attempt to revoke the provided tokens using the auth service.
	if err := h.auth.Logout(ctx.Request.Context(), req.RefreshToken, req.AccessToken); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, internalauth.ErrInvalidToken) {
			status = http.StatusUnauthorized
		}
		respond.Error(ctx, status, "failed to logout", map[string]interface{}{"details": err.Error()})
		return
	}

	// 4.- Indicate success with an empty data payload.
	respond.Success(ctx, http.StatusOK, map[string]any{"revoked": true}, nil)
}

// 1.- CurrentUser returns the authenticated principal captured by middleware.
func (h Handler) CurrentUser(ctx *gin.Context) {
	// 1.- Extract the principal from the Gin context.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		respond.Error(ctx, http.StatusUnauthorized, "authentication required", map[string]interface{}{"reason": "principal missing"})
		return
	}

	// 2.- Load the persisted user so contact details can be returned alongside roles.
	user, err := h.users.FindByID(ctx.Request.Context(), principal.Subject)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			respond.Error(ctx, http.StatusNotFound, "user not found", map[string]interface{}{"fields": map[string][]validation.FieldError{
				"id": []validation.FieldError{{Field: "id", Rule: "exists", Message: "user not found"}},
			}})
			return
		}
		respond.Error(ctx, http.StatusInternalServerError, "failed to load user", map[string]interface{}{"details": err.Error()})
		return
	}

	// 3.- Return the principal details as a success envelope.
	respond.Success(ctx, http.StatusOK, principalResponse{
		Subject:     principal.Subject,
		Email:       user.Email,
		Name:        user.Name,
		Roles:       principal.Roles,
		Permissions: principal.Permissions,
	}, nil)
}

// 1.- VerifyEmail confirms a user's email address via Laravel-compatible parameters.
func (h Handler) VerifyEmail(ctx *gin.Context) {
	// 1.- Guard against missing verification dependencies to surface clear errors.
	if h.verification == nil {
		respond.Error(ctx, http.StatusServiceUnavailable, "verification service unavailable", map[string]interface{}{"reason": "not configured"})
		return
	}

	// 2.- Normalize and validate required path parameters.
	userID := strings.TrimSpace(ctx.Param("id"))
	hash := strings.TrimSpace(ctx.Param("hash"))
	if userID == "" || hash == "" {
		respond.Error(ctx, http.StatusBadRequest, "invalid verification link", map[string]interface{}{"verification": "missing id or hash"})
		return
	}

	// 3.- Delegate the verification logic to the backing service.
	if err := h.verification.Verify(ctx.Request.Context(), userID, hash); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			respond.Error(ctx, http.StatusNotFound, "user not found", map[string]interface{}{"fields": map[string][]validation.FieldError{
				"id": []validation.FieldError{{Field: "id", Rule: "exists", Message: "user not found"}},
			}})
		case errors.Is(err, ErrInvalidVerification):
			respond.Error(ctx, http.StatusBadRequest, "invalid verification link", map[string]interface{}{"verification": "hash mismatch"})
		default:
			respond.Error(ctx, http.StatusInternalServerError, "failed to verify email", map[string]interface{}{"details": err.Error()})
		}
		return
	}

	// 4.- Respond with a Laravel-compatible success payload signalling verification completion.
	respond.Success(ctx, http.StatusOK, map[string]any{"verified": true}, nil)
}

// 1.- ResendVerification triggers a new verification email for the authenticated user.
func (h Handler) ResendVerification(ctx *gin.Context) {
	// 1.- Ensure the verification dependency is configured.
	if h.verification == nil {
		respond.Error(ctx, http.StatusServiceUnavailable, "verification service unavailable", map[string]interface{}{"reason": "not configured"})
		return
	}

	// 2.- Require an authenticated principal mirroring Laravel's middleware behavior.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		respond.Error(ctx, http.StatusUnauthorized, "authentication required", map[string]interface{}{"reason": "principal missing"})
		return
	}

	// 3.- Ask the service to deliver a new verification notification for the subject.
	if err := h.verification.Resend(ctx.Request.Context(), principal.Subject); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			respond.Error(ctx, http.StatusNotFound, "user not found", map[string]interface{}{"fields": map[string][]validation.FieldError{
				"id": []validation.FieldError{{Field: "id", Rule: "exists", Message: "user not found"}},
			}})
		case errors.Is(err, ErrVerificationThrottled):
			respond.Error(ctx, http.StatusTooManyRequests, "verification resend throttled", map[string]interface{}{"reason": "rate limited"})
		default:
			respond.Error(ctx, http.StatusInternalServerError, "failed to resend verification", map[string]interface{}{"details": err.Error()})
		}
		return
	}

	// 4.- Return an accepted response to align with Laravel's resend semantics.
	respond.Success(ctx, http.StatusAccepted, map[string]any{"resent": true}, nil)
}
