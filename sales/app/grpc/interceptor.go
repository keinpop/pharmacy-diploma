package grpc

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// methodRoles — какие роли допущены к каждому методу.
// Сервисный токен (внутренние вызовы между микросервисами) учитывается отдельно.
var methodRoles = map[string][]string{
	"/sales.SalesService/CreateSale": {"pharmacist", "admin"},
	"/sales.SalesService/GetSale":    {"pharmacist", "admin", "manager"},
	"/sales.SalesService/ListSales":  {"pharmacist", "admin", "manager"},
}

// contextKey — типизированный ключ для значений в context, чтобы не пересекаться
// с ключами других пакетов (требование линтера / vet).
type contextKey string

const (
	authTokenKey    contextKey = "auth_token"
	authUsernameKey contextKey = "auth_username"
	authRoleKey     contextKey = "auth_role"
	authUserIDKey   contextKey = "auth_user_id"
)

// AuthInterceptor валидирует JWT-токен через Auth-сервис, проверяет роль
// и кладёт username/role/user_id вызывающего в context.
// Ниже по стеку (в use-case) username читается из context и сохраняется в Sale.
func AuthInterceptor(serviceToken string, authClient *AuthClient, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		allowed, ok := methodRoles[info.FullMethod]
		if !ok {
			return handler(ctx, req)
		}

		md, _ := metadata.FromIncomingContext(ctx)
		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}
		token := strings.TrimPrefix(tokens[0], "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")

		// Межсервисный вызов по service-токену.
		if token == serviceToken {
			if hasRole(allowed, "service") {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		userID, username, role, err := authClient.ValidateToken(ctx, token)
		if err != nil {
			logger.Warn("token validation failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
			)
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		if !hasRole(allowed, role) {
			logger.Warn("access denied",
				zap.String("method", info.FullMethod),
				zap.String("role", role),
			)
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		ctx = context.WithValue(ctx, authTokenKey, token)
		ctx = context.WithValue(ctx, authUsernameKey, username)
		ctx = context.WithValue(ctx, authRoleKey, role)
		ctx = context.WithValue(ctx, authUserIDKey, userID)

		return handler(ctx, req)
	}
}

func hasRole(allowed []string, role string) bool {
	for _, r := range allowed {
		if r == role {
			return true
		}
	}
	return false
}

// AuthTokenFromContext возвращает JWT-токен текущего пользователя.
func AuthTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authTokenKey).(string)
	return v
}

// AuthUsernameFromContext возвращает username текущего пользователя.
func AuthUsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authUsernameKey).(string)
	return v
}

// AuthRoleFromContext возвращает роль текущего пользователя.
func AuthRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authRoleKey).(string)
	return v
}
