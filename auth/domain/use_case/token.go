package use_case

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pharma/auth/domain"
)

// TokenManager инкапсулирует выпуск и валидацию JWT.
// JWT хранит userID, username и роль (pharmacist | admin | manager),
// что позволяет нижестоящим сервисам сразу получать сведения о пользователе.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenManager создаёт TokenManager с указанным секретом и временем жизни токена.
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = sessionTTL
	}
	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// Claims — кастомные claims, которые попадают в JWT.
// Все downstream-сервисы (sales / inventory / analytics) могут читать username
// прямо из payload без дополнительных вызовов в auth.
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Generate выпускает новый JWT для пользователя.
func (m *TokenManager) Generate(user *domain.User) (string, error) {
	if user == nil {
		return "", errors.New("token: user is nil")
	}
	now := time.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "pharmacy-auth",
			ID:        fmt.Sprintf("%d-%d", user.ID, now.UnixNano()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("token sign: %w", err)
	}
	return signed, nil
}

// Parse валидирует подпись и срок действия и возвращает claims.
func (m *TokenManager) Parse(raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// TTL возвращает срок жизни токена.
func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}
