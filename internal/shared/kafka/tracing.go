package kafka

import (
	"context"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Header keys for cross-service context propagation via Kafka.
const (
	HeaderTraceID   = "x-trace-id"
	HeaderRequestID = "x-request-id"
	HeaderMessageID = "x-message-id"
)

type ctxKey int

const (
	ctxKeyTraceID ctxKey = iota
	ctxKeyRequestID
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok {
		return v
	}
	return ""
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// injectHeaders builds Kafka headers from ctx values, generating a fresh
// trace_id if one isn't present so every produced record is traceable.
func injectHeaders(ctx context.Context, messageID string, extra []kafka.Header) []kafka.Header {
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		traceID = uuid.NewString()
	}
	headers := []kafka.Header{
		{Key: HeaderTraceID, Value: []byte(traceID)},
		{Key: HeaderMessageID, Value: []byte(messageID)},
	}
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		headers = append(headers, kafka.Header{Key: HeaderRequestID, Value: []byte(reqID)})
	}
	headers = append(headers, extra...)
	return headers
}

// extractHeaders pulls trace/request id from incoming Kafka headers and
// returns a ctx enriched with them, plus the raw values for logging.
func extractHeaders(ctx context.Context, headers []kafka.Header) (context.Context, string, string, string) {
	var traceID, requestID, messageID string
	for _, h := range headers {
		switch h.Key {
		case HeaderTraceID:
			traceID = string(h.Value)
		case HeaderRequestID:
			requestID = string(h.Value)
		case HeaderMessageID:
			messageID = string(h.Value)
		}
	}
	if traceID != "" {
		ctx = WithTraceID(ctx, traceID)
	}
	if requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	return ctx, traceID, requestID, messageID
}
