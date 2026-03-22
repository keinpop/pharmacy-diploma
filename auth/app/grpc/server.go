package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	usecase "pharma/auth/domain/use_case"
	authpb "pharma/auth/gen/auth/auth"
)

func NewServer(uc *usecase.AuthUseCase) *grpc.Server {
	srv := grpc.NewServer()

	handler := NewAuthHandler(uc)
	authpb.RegisterAuthServiceServer(srv, handler)

	reflection.Register(srv)

	return srv
}

func Run(srv *grpc.Server, port string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}
	return srv.Serve(lis)
}
