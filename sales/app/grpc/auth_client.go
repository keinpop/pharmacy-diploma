package grpc

import (
	"context"
	"fmt"

	authpb "pharmacy/sales/gen/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	client authpb.AuthServiceClient
}

func NewAuthClient(authAddr string) (*AuthClient, error) {
	conn, err := grpc.NewClient(authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("auth client: %w", err)
	}
	return &AuthClient{client: authpb.NewAuthServiceClient(conn)}, nil
}

// ValidateToken возвращает userID, username и роль пользователя для JWT-токена.
func (a *AuthClient) ValidateToken(ctx context.Context, token string) (userID int64, username, role string, err error) {
	resp, err := a.client.ValidateToken(ctx, &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		return 0, "", "", err
	}
	return resp.UserId, resp.Username, resp.Role, nil
}
