package use_case_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"
)

// — mocks ─

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if u, ok := args.Get(0).(*domain.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	args := m.Called(ctx, id)
	if u, ok := args.Get(0).(*domain.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

type mockSessionRepo struct{ mock.Mock }

func (m *mockSessionRepo) Set(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	return m.Called(ctx, token, userID, ttl).Error(0)
}

func (m *mockSessionRepo) Get(ctx context.Context, token string) (int64, error) {
	args := m.Called(ctx, token)
	id, _ := args.Get(0).(int64)
	return id, args.Error(1)
}

func (m *mockSessionRepo) Delete(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func newTestTokenManager() *usecase.TokenManager {
	return usecase.NewTokenManager("test-secret", time.Hour)
}

func newTestUC(ur *mockUserRepo, sr *mockSessionRepo) *usecase.AuthUseCase {
	return usecase.NewAuthUseCase(ur, sr, newTestTokenManager())
}

// — Register —

func TestAuthUseCase_Register(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		username string
		password string
		role     domain.Role
		setup    func(*mockUserRepo, *mockSessionRepo)
		wantErr  error
	}{
		{
			name:     "success",
			username: "alice",
			password: "password123",
			role:     domain.RolePharmacist,
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				ur.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
					Run(func(args mock.Arguments) {
						args.Get(1).(*domain.User).ID = 10
					}).
					Return(nil)
				sr.On("Set", mock.Anything, mock.AnythingOfType("string"), int64(10), mock.Anything).
					Return(nil)
			},
		},
		{
			name:     "empty username",
			username: "",
			password: "password123",
			role:     domain.RolePharmacist,
			setup:    func(ur *mockUserRepo, sr *mockSessionRepo) {},
			wantErr:  domain.ErrEmptyUsername,
		},
		{
			name:     "empty password",
			username: "alice",
			password: "",
			role:     domain.RolePharmacist,
			setup:    func(ur *mockUserRepo, sr *mockSessionRepo) {},
			wantErr:  domain.ErrEmptyPassword,
		},
		{
			name:     "invalid role",
			username: "alice",
			password: "password123",
			role:     domain.Role("superuser"),
			setup:    func(ur *mockUserRepo, sr *mockSessionRepo) {},
			wantErr:  domain.ErrInvalidRole,
		},
		{
			name:     "user already exists",
			username: "alice",
			password: "password123",
			role:     domain.RoleAdmin,
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				ur.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(usecase.ErrUserExists)
			},
			wantErr: usecase.ErrUserExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := newTestUC(ur, sr)

			user, token, err := uc.Register(context.Background(), tt.username, tt.password, tt.role)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.NotEmpty(t, token)
				assert.Equal(t, int64(10), user.ID)
				// Токен должен парситься нашим менеджером.
				claims, err := newTestTokenManager().Parse(token)
				require.NoError(t, err)
				assert.Equal(t, "alice", claims.Username)
				assert.Equal(t, string(tt.role), claims.Role)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// — Login —

func TestAuthUseCase_Login(t *testing.T) {
	t.Parallel()
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.DefaultCost)
	existingUser := &domain.User{
		ID:           5,
		Username:     "bob",
		PasswordHash: string(hash),
		Role:         domain.RoleManager,
	}

	tests := []struct {
		name     string
		username string
		password string
		setup    func(*mockUserRepo, *mockSessionRepo)
		wantErr  error
	}{
		{
			name:     "success",
			username: "bob",
			password: "correctpass",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				ur.On("GetByUsername", mock.Anything, "bob").Return(existingUser, nil)
				sr.On("Set", mock.Anything, mock.AnythingOfType("string"), int64(5), mock.Anything).
					Return(nil)
			},
		},
		{
			name:     "user not found returns ErrInvalidPassword",
			username: "ghost",
			password: "anypass",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				ur.On("GetByUsername", mock.Anything, "ghost").Return(nil, usecase.ErrUserNotFound)
			},
			wantErr: usecase.ErrInvalidPassword,
		},
		{
			name:     "wrong password",
			username: "bob",
			password: "wrongpass",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				ur.On("GetByUsername", mock.Anything, "bob").Return(existingUser, nil)
			},
			wantErr: usecase.ErrInvalidPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := newTestUC(ur, sr)

			user, token, err := uc.Login(context.Background(), tt.username, tt.password)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.NotEmpty(t, token)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// — ValidateToken —

func TestAuthUseCase_ValidateToken(t *testing.T) {
	t.Parallel()
	existingUser := &domain.User{ID: 7, Username: "carol", Role: domain.RoleAdmin}

	tm := newTestTokenManager()
	validToken, err := tm.Generate(existingUser)
	require.NoError(t, err)
	validClaims, err := tm.Parse(validToken)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		setup   func(*mockUserRepo, *mockSessionRepo)
		wantErr error
	}{
		{
			name:  "valid token",
			token: validToken,
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, validClaims.ID).Return(int64(7), nil)
				ur.On("GetByID", mock.Anything, int64(7)).Return(existingUser, nil)
			},
		},
		{
			name:    "malformed token",
			token:   "not-a-jwt",
			setup:   func(ur *mockUserRepo, sr *mockSessionRepo) {},
			wantErr: usecase.ErrInvalidToken,
		},
		{
			name:  "session revoked",
			token: validToken,
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, validClaims.ID).Return(int64(0), usecase.ErrInvalidToken)
			},
			wantErr: usecase.ErrInvalidToken,
		},
		{
			name:  "user deleted after session created",
			token: validToken,
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, validClaims.ID).Return(int64(7), nil)
				ur.On("GetByID", mock.Anything, int64(7)).Return(nil, usecase.ErrUserNotFound)
			},
			wantErr: usecase.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := usecase.NewAuthUseCase(ur, sr, tm)

			user, err := uc.ValidateToken(context.Background(), tt.token)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.Equal(t, existingUser.ID, user.ID)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// — Logout —

func TestAuthUseCase_Logout(t *testing.T) {
	t.Parallel()
	user := &domain.User{ID: 5, Username: "bob", Role: domain.RoleAdmin}
	tm := newTestTokenManager()
	validToken, err := tm.Generate(user)
	require.NoError(t, err)
	validClaims, err := tm.Parse(validToken)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		setup   func(*mockSessionRepo)
		wantErr error
	}{
		{
			name:  "success",
			token: validToken,
			setup: func(sr *mockSessionRepo) {
				sr.On("Delete", mock.Anything, validClaims.ID).Return(nil)
			},
		},
		{
			name:  "redis error propagates",
			token: validToken,
			setup: func(sr *mockSessionRepo) {
				sr.On("Delete", mock.Anything, validClaims.ID).Return(errors.New("boom"))
			},
			wantErr: errors.New("boom"),
		},
		{
			name:  "invalid token is silently accepted",
			token: "junk",
			setup: func(sr *mockSessionRepo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(sr)
			uc := usecase.NewAuthUseCase(ur, sr, tm)

			err := uc.Logout(context.Background(), tt.token)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			sr.AssertExpectations(t)
		})
	}
}
