package handler

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ping "golang-project-boilerplate/gen/ping"
)

type PingHandler struct {
	ping.UnimplementedPingServiceServer
}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) Ping(ctx context.Context, req *ping.PingRequest) (*ping.PingResponse, error) {
	if ctx.Err() != nil {
		return nil, status.Errorf(codes.Canceled, "request canceled")
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &ping.PingResponse{
		Message:   "pong",
		Hostname:  hostname,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}
