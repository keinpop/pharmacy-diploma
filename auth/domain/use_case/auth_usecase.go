package use_case

import (
	"context"
	"errors"
	"time"

	"pharma/auth/app/metrics"
	"pharma/auth/domain"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidToken    = errors.New("invalid or expired token")
)

const sessionTTL = 24 * time.Hour

// AuthUseCase реализует основные сценарии аутентификации.
//
// Токены — JWT (HS256). В Redis хранится «whitelist» активных JWT id (jti),
// что позволяет:
//  1. читать роль/username прямо из payload (без обращения в Redis), и
//  2. при необходимости отзывать токен (Logout) до истечения срока жизни.
type AuthUseCase struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
	tokens      *TokenManager
}

func NewAuthUseCase(userRepo UserRepository, sessionRepo SessionRepository, tokens *TokenManager) *AuthUseCase {
	if tokens == nil {
		// Безопасный дефолт для тестов и локальных запусков.
		tokens = NewTokenManager("dev-secret-change-me", sessionTTL)
	}
	return &AuthUseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		tokens:      tokens,
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

	return uc.issueToken(ctx, user)
}

func (uc *AuthUseCase) Login(ctx context.Context, username, password string) (*domain.User, string, error) {
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		metrics.LoginAttempts.WithLabelValues("failure").Inc()
		return nil, "", ErrInvalidPassword
	}

	if !user.CheckPassword(password) {
		metrics.LoginAttempts.WithLabelValues("failure").Inc()
		return nil, "", ErrInvalidPassword
	}

	metrics.LoginAttempts.WithLabelValues("success").Inc()
	return uc.issueToken(ctx, user)
}

// ValidateToken проверяет JWT и наличие активной сессии.
// Возвращает текущие данные пользователя (имя/роль могли поменяться — берём из БД).
func (uc *AuthUseCase) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	claims, err := uc.tokens.Parse(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Сверяем, что сессия не была отозвана.
	if _, err := uc.sessionRepo.Get(ctx, claims.ID); err != nil {
		return nil, ErrInvalidToken
	}

	user, err := uc.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// Logout отзывает токен — удаляет jti из активных сессий.
func (uc *AuthUseCase) Logout(ctx context.Context, token string) error {
	claims, err := uc.tokens.Parse(token)
	if err != nil {
		// Идемпотентность: невалидный токен — уже не активен.
		return nil
	}
	if err := uc.sessionRepo.Delete(ctx, claims.ID); err != nil {
		return err
	}
	metrics.TokensRevoked.Inc()
	return nil
}

func (uc *AuthUseCase) issueToken(ctx context.Context, user *domain.User) (*domain.User, string, error) {
	token, err := uc.tokens.Generate(user)
	if err != nil {
		return nil, "", err
	}
	claims, err := uc.tokens.Parse(token)
	if err != nil {
		return nil, "", err
	}
	if err := uc.sessionRepo.Set(ctx, claims.ID, user.ID, uc.tokens.TTL()); err != nil {
		return nil, "", err
	}
	metrics.TokensIssued.Inc()
	return user, token, nil
}
