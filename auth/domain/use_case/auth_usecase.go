package use_case

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"pharma/auth/domain"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidToken    = errors.New("invalid or expired token")
)

const sessionTTL = 24 * time.Hour

type AuthUseCase struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
}

func NewAuthUseCase(userRepo UserRepository, sessionRepo SessionRepository) *AuthUseCase {
	return &AuthUseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, username, password string, role domain.Role) (*domain.User, string, error) {
	user, err := domain.NewUser(username, password, role)
	if err != nil {
		return nil, "", err
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, "", err
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	if err := uc.sessionRepo.Set(ctx, token, user.ID, sessionTTL); err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, username, password string) (*domain.User, string, error) {
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, "", ErrInvalidPassword
	}

	if !user.CheckPassword(password) {
		return nil, "", ErrInvalidPassword
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	if err := uc.sessionRepo.Set(ctx, token, user.ID, sessionTTL); err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (uc *AuthUseCase) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	userID, err := uc.sessionRepo.Get(ctx, token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, token string) error {
	return uc.sessionRepo.Delete(ctx, token)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
