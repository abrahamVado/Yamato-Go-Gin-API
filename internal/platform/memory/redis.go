package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 1.- entry stores the cached value alongside its optional expiration deadline.
type entry struct {
	value     string
	expiresAt time.Time
}

// 1.- Redis emulates the subset of Redis commands required by the auth service in memory.
type Redis struct {
	mu     sync.RWMutex
	values map[string]entry
}

// 1.- NewRedis constructs an in-memory Redis replacement with empty state.
func NewRedis() *Redis {
	return &Redis{values: map[string]entry{}}
}

// 1.- Set stores the provided value and applies the optional expiration window.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	// 2.- Coerce the value into a string so auth tokens retain fidelity.
	str := fmt.Sprint(value)
	// 3.- Compute the expiration deadline when a TTL is supplied.
	deadline := time.Time{}
	if expiration > 0 {
		deadline = time.Now().Add(expiration)
	}

	r.mu.Lock()
	r.values[key] = entry{value: str, expiresAt: deadline}
	r.mu.Unlock()

	// 4.- Mirror go-redis return behaviour by emitting an OK status command.
	cmd := redis.NewStatusCmd(ctx, "set", key, value, expiration)
	cmd.SetVal("OK")
	return cmd
}

// 1.- Get retrieves a stored value, returning redis.Nil for missing or expired entries.
func (r *Redis) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "get", key)

	r.mu.RLock()
	value, ok := r.values[key]
	r.mu.RUnlock()
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	if !value.expiresAt.IsZero() && time.Now().After(value.expiresAt) {
		r.mu.Lock()
		delete(r.values, key)
		r.mu.Unlock()
		cmd.SetErr(redis.Nil)
		return cmd
	}

	cmd.SetVal(value.value)
	return cmd
}

// 1.- Del removes the provided keys and returns the number of deleted entries.
func (r *Redis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, append([]interface{}{"del"}, stringSliceToInterface(keys)...)...)

	removed := int64(0)
	r.mu.Lock()
	for _, key := range keys {
		if _, exists := r.values[key]; exists {
			delete(r.values, key)
			removed++
		}
	}
	r.mu.Unlock()

	cmd.SetVal(removed)
	return cmd
}

// 1.- stringSliceToInterface converts a slice of strings to a slice of empty interfaces.
func stringSliceToInterface(values []string) []interface{} {
	converted := make([]interface{}, len(values))
	for i, value := range values {
		converted[i] = value
	}
	return converted
}
