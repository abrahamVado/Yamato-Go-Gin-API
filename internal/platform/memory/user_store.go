package memory

import (
	"context"
	"errors"
	"strings"
	"sync"

	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
)

// 1.- UserStore provides a thread-safe in-memory implementation of the auth user repository.
type UserStore struct {
	mu      sync.RWMutex
	users   map[string]authhttp.User
	byEmail map[string]string
}

// 1.- NewUserStore prepares an empty store ready for dependency injection.
func NewUserStore() *UserStore {
	return &UserStore{users: map[string]authhttp.User{}, byEmail: map[string]string{}}
}

// 1.- Create persists the provided user, enforcing unique email constraints.
func (s *UserStore) Create(_ context.Context, user authhttp.User) (authhttp.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(user.Email))
	if key == "" {
		return authhttp.User{}, errors.New("memory user store: email is required")
	}
	if _, exists := s.byEmail[key]; exists {
		return authhttp.User{}, errors.New("memory user store: email already exists")
	}

	copied := user
	copied.Email = key
	s.users[user.ID] = copied
	s.byEmail[key] = user.ID
	return copied, nil
}

// 1.- FindByEmail retrieves a stored user by e-mail address.
func (s *UserStore) FindByEmail(_ context.Context, email string) (authhttp.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return authhttp.User{}, authhttp.ErrUserNotFound
	}
	return s.users[id], nil
}

// 1.- FindByID retrieves a stored user using the identifier.
func (s *UserStore) FindByID(_ context.Context, id string) (authhttp.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return authhttp.User{}, authhttp.ErrUserNotFound
	}
	return user, nil
}
