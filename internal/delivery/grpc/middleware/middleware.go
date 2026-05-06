package middleware

import (
	"golang-project-boilerplate/internal/shared/logger"

	"google.golang.org/grpc"
)

type Middleware struct {
	LoggingUnaryInterceptor  grpc.UnaryServerInterceptor
	LoggingStreamInterceptor grpc.StreamServerInterceptor
}

func NewMiddleware(
	logger *logger.Logger,
) *Middleware {
	return &Middleware{
		LoggingUnaryInterceptor:  UnaryLoggingInterceptor(logger),
		LoggingStreamInterceptor: StreamLoggingInterceptor(logger),
	}
}
