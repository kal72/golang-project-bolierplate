package server

import (
	"golang-project-boilerplate/gen/ping"
	"golang-project-boilerplate/internal/delivery/grpc/handler"
	"golang-project-boilerplate/internal/delivery/grpc/middleware"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
)

type GrpcServer struct {
	middleware  *middleware.Middleware
	pingHandler *handler.PingHandler
}

func NewGrpcServer(
	middleware *middleware.Middleware,
	pingHandler *handler.PingHandler,
) *GrpcServer {
	return &GrpcServer{
		middleware:  middleware,
		pingHandler: pingHandler,
	}
}

func (s *GrpcServer) SetupGrpc() *grpc.Server {
	server := grpc.NewServer(
		// Interceptor unary
		grpc.ChainUnaryInterceptor(
			s.middleware.LoggingUnaryInterceptor,
			recovery.UnaryServerInterceptor(),
		),
		// Interceptor stream
		grpc.ChainStreamInterceptor(
			s.middleware.LoggingStreamInterceptor,
			recovery.StreamServerInterceptor(),
		),
	)

	// Setup handler
	ping.RegisterPingServiceServer(server, s.pingHandler)

	return server
}
