package auth

import (
	"context"

	"google.golang.org/grpc"
)

type AuthenticationProvider interface {
	// Authenticate tries to authenticate the token
	Authenticate(ctx context.Context, token string) (*AuthResponse, error)
}

type AuthResponse struct {
	Known    bool              `json:"known"`
	Username string            `json:"username"`
	Metadata map[string]string `json:"metadata"`
}

type Interceptor interface {
	Unary() grpc.UnaryServerInterceptor
	Stream() grpc.StreamServerInterceptor
}
