package users

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
)

// Store implements authhttp.UserStore backed by Postgres.
type Store struct {
	db *sql.DB
}

// NewStore constructs a Store for the provided database handle.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new user record and returns the created user with its ID populated.
func (s *Store) Create(ctx context.Context, user authhttp.User) (authhttp.User, error) {
	const q = `
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id`

	var id int64
	if err := s.db.QueryRowContext(ctx, q,
		user.Email,
		user.PasswordHash,
	).Scan(&id); err != nil {
		return authhttp.User{}, fmt.Errorf("create user: %w", err)
	}

	user.ID = strconv.FormatInt(id, 10)
	return user, nil
}

// FindByEmail retrieves a user by email. It returns a zero-value user and nil error when not found.
func (s *Store) FindByEmail(ctx context.Context, email string) (authhttp.User, error) {
	const q = `
SELECT id, email, password_hash
FROM users
WHERE email = $1
LIMIT 1`

	var (
		id           int64
		u            authhttp.User
		passwordHash string
	)

	err := s.db.QueryRowContext(ctx, q, email).Scan(
		&id,
		&u.Email,
		&passwordHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
            // mimic in-memory store semantics: not found = zero user, nil error
			return authhttp.User{}, nil
		}
		return authhttp.User{}, fmt.Errorf("find user by email: %w", err)
	}

	u.ID = strconv.FormatInt(id, 10)
	u.PasswordHash = passwordHash

	return u, nil
}

// FindByID retrieves a user by ID. It returns a zero-value user and nil error when not found.
func (s *Store) FindByID(ctx context.Context, id string) (authhttp.User, error) {
	const q = `
SELECT id, email, password_hash
FROM users
WHERE id = $1
LIMIT 1`

	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return authhttp.User{}, fmt.Errorf("invalid user id %q: %w", id, err)
	}

	var (
		dbID         int64
		u            authhttp.User
		passwordHash string
	)

	err = s.db.QueryRowContext(ctx, q, intID).Scan(
		&dbID,
		&u.Email,
		&passwordHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return authhttp.User{}, nil
		}
		return authhttp.User{}, fmt.Errorf("find user by id: %w", err)
	}

	u.ID = strconv.FormatInt(dbID, 10)
	u.PasswordHash = passwordHash

	return u, nil
}
