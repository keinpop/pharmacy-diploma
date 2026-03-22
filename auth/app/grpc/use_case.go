package grpc

import (
	"context"

	"pharma/auth/domain"
)

// AuthUseCasePort — интерфейс, который реализует *usecase.AuthUseCase.
// Нужен для подмены в тестах.
type AuthUseCasePort interface {
	Register(ctx context.Context, username, password string, role domain.Role) (*domain.User, string, error)
	Login(ctx context.Context, username, password string) (*domain.User, string, error)
	ValidateToken(ctx context.Context, token string) (*domain.User, error)
	Logout(ctx context.Context, token string) error
}
