package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/Yamato-Go-Gin-API/internal/config"
)

// 1.- ErrInvalidToken indicates that a provided token failed validation or parsing checks.
var ErrInvalidToken = errors.New("auth: invalid token")

// 1.- ErrBlacklisted indicates that a token or token family has been revoked server-side.
var ErrBlacklisted = errors.New("auth: token blacklisted")

// 1.- ErrReuseDetected signals detection of refresh token reuse for a tracked family.
var ErrReuseDetected = errors.New("auth: refresh token reuse detected")

// 1.- RedisCommander defines the minimal Redis commands required by the auth service.
type RedisCommander interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// 1.- Service coordinates password hashing, token issuance, and revocation bookkeeping.
type Service struct {
	cfg    config.JWTConfig
	redis  RedisCommander
	now    func() time.Time
	secret []byte
}

// 1.- TokenPair captures the issued tokens alongside their expiration timestamps.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// 1.- accessClaims extends registered JWT claims with the owning refresh family identifier.
type accessClaims struct {
	FamilyID string `json:"fam"`
	jwt.RegisteredClaims
}

// 1.- refreshClaims embeds the refresh family identifier for reuse detection.
type refreshClaims struct {
	FamilyID string `json:"fam"`
	jwt.RegisteredClaims
}

// 1.- NewService constructs a Service with sane defaults for token lifetimes and clock source.
func NewService(cfg config.JWTConfig, redis RedisCommander) (*Service, error) {
	if cfg.Secret == "" {
		return nil, errors.New("auth: jwt secret must be configured")
	}

	//1.- Adopt opinionated defaults when durations were not loaded from configuration.
	if cfg.AccessExpiration <= 0 {
		cfg.AccessExpiration = 15 * time.Minute
	}
	if cfg.RefreshExpiration <= 0 {
		cfg.RefreshExpiration = 30 * 24 * time.Hour
	}

	return &Service{
		cfg:    cfg,
		redis:  redis,
		now:    time.Now,
		secret: []byte(cfg.Secret),
	}, nil
}

// 1.- HashPassword hashes a plaintext password using bcrypt with cost 12 for storage.
func (s *Service) HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("auth: bcrypt hash: %w", err)
	}
	return string(hashed), nil
}

// 1.- CheckPassword compares a stored bcrypt hash against a provided password candidate.
func (s *Service) CheckPassword(hash string, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidToken
		}
		return fmt.Errorf("auth: bcrypt compare: %w", err)
	}
	return nil
}

// 1.- Login issues a new access and refresh token pair, tracking refresh state in Redis.
func (s *Service) Login(ctx context.Context, subject string) (TokenPair, error) {
	familyID := uuid.NewString()
	return s.issueTokens(ctx, subject, familyID)
}

// 1.- Refresh validates a refresh token, rotates it, and revokes the old instance.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := s.parseRefreshClaims(refreshToken)
	if err != nil {
		return TokenPair{}, err
	}

	//1.- Abort if the entire family has already been revoked (logout or prior reuse).
	blacklisted, err := s.isFamilyBlacklisted(ctx, claims.FamilyID)
	if err != nil {
		return TokenPair{}, err
	}
	if blacklisted {
		return TokenPair{}, ErrReuseDetected
	}

	//1.- Confirm that the token presented matches the currently active refresh identifier.
	stored, err := s.redis.Get(ctx, refreshFamilyKey(claims.FamilyID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return TokenPair{}, ErrInvalidToken
		}
		return TokenPair{}, fmt.Errorf("auth: lookup refresh family: %w", err)
	}
	if stored != claims.ID {
		if err := s.blacklistFamily(ctx, claims.FamilyID); err != nil {
			return TokenPair{}, err
		}
		return TokenPair{}, ErrReuseDetected
	}

	pair, err := s.issueTokens(ctx, claims.Subject, claims.FamilyID)
	if err != nil {
		return TokenPair{}, err
	}

	return pair, nil
}

// 1.- Logout blacklists the provided refresh token family and the supplied access token JTI.
func (s *Service) Logout(ctx context.Context, refreshToken string, accessToken string) error {
	rClaims, err := s.parseRefreshClaims(refreshToken)
	if err != nil {
		return err
	}
	if err := s.blacklistFamily(ctx, rClaims.FamilyID); err != nil {
		return err
	}

	aClaims, err := s.parseAccessClaims(accessToken)
	if err != nil {
		return err
	}

	ttl := time.Until(aClaims.ExpiresAt.Time)
	if ttl <= 0 {
		ttl = time.Second
	}
	if err := s.redis.Set(ctx, accessBlacklistKey(aClaims.ID), "1", ttl).Err(); err != nil {
		return fmt.Errorf("auth: blacklist access token: %w", err)
	}
	return nil
}

