package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// Header keys carrying the original message metadata when republished to DLQ.
const (
	HeaderDLQOriginalTopic = "x-dlq-original-topic"
	HeaderDLQErrorReason   = "x-dlq-error-reason"
	HeaderDLQAttempts      = "x-dlq-attempts"
)

// DLQTopicName returns the conventional DLQ topic name for a given source topic.
// Kafka has no auto-DLX like RabbitMQ — DLQ is a regular topic the consumer
// produces to on error.
func DLQTopicName(sourceTopic string) string {
	return sourceTopic + ".dlq"
}

// SendToDLQ republishes a failed message to its DLQ topic with extra context
// headers. The producer should be reused (not created per call).
func SendToDLQ(ctx context.Context, producer *Producer, msg kafka.Message, handlerErr error) error {
	if producer == nil {
		return fmt.Errorf("kafka: dlq producer is nil")
	}

	dlqTopic := DLQTopicName(msg.Topic)
	extra := []kafka.Header{
		{Key: HeaderDLQOriginalTopic, Value: []byte(msg.Topic)},
		{Key: HeaderDLQErrorReason, Value: []byte(handlerErr.Error())},
	}
	// Preserve trace headers and message id from the original delivery so
	// downstream tools can correlate.
	for _, h := range msg.Headers {
		switch h.Key {
		case HeaderTraceID, HeaderRequestID, HeaderMessageID:
			extra = append(extra, h)
		}
	}

	return producer.Publish(ctx, dlqTopic, string(msg.Key), msg.Value, extra...)
}
