package websocket

import (
	"context"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- ServiceAuthenticator wraps the core auth service to satisfy the WebSocket authenticator.
type ServiceAuthenticator struct {
	service *internalauth.Service
}

// 1.- NewServiceAuthenticator instantiates the adapter with the provided auth service pointer.
func NewServiceAuthenticator(service *internalauth.Service) ServiceAuthenticator {
	return ServiceAuthenticator{service: service}
}

// 1.- Authenticate validates the bearer token and returns a principal scoped to the subject.
func (a ServiceAuthenticator) Authenticate(ctx context.Context, token string) (internalauth.Principal, error) {
	claims, err := a.service.ValidateAccessToken(ctx, token)
	if err != nil {
		return internalauth.Principal{}, err
	}
	return internalauth.Principal{Subject: claims.Subject}, nil
}
