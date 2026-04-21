package grpc

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "pharmacy/analytics/gen/analytics"
)

// Server wraps the gRPC server.
type Server struct {
	grpcServer *grpc.Server
	port       string
	logger     *zap.Logger
}

// NewServer creates a new gRPC Server with the analytics handler and auth interceptor registered.
func NewServer(port string, handler *Handler, authClient *AuthClient, logger *zap.Logger, serviceToken string) *Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor(serviceToken, authClient, logger)),
	)
	pb.RegisterAnalyticsServiceServer(srv, handler)
	reflection.Register(srv)
	return &Server{grpcServer: srv, port: port, logger: logger}
}

// Run starts listening and serving gRPC requests.
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return err
	}
	s.logger.Info("analytics gRPC listening", zap.String("port", s.port))
	return s.grpcServer.Serve(lis)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() { s.grpcServer.GracefulStop() }
