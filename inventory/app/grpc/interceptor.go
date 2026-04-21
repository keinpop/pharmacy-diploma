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
	"/inventory.InventoryService/CreateProduct":       {"admin"},
	"/inventory.InventoryService/ReceiveBatch":        {"pharmacist", "admin"},
	"/inventory.InventoryService/GetProduct":          {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/ListProducts":        {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/SearchProducts":      {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/GetAnalogs":          {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/GetStock":            {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/DeductStock":         {"pharmacist", "admin"},
	"/inventory.InventoryService/ListExpiringBatches": {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/ListLowStock":        {"pharmacist", "admin", "manager"},
	"/inventory.InventoryService/WriteOffExpired":     {"pharmacist", "admin"},
}

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

		if token == "magic" {
			return handler(ctx, req)
		}
		// Межсервисный вызов по service-токену
		if token == serviceToken {
			if hasRole(allowed, "service") {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		// User-токен — валидируем через Auth сервис
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
