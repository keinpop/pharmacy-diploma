package use_case_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

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
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockSessionRepo) Delete(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestAuthUseCase_Register(t *testing.T) {
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
			wantErr: nil,
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
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := usecase.NewAuthUseCase(ur, sr)

			user, token, err := uc.Register(context.Background(), tt.username, tt.password, tt.role)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.NotEmpty(t, token)
				assert.Equal(t, int64(10), user.ID)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestAuthUseCase_Login(t *testing.T) {
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
			wantErr: nil,
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
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := usecase.NewAuthUseCase(ur, sr)

			user, token, err := uc.Login(context.Background(), tt.username, tt.password)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.NotEmpty(t, token)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── ValidateToken ─────────────────────────────────────────────────────────────

func TestAuthUseCase_ValidateToken(t *testing.T) {
	existingUser := &domain.User{ID: 7, Username: "carol", Role: domain.RoleAdmin}

	tests := []struct {
		name    string
		token   string
		setup   func(*mockUserRepo, *mockSessionRepo)
		wantErr error
	}{
		{
			name:  "valid token",
			token: "validtoken",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, "validtoken").Return(int64(7), nil)
				ur.On("GetByID", mock.Anything, int64(7)).Return(existingUser, nil)
			},
			wantErr: nil,
		},
		{
			name:  "token not in redis",
			token: "expired",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, "expired").Return(int64(0), usecase.ErrInvalidToken)
			},
			wantErr: usecase.ErrInvalidToken,
		},
		{
			name:  "user deleted after session created",
			token: "orphan",
			setup: func(ur *mockUserRepo, sr *mockSessionRepo) {
				sr.On("Get", mock.Anything, "orphan").Return(int64(99), nil)
				ur.On("GetByID", mock.Anything, int64(99)).Return(nil, usecase.ErrUserNotFound)
			},
			wantErr: usecase.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(ur, sr)
			uc := usecase.NewAuthUseCase(ur, sr)

			user, err := uc.ValidateToken(context.Background(), tt.token)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, existingUser.ID, user.ID)
			}
			ur.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── Logout ───────────────────────────────────────────────────────────────────

func TestAuthUseCase_Logout(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		setup   func(*mockSessionRepo)
		wantErr error
	}{
		{
			name:  "success",
			token: "activetoken",
			setup: func(sr *mockSessionRepo) {
				sr.On("Delete", mock.Anything, "activetoken").Return(nil)
			},
			wantErr: nil,
		},
		{
			name:  "redis error",
			token: "token",
			setup: func(sr *mockSessionRepo) {
				sr.On("Delete", mock.Anything, "token").Return(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ur := &mockUserRepo{}
			sr := &mockSessionRepo{}
			tt.setup(sr)
			uc := usecase.NewAuthUseCase(ur, sr)

			err := uc.Logout(context.Background(), tt.token)

			assert.ErrorIs(t, err, tt.wantErr)
			sr.AssertExpectations(t)
		})
	}
}
