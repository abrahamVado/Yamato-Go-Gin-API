package authorization

import (
	"errors"
	"sync"

	"github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- ErrForbidden indicates that a gate check rejected the current principal.
var ErrForbidden = errors.New("authorization: forbidden")

// 1.- Gate declares the role and permission requirements for a protected handler.
type Gate struct {
	AnyRoles       []string
	AllPermissions []string
}

// 1.- Policy caches compiled permission sets and evaluates gates for principals.
type Policy struct {
	mu          sync.RWMutex
	permissions map[string]map[string]struct{}
}

// 1.- NewPolicy constructs a Policy with empty caches ready for use by middleware.
func NewPolicy() *Policy {
	return &Policy{
		permissions: make(map[string]map[string]struct{}),
	}
}

// 1.- Authorize checks whether the supplied principal satisfies the provided gate.
func (p *Policy) Authorize(principal auth.Principal, gate Gate) error {
	//2.- Enforce any role requirement by walking the candidate list.
	if len(gate.AnyRoles) > 0 {
		allowed := false
		for _, role := range gate.AnyRoles {
			if principal.HasRole(role) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrForbidden
		}
	}

	//2.- Enforce all required permissions using the cached permission map.
	if len(gate.AllPermissions) > 0 {
		permSet := p.permissionSet(principal)
		for _, permission := range gate.AllPermissions {
			if _, ok := permSet[permission]; !ok {
				return ErrForbidden
			}
		}
	}

	return nil
}

// 1.- Invalidate clears the cached permission set for the given principal subject.
func (p *Policy) Invalidate(subject string) {
	//2.- Remove the cache entry so the next check rebuilds it from the principal payload.
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.permissions, subject)
}

// 1.- permissionSet converts the principal's permissions slice into a cached lookup map.
func (p *Policy) permissionSet(principal auth.Principal) map[string]struct{} {
	//2.- Attempt a fast read through the cache without blocking writers.
	p.mu.RLock()
	cached, ok := p.permissions[principal.Subject]
	p.mu.RUnlock()
	if ok {
		return cached
	}

	//2.- Build a new lookup map from the principal's permissions slice.
	built := make(map[string]struct{}, len(principal.Permissions))
	for _, permission := range principal.Permissions {
		built[permission] = struct{}{}
	}

	//2.- Store the map for subsequent requests before returning it.
	p.mu.Lock()
	p.permissions[principal.Subject] = built
	p.mu.Unlock()

	return built
}
