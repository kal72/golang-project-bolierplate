package middleware

import (
	"context"
	"golang-project-boilerplate/internal/shared/logger"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type requestInfo struct {
	corelationID string
	reqId        string
	clientIP     string
	service      string
	source       string
	userAgent    string
	method       string // gRPC full method = http_method + path
	statusCode   codes.Code
	duration     time.Duration
	err          error
}

func UnaryLoggingInterceptor(logger *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		logRequest(logger, requestInfo{
			corelationID: extractCorelationID(ctx),
			clientIP:     extractIP(ctx),
			service:      logger.AppName,
			source:       extractSource(ctx),
			userAgent:    extractUserAgent(ctx),
			method:       info.FullMethod,
			statusCode:   statusCode(err),
			duration:     time.Since(start),
			err:          err,
		})

		return resp, err
	}
}

func StreamLoggingInterceptor(logger *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, ss)

		logRequest(logger, requestInfo{
			corelationID: extractCorelationID(ss.Context()),
			clientIP:     extractIP(ss.Context()),
			service:      logger.AppName,
			source:       extractSource(ss.Context()),
			userAgent:    extractUserAgent(ss.Context()),
			method:       info.FullMethod,
			statusCode:   statusCode(err),
			duration:     time.Since(start),
			err:          err,
		})

		return err
	}
}

func logRequest(logger *logger.Logger, info requestInfo) {
	reqId := uuid.New().String()
	fields := logrus.Fields{
		"corelation_id": info.corelationID,
		"request_id":    reqId,
		"client_ip":     info.clientIP,
		"service":       info.service,
		"source":        info.source,
		"user_agent":    info.userAgent,
		"grpc_method":   info.method,
		"grpc_status":   info.statusCode.String(),
		"duration_ms":   info.duration.Milliseconds(),
	}

	if info.err != nil {
		fields["error"] = info.err.Error()
		logger.Error("GRPC Error", fields)
		return
	}

	logger.Info("GRPC OK", fields)
}

func extractIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if v := md.Get("x-forwarded-for"); len(v) > 0 {
			return v[0]
		}
		if v := md.Get("x-real-ip"); len(v) > 0 {
			return v[0]
		}
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}

	return host
}

func extractUserAgent(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "unknown"
	}
	if v := md.Get("user-agent"); len(v) > 0 {
		return v[0]
	}
	return "unknown"
}

func extractSource(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "unknown"
	}

	if v := md.Get("x-source"); len(v) > 0 {
		return v[0]
	}

	return "unknown"
}

func statusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Internal
}

func extractCorelationID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "unknown"
	}
	if v := md.Get("x-corelation-id"); len(v) > 0 {
		return v[0]
	}
	return "unknown"
}