// 1.- ValidateAccessToken verifies signature, expiry, and blacklist state of an access token.
func (s *Service) ValidateAccessToken(ctx context.Context, token string) (*accessClaims, error) {
	claims, err := s.parseAccessClaims(token)
	if err != nil {
		return nil, err
	}

	//1.- Reject tokens that have been explicitly revoked in Redis.
	_, err = s.redis.Get(ctx, accessBlacklistKey(claims.ID)).Result()
	if err == nil {
		return nil, ErrBlacklisted
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("auth: access blacklist lookup: %w", err)
	}

	return claims, nil
}

// 1.- issueTokens mints signed JWT access and refresh tokens and persists refresh metadata.
func (s *Service) issueTokens(ctx context.Context, subject string, familyID string) (TokenPair, error) {
	now := s.now()
	accessID := uuid.NewString()
	refreshID := uuid.NewString()

	aClaims := accessClaims{
		FamilyID: familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    s.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessExpiration)),
			ID:        accessID,
		},
	}
	rClaims := refreshClaims{
		FamilyID: familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    s.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.RefreshExpiration)),
			ID:        refreshID,
		},
	}

	if s.cfg.Audience != "" {
		aClaims.Audience = jwt.ClaimStrings{s.cfg.Audience}
		rClaims.Audience = jwt.ClaimStrings{s.cfg.Audience}
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, aClaims).SignedString(s.secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: sign access token: %w", err)
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, rClaims).SignedString(s.secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: sign refresh token: %w", err)
	}

	if err := s.redis.Set(ctx, refreshFamilyKey(familyID), refreshID, s.cfg.RefreshExpiration).Err(); err != nil {
		return TokenPair{}, fmt.Errorf("auth: persist refresh family: %w", err)
	}

	return TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  aClaims.ExpiresAt.Time,
		RefreshExpiresAt: rClaims.ExpiresAt.Time,
	}, nil
}

// 1.- parseAccessClaims decodes and validates access token claims from the compact representation.
func (s *Service) parseAccessClaims(token string) (*accessClaims, error) {
	claims := &accessClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyFunc(), s.jwtOptions()...)
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// 1.- parseRefreshClaims decodes refresh token claims and ensures they remain valid.
func (s *Service) parseRefreshClaims(token string) (*refreshClaims, error) {
	claims := &refreshClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyFunc(), s.jwtOptions()...)
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// 1.- keyFunc supplies the signing secret to the JWT parser.
func (s *Service) keyFunc() jwt.Keyfunc {
	return func(_ *jwt.Token) (interface{}, error) {
		return s.secret, nil
	}
}

// 1.- jwtOptions returns reusable options for JWT parsing (audience enforcement when set).
func (s *Service) jwtOptions() []jwt.ParserOption {
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})}
	if s.cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(s.cfg.Audience))
	}
	if s.cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(s.cfg.Issuer))
	}
	return opts
}

// 1.- blacklistFamily revokes an entire refresh token family and removes its active entry.
func (s *Service) blacklistFamily(ctx context.Context, familyID string) error {
	if err := s.redis.Set(ctx, familyBlacklistKey(familyID), "1", s.cfg.RefreshExpiration).Err(); err != nil {
		return fmt.Errorf("auth: blacklist family: %w", err)
	}
	if _, err := s.redis.Del(ctx, refreshFamilyKey(familyID)).Result(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("auth: delete refresh family: %w", err)
	}
	return nil
}

// 1.- isFamilyBlacklisted checks whether a refresh token family is revoked in Redis.
func (s *Service) isFamilyBlacklisted(ctx context.Context, familyID string) (bool, error) {
	_, err := s.redis.Get(ctx, familyBlacklistKey(familyID)).Result()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return false, fmt.Errorf("auth: refresh family blacklist lookup: %w", err)
}

// 1.- refreshFamilyKey builds the Redis key storing the current refresh token identifier.
func refreshFamilyKey(familyID string) string {
	return fmt.Sprintf("auth:refresh-family:%s", familyID)
}

// 1.- familyBlacklistKey constructs the Redis key tracking revoked refresh families.
func familyBlacklistKey(familyID string) string {
	return fmt.Sprintf("auth:blacklist:family:%s", familyID)
}

// 1.- accessBlacklistKey constructs the Redis key marking revoked access tokens by JTI.
func accessBlacklistKey(jti string) string {
	return fmt.Sprintf("auth:blacklist:access:%s", jti)
}
