package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pharma/auth/domain"
	usecase "pharma/auth/domain/use_case"
	authpb "pharma/auth/gen/auth/auth"
)

type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	uc AuthUseCasePort
}

func NewAuthHandler(uc AuthUseCasePort) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	role := domain.Role(req.GetRole())
	if !role.IsValid() {
		return nil, status.Errorf(codes.InvalidArgument, "invalid role: %s", req.GetRole())
	}

	user, token, err := h.uc.Register(ctx, req.GetUsername(), req.GetPassword(), role)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &authpb.RegisterResponse{UserId: user.ID, Token: token}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	user, token, err := h.uc.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &authpb.LoginResponse{UserId: user.ID, Token: token, Role: string(user.Role)}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	user, err := h.uc.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &authpb.ValidateTokenResponse{
		UserId:   user.ID,
		Username: user.Username,
		Role:     string(user.Role),
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	if err := h.uc.Logout(ctx, req.GetToken()); err != nil {
		return nil, mapDomainError(err)
	}
	return &authpb.LogoutResponse{Success: true}, nil
}

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, usecase.ErrInvalidPassword):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, usecase.ErrUserExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, usecase.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrInvalidRole),
		errors.Is(err, domain.ErrEmptyUsername),
		errors.Is(err, domain.ErrEmptyPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
