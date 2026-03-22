package use_case

import (
	"context"
	"time"

	"pharma/auth/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

type SessionRepository interface {
	Set(ctx context.Context, token string, userID int64, ttl time.Duration) error
	Get(ctx context.Context, token string) (int64, error)
	Delete(ctx context.Context, token string) error
}
