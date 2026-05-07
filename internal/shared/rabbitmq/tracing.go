package rabbitmq

import (
	"context"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Header names yang dipakai untuk propagasi context lintas service via AMQP.
const (
	HeaderTraceID   = "x-trace-id"
	HeaderRequestID = "x-request-id"
)

type ctxKey int

const (
	ctxKeyTraceID ctxKey = iota
	ctxKeyRequestID
)

// WithTraceID menyisipkan trace ID ke context. Dipakai oleh publisher untuk
// inject ke headers AMQP dan oleh consumer setelah ekstraksi dari delivery.
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

// injectHeaders menyalin trace/request id dari ctx ke headers AMQP. Jika
// trace_id belum ada di ctx, generate baru agar selalu terlacak end-to-end.
func injectHeaders(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		traceID = uuid.NewString()
	}
	headers[HeaderTraceID] = traceID

	if reqID := RequestIDFromContext(ctx); reqID != "" {
		headers[HeaderRequestID] = reqID
	}
	return headers
}

// extractHeaders mengembalikan ctx baru yang dilengkapi trace/request id dari
// headers AMQP — dipakai consumer agar handler menerima ctx ber-trace.
func extractHeaders(ctx context.Context, headers amqp.Table) (context.Context, string, string) {
	traceID, _ := headers[HeaderTraceID].(string)
	requestID, _ := headers[HeaderRequestID].(string)

	if traceID != "" {
		ctx = WithTraceID(ctx, traceID)
	}
	if requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	return ctx, traceID, requestID
}
