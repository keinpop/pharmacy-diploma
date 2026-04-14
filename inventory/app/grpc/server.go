package grpc

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "pharmacy/inventory/gen/inventory"
)

type Server struct {
	grpcServer *grpc.Server
	port       string
	logger     *zap.Logger
}

func NewServer(port string, handler *Handler, authClient *AuthClient, logger *zap.Logger, serviceToken string) *Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor(serviceToken, authClient, logger)),
	)
	pb.RegisterInventoryServiceServer(srv, handler)
	reflection.Register(srv)
	return &Server{grpcServer: srv, port: port, logger: logger}
}

func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return err
	}
	s.logger.Info("inventory gRPC listening", zap.String("port", s.port))
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() { s.grpcServer.GracefulStop() }
