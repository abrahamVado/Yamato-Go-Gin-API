package controllers

import (
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	mrand "math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// 1.- PhoneVerification models a DB row for JSON responses.
type PhoneVerification struct {
	ID        int64     `json:"id"`
	Phone     string    `json:"phone"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 2.- PhoneVerificationController exposes HTTP handlers for phone verification.
type PhoneVerificationController struct {
	db *sql.DB
}

// 3.- NewPhoneVerificationController creates a new instance of PhoneVerificationController.
func NewPhoneVerificationController(db *sql.DB) PhoneVerificationController {
	return PhoneVerificationController{db: db}
}

// 4.- RequestCode handles POST /api/phone-verifications.
//     Body: { "phone": "+52...", "name": "John Doe" }.
func (c PhoneVerificationController) RequestCode(ctx *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Name  string `json:"name"  binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone and name are required"})
		return
	}

	code := generateCode(6)
	expiresAt := time.Now().Add(10 * time.Minute)

	const q = `
INSERT INTO phone_verifications (phone, code, status, name, expires_at)
VALUES ($1, $2, 'pending', $3, $4)
RETURNING id, created_at, updated_at`

	var (
		id        int64
		createdAt time.Time
		updatedAt time.Time
	)

	if err := c.db.QueryRowContext(ctx.Request.Context(), q,
		req.Phone,
		code,
		req.Name,
		expiresAt,
	).Scan(&id, &createdAt, &updatedAt); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_create_verification",
			"detail": err.Error(), // debug
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"id":         id,
		"phone":      req.Phone,
		"name":       req.Name,
		"status":     "pending",
		"code":       code, // para que puedas leerlo y enviarlo manualmente
		"expires_at": expiresAt,
		"created_at": createdAt,
		"updated_at": updatedAt,
	})
}

// 5.- ConfirmCode handles POST /api/phone-verifications/confirm.
//     Body: { "phone": "+52...", "code": "123456" }.
func (c PhoneVerificationController) ConfirmCode(ctx *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Code  string `json:"code"  binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Code == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone and code are required"})
		return
	}

	const selectQ = `
SELECT id, status, expires_at
FROM phone_verifications
WHERE phone = $1 AND code = $2
ORDER BY created_at DESC
LIMIT 1`

	var (
		id        int64
		status    string
		expiresAt time.Time
	)

	err := c.db.QueryRowContext(ctx.Request.Context(), selectQ, req.Phone, req.Code).Scan(&id, &status, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusConflict, gin.H{"error": "invalid_or_expired_code"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_verify_code",
			"detail": err.Error(),
		})
		return
	}

	if status != "pending" || time.Now().After(expiresAt) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "invalid_or_expired_code"})
		return
	}

	// 5.1.- Mark this verification as verified.
	const updateQ = `
UPDATE phone_verifications
SET status = 'verified', updated_at = NOW()
WHERE id = $1`

	if _, err := c.db.ExecContext(ctx.Request.Context(), updateQ, id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_mark_verified",
			"detail": err.Error(),
		})
		return
	}

	// 5.2.- Generate a long-lived device token for passwordless usage.
	deviceToken, err := generateDeviceToken()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_generate_device_token",
			"detail": err.Error(),
		})
		return
	}

	const insertTokenQ = `
INSERT INTO device_tokens (phone, token)
VALUES ($1, $2)
RETURNING id, created_at, last_used_at`

	var (
		tokenID   int64
		createdAt time.Time
		lastUsed  time.Time
	)

	if err := c.db.QueryRowContext(ctx.Request.Context(), insertTokenQ,
		req.Phone,
		deviceToken,
	).Scan(&tokenID, &createdAt, &lastUsed); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_persist_device_token",
			"detail": err.Error(),
		})
		return
	}

	// 5.3.- Return success + device_token to the client.
	ctx.JSON(http.StatusOK, gin.H{
		"phone":        req.Phone,
		"verified":     true,
		"device_token": deviceToken,
	})
}

// 6.- ListUnverified handles GET /v1/phone-verifications/unverified.
func (c PhoneVerificationController) ListUnverified(ctx *gin.Context) {
	limit := parsePositiveInt(ctx.Query("limit"), 50)
	offset := parsePositiveInt(ctx.Query("offset"), 0)

	const q = `
SELECT id, phone, code, status, name, expires_at, created_at, updated_at
FROM phone_verifications
WHERE status = 'pending' AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`

	rows, err := c.db.QueryContext(ctx.Request.Context(), q, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_list_verifications",
			"detail": err.Error(),
		})
		return
	}
	defer rows.Close()

	var items []PhoneVerification
	for rows.Next() {
		var v PhoneVerification
		if err := rows.Scan(
			&v.ID,
			&v.Phone,
			&v.Code,
			&v.Status,
			&v.Name,
			&v.ExpiresAt,
			&v.CreatedAt,
			&v.UpdatedAt,
		); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":  "could_not_parse_verifications",
				"detail": err.Error(),
			})
			return
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":  "could_not_list_verifications",
			"detail": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"count":  len(items),
	})
}

// 7.- parsePositiveInt converts a query parameter to a positive int with fallback.
func parsePositiveInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// 8.- generateCode returns a numeric code of the given length.
func generateCode(length int) string {
	if length <= 0 {
		length = 6
	}
	n := mrand.Intn(900000) + 100000
	s := strconv.Itoa(n)
	if length == 6 {
		return s
	}
	if len(s) > length {
		return s[:length]
	}
	for len(s) < length {
		s = "0" + s
	}
	return s
}

// 9.- generateDeviceToken returns a cryptographically-strong random token.
func generateDeviceToken() (string, error) {
	// 32 bytes = 256 bits -> 64 hex chars
	buf := make([]byte, 32)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
