package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appgrpc "pharma/auth/app/grpc"
	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"
	authpb "pharma/auth/gen/auth"
)

// mock

type mockUseCase struct{ mock.Mock }

func (m *mockUseCase) Register(ctx context.Context, username, password string, role domain.Role) (*domain.User, string, error) {
	args := m.Called(ctx, username, password, role)
	if u, ok := args.Get(0).(*domain.User); ok {
		return u, args.String(1), args.Error(2)
	}
	return nil, "", args.Error(2)
}

func (m *mockUseCase) Login(ctx context.Context, username, password string) (*domain.User, string, error) {
	args := m.Called(ctx, username, password)
	if u, ok := args.Get(0).(*domain.User); ok {
		return u, args.String(1), args.Error(2)
	}
	return nil, "", args.Error(2)
}

func (m *mockUseCase) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	args := m.Called(ctx, token)
	if u, ok := args.Get(0).(*domain.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUseCase) Logout(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

// helpers

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	s, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return s.Code()
}

func makeHandler(uc appgrpc.AuthUseCasePort) *appgrpc.AuthHandler {
	return appgrpc.NewAuthHandler(uc)
}

// Register ─

func TestAuthHandler_Register(t *testing.T) {
	t.Parallel()
	validUser := &domain.User{ID: 1, Username: "alice", Role: domain.RolePharmacist}

	tests := []struct {
		name     string
		req      *authpb.RegisterRequest
		setup    func(*mockUseCase)
		wantCode codes.Code
		wantID   int64
	}{
		{
			name: "success",
			req:  &authpb.RegisterRequest{Username: "alice", Password: "pass", Role: "pharmacist"},
			setup: func(m *mockUseCase) {
				m.On("Register", mock.Anything, "alice", "pass", domain.RolePharmacist).
					Return(validUser, "tok", nil)
			},
			wantCode: codes.OK,
			wantID:   1,
		},
		{
			name:     "invalid role",
			req:      &authpb.RegisterRequest{Username: "alice", Password: "pass", Role: "superuser"},
			setup:    func(m *mockUseCase) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "user already exists",
			req:  &authpb.RegisterRequest{Username: "alice", Password: "pass", Role: "admin"},
			setup: func(m *mockUseCase) {
				m.On("Register", mock.Anything, "alice", "pass", domain.RoleAdmin).
					Return(nil, "", usecase.ErrUserExists)
			},
			wantCode: codes.AlreadyExists,
		},
		{
			name: "empty username from usecase",
			req:  &authpb.RegisterRequest{Username: "", Password: "pass", Role: "pharmacist"},
			setup: func(m *mockUseCase) {
				m.On("Register", mock.Anything, "", "pass", domain.RolePharmacist).
					Return(nil, "", domain.ErrEmptyUsername)
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "internal error",
			req:  &authpb.RegisterRequest{Username: "alice", Password: "pass", Role: "manager"},
			setup: func(m *mockUseCase) {
				m.On("Register", mock.Anything, "alice", "pass", domain.RoleManager).
					Return(nil, "", errors.New("db down"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := &mockUseCase{}
			tt.setup(uc)
			h := makeHandler(uc)

			resp, err := h.Register(context.Background(), tt.req)

			assert.Equal(t, tt.wantCode, grpcCode(err))
			if tt.wantCode == codes.OK {
				assert.NotNil(t, resp)
				assert.Equal(t, tt.wantID, resp.UserId)
			}
			uc.AssertExpectations(t)
		})
	}
}

// Login

func TestAuthHandler_Login(t *testing.T) {
	t.Parallel()
	validUser := &domain.User{ID: 2, Username: "bob", Role: domain.RoleAdmin}

	tests := []struct {
		name     string
		req      *authpb.LoginRequest
		setup    func(*mockUseCase)
		wantCode codes.Code
		wantRole string
	}{
		{
			name: "success",
			req:  &authpb.LoginRequest{Username: "bob", Password: "secret"},
			setup: func(m *mockUseCase) {
				m.On("Login", mock.Anything, "bob", "secret").
					Return(validUser, "tok42", nil)
			},
			wantCode: codes.OK,
			wantRole: "admin",
		},
		{
			name: "wrong credentials",
			req:  &authpb.LoginRequest{Username: "bob", Password: "wrong"},
			setup: func(m *mockUseCase) {
				m.On("Login", mock.Anything, "bob", "wrong").
					Return(nil, "", usecase.ErrInvalidPassword)
			},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := &mockUseCase{}
			tt.setup(uc)
			h := makeHandler(uc)

			resp, err := h.Login(context.Background(), tt.req)

			assert.Equal(t, tt.wantCode, grpcCode(err))
			if tt.wantCode == codes.OK {
				assert.Equal(t, tt.wantRole, resp.Role)
			}
			uc.AssertExpectations(t)
		})
	}
}

// ValidateToken

func TestAuthHandler_ValidateToken(t *testing.T) {
	t.Parallel()
	validUser := &domain.User{ID: 3, Username: "carol", Role: domain.RolePharmacist}

	tests := []struct {
		name     string
		token    string
		setup    func(*mockUseCase)
		wantCode codes.Code
	}{
		{
			name:  "valid token",
			token: "goodtoken",
			setup: func(m *mockUseCase) {
				m.On("ValidateToken", mock.Anything, "goodtoken").Return(validUser, nil)
			},
			wantCode: codes.OK,
		},
		{
			name:  "expired token",
			token: "badtoken",
			setup: func(m *mockUseCase) {
				m.On("ValidateToken", mock.Anything, "badtoken").Return(nil, usecase.ErrInvalidToken)
			},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := &mockUseCase{}
			tt.setup(uc)
			h := makeHandler(uc)

			resp, err := h.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: tt.token})

			assert.Equal(t, tt.wantCode, grpcCode(err))
			if tt.wantCode == codes.OK {
				assert.Equal(t, "carol", resp.Username)
			}
			uc.AssertExpectations(t)
		})
	}
}

// Logout ─

func TestAuthHandler_Logout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		token    string
		setup    func(*mockUseCase)
		wantCode codes.Code
	}{
		{
			name:  "success",
			token: "sometoken",
			setup: func(m *mockUseCase) {
				m.On("Logout", mock.Anything, "sometoken").Return(nil)
			},
			wantCode: codes.OK,
		},
		{
			name:  "already invalid",
			token: "expiredtoken",
			setup: func(m *mockUseCase) {
				m.On("Logout", mock.Anything, "expiredtoken").Return(usecase.ErrInvalidToken)
			},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := &mockUseCase{}
			tt.setup(uc)
			h := makeHandler(uc)

			resp, err := h.Logout(context.Background(), &authpb.LogoutRequest{Token: tt.token})

			assert.Equal(t, tt.wantCode, grpcCode(err))
			if tt.wantCode == codes.OK {
				assert.True(t, resp.Success)
			}
			uc.AssertExpectations(t)
		})
	}
}
