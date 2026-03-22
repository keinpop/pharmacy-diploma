package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"

	"github.com/lib/pq"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (username, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	err := r.db.QueryRowContext(ctx, query,
		user.Username, user.PasswordHash, string(user.Role), user.CreatedAt,
	).Scan(&user.ID)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return usecase.ErrUserExists
		}
		return fmt.Errorf("postgres.UserRepository.Create: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const query = `
		SELECT id, username, password_hash, role, created_at
		FROM users WHERE username = $1`

	user := &domain.User{}
	var role string

	err := r.db.QueryRowContext(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, usecase.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres.UserRepository.GetByUsername: %w", err)
	}

	user.Role = domain.Role(role)
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	const query = `
		SELECT id, username, password_hash, role, created_at
		FROM users WHERE id = $1`

	user := &domain.User{}
	var role string

	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, usecase.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres.UserRepository.GetByID: %w", err)
	}

	user.Role = domain.Role(role)
	return user, nil
}
