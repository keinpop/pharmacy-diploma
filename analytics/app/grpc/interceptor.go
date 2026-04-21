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

var methodRoles = map[string][]string{
	"/analytics.AnalyticsService/CreateSalesReport":    {"admin", "manager"},
	"/analytics.AnalyticsService/GetSalesReport":       {"admin", "manager"},
	"/analytics.AnalyticsService/CreateWriteOffReport": {"admin", "manager"},
	"/analytics.AnalyticsService/GetWriteOffReport":    {"admin", "manager"},
	"/analytics.AnalyticsService/CreateForecast":       {"admin", "manager"},
	"/analytics.AnalyticsService/GetForecast":          {"admin", "manager"},
	"/analytics.AnalyticsService/CreateWasteAnalysis":  {"admin", "manager"},
	"/analytics.AnalyticsService/GetWasteAnalysis":     {"admin", "manager"},
}

// добавь тип ключа вверху файла
type contextKey string

const authTokenKey contextKey = "auth_token"

// AuthInterceptor validates tokens and enforces role-based access control.
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

		// Inter-service call via service token
		if token == serviceToken {
			if hasRole(allowed, "service") {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		// User token — validate via Auth service
		_, role, err := authClient.ValidateToken(ctx, token)
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

func AuthTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authTokenKey).(string)
	return v
}
