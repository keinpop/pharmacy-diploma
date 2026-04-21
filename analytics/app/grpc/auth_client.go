package grpc

import (
	"context"
	"fmt"

	authpb "pharmacy/analytics/gen/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthClient wraps the gRPC client to the Auth service.
type AuthClient struct {
	client authpb.AuthServiceClient
}

// NewAuthClient creates a new AuthClient connected to the given address.
func NewAuthClient(authAddr string) (*AuthClient, error) {
	conn, err := grpc.NewClient(authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("auth client: %w", err)
	}
	return &AuthClient{client: authpb.NewAuthServiceClient(conn)}, nil
}

// ValidateToken validates a JWT token via the Auth service.
// Uses context.Background() to avoid propagating incoming metadata.
func (a *AuthClient) ValidateToken(ctx context.Context, token string) (userID int64, role string, err error) {
	resp, err := a.client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		return 0, "", err
	}
	return resp.UserId, resp.Role, nil
}
