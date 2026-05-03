package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharma/auth/adapter/postgres"
	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"
)

func newMockDB(t *testing.T) (*postgres.UserRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return postgres.NewUserRepository(db), mock
}

// Create ─

func TestUserRepository_Create(t *testing.T) {
	t.Parallel()
	validUser := &domain.User{
		Username:     "alice",
		PasswordHash: "hash",
		Role:         domain.RolePharmacist,
		CreatedAt:    time.Now(),
	}

	tests := []struct {
		name    string
		user    *domain.User
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{
			name: "success",
			user: validUser,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`INSERT INTO users`).
					WithArgs(validUser.Username, validUser.PasswordHash, string(validUser.Role), validUser.CreatedAt).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			wantErr: nil,
		},
		{
			name: "duplicate username",
			user: validUser,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`INSERT INTO users`).
					WithArgs(validUser.Username, validUser.PasswordHash, string(validUser.Role), validUser.CreatedAt).
					WillReturnError(&pq.Error{Code: "23505"})
			},
			wantErr: usecase.ErrUserExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newMockDB(t)
			tt.setup(mock)

			err := repo.Create(context.Background(), tt.user)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(1), tt.user.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// GetByUsername─

func TestUserRepository_GetByUsername(t *testing.T) {
	t.Parallel()
	cols := []string{"id", "username", "password_hash", "role", "created_at"}
	now := time.Now()

	tests := []struct {
		name         string
		username     string
		setup        func(sqlmock.Sqlmock)
		wantErr      error
		wantUsername string
	}{
		{
			name:     "found",
			username: "alice",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT .+ FROM users WHERE username`).
					WithArgs("alice").
					WillReturnRows(sqlmock.NewRows(cols).AddRow(1, "alice", "hash", "pharmacist", now))
			},
			wantErr:      nil,
			wantUsername: "alice",
		},
		{
			name:     "not found",
			username: "ghost",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT .+ FROM users WHERE username`).
					WithArgs("ghost").
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wantErr: usecase.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newMockDB(t)
			tt.setup(mock)

			user, err := repo.GetByUsername(context.Background(), tt.username)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantUsername, user.Username)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// GetByID

func TestUserRepository_GetByID(t *testing.T) {
	t.Parallel()
	cols := []string{"id", "username", "password_hash", "role", "created_at"}
	now := time.Now()

	tests := []struct {
		name    string
		id      int64
		setup   func(sqlmock.Sqlmock)
		wantErr error
		wantID  int64
	}{
		{
			name: "found",
			id:   42,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT .+ FROM users WHERE id`).
					WithArgs(int64(42)).
					WillReturnRows(sqlmock.NewRows(cols).AddRow(42, "bob", "hash", "admin", now))
			},
			wantErr: nil,
			wantID:  42,
		},
		{
			name: "not found",
			id:   999,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT .+ FROM users WHERE id`).
					WithArgs(int64(999)).
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wantErr: usecase.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newMockDB(t)
			tt.setup(mock)

			user, err := repo.GetByID(context.Background(), tt.id)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, user.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
