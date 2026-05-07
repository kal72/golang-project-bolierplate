# Kafka Module

Production-ready Kafka integration built on `segmentio/kafka-go`. Provides
reliable producing, consumer-group consuming with worker pools, DLQ-on-error,
circuit-breaker protection, and trace propagation through Kafka headers.

## Table of Contents

1. [Architecture](#architecture)
2. [Configuration](#configuration)
3. [Client](#client)
4. [Producer](#producer)
5. [Consumer](#consumer)
6. [Dead Letter Queue](#dead-letter-queue)
7. [Tracing & Observability](#tracing--observability)
8. [Circuit Breaker](#circuit-breaker)
9. [Graceful Shutdown](#graceful-shutdown)
10. [End-to-End Example](#end-to-end-example)
11. [Operational Notes](#operational-notes)
12. [Differences vs. RabbitMQ Module](#differences-vs-rabbitmq-module)

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                  Application Container                     │
│                                                            │
│   ┌────────────┐         ┌──────────────────────────┐      │
│   │  Producer  │────────▶│       Kafka Cluster      │      │
│   │ (+breaker) │         │  ┌────────┐  ┌────────┐  │      │
│   └────────────┘         │  │ topic  │  │ topic  │  │      │
│                          │  │ user.* │  │ *.dlq  │  │      │
│   ┌────────────┐         │  └────────┘  └────────┘  │      │
│   │ DLQ        │────────▶│       (partitions)       │      │
│   │ Producer   │         │                          │      │
│   └────────────┘         └────────────┬─────────────┘      │
│         ▲                             │                    │
│         │                             ▼                    │
│         │                  ┌────────────────────┐          │
│         └──────────────────│      Consumer      │          │
│           on handler error │  (worker pool +    │          │
│                            │   manual commit)   │          │
│                            └────────────────────┘          │
└────────────────────────────────────────────────────────────┘
```

### Files

| File | Purpose |
|---|---|
| `client.go` | Shared client: dialer, transport, TLS, SASL (PLAIN / SCRAM-SHA-256 / SCRAM-SHA-512). |
| `producer.go` | `Producer` wrapping `kafka.Writer` with `acks=all`, hash partitioner, sync writes, optional circuit breaker. |
| `consumer.go` | Consumer-group reader with worker pool, manual commit, panic recovery, configurable error policy. |
| `tracing.go` | Trace ID / Request ID / Message ID propagation via Kafka headers. |
| `dlq.go` | `SendToDLQ` helper + DLQ topic naming convention (`<topic>.dlq`). |

---

## Configuration

All knobs live in `config.json` under `kafka`. Zero values fall back to safe
defaults — existing configs keep working.

```json
{
  "kafka": {
    "brokers": ["kafka:9092"],
    "clientid": "golang-project-boilerplate",
    "tls": false,
    "sasl": {
      "enabled": false,
      "mechanism": "PLAIN",
      "username": "",
      "password": ""
    },
    "dialtimeout": 10,
    "writetimeout": 10,
    "readtimeout": 10,
    "requiredacks": -1,
    "compression": "snappy",
    "batchsize": 100,
    "batchtimeout": 50,
    "groupid": "golang-project-boilerplate",
    "startoffset": "latest",
    "minbytes": 1,
    "maxbytes": 1048576,
    "maxattempts": 5,
    "breaker": {
      "failurethreshold": 0.5,
      "minrequests": 10,
      "timeout": "30s",
      "maxhalfopenreq": 3
    }
  }
}
```

### Key fields

| Field | Meaning | Recommended |
|---|---|---|
| `requiredacks` | -1 = all in-sync replicas, 1 = leader only, 0 = fire-and-forget | `-1` for durability |
| `compression` | `none` / `gzip` / `snappy` / `lz4` / `zstd` | `snappy` (fast, decent ratio) |
| `batchsize` / `batchtimeout` | Producer batching window | 100 / 50ms is a good balance |
| `startoffset` | `earliest` (replay all) or `latest` (skip backlog) | `latest` for new groups |
| `groupid` | Default consumer group; can be overridden per-consumer | one per service |

### SASL mechanisms

- `PLAIN` — username/password over TLS only (set `tls: true`).
- `SCRAM-SHA-256`, `SCRAM-SHA-512` — challenge-response, safer than PLAIN.
- Leave `sasl.enabled = false` for local / unauthenticated brokers.

---

## Client

```go
import "golang-project-boilerplate/internal/shared/kafka"

client, err := kafka.NewClient(cfg.Kafka, appLog)
if err != nil { /* fatal */ }
defer client.Close()
```

### Responsibilities

- Validates brokers list.
- Builds a shared `*kafka.Dialer` and `*kafka.Transport` with TLS / SASL.
- Both are reused by every producer and consumer derived from this client.

### Why one shared client?

Kafka connections are expensive. A single transport pool keeps TCP
connections warm across producers, and a single dialer lets all consumers
authenticate identically.

---

## Producer

```go
producer := kafka.NewProducer(client)
defer producer.Close()

err := producer.PublishJSON(ctx, "user.events", "user-42", payload)
```

### Behavior

- **Sync writes** (`Async: false`) — `WriteMessages` returns when broker
  acknowledges based on `requiredacks`.
- **Hash balancer** — same key → same partition → ordering preserved per key.
- **Trace headers injected** — `x-trace-id`, `x-message-id`, optional
  `x-request-id` are added automatically from `ctx`.
- **MessageID** is a fresh UUID for every record; surfaced as a header so
  consumers can dedup.
- **Compression** controlled by config (`snappy` default).
- **Retries** governed by `maxattempts` (5 by default).
- Returns an error if:
  - The broker rejects the write (e.g. quota, ACL).
  - All retries are exhausted.
  - The breaker is `Open` (when configured with `NewProducerWithBreaker`).

### Raw bytes

```go
err := producer.Publish(ctx, "user.events", "user-42", []byte(`{"id":42}`))
```

### Custom headers

Pass extra `kafka.Header` values as variadic args; they're appended after
the trace headers (so trace headers are never overwritten).

### Idempotency

`segmentio/kafka-go`'s `Writer` is not transactional, but with
`requiredacks: -1` plus retries, duplicate writes can occur during retry
storms. Make consumers idempotent (use `x-message-id` as a dedup key).

---

## Consumer

```go
import (
    "golang-project-boilerplate/internal/shared/kafka"
    kafkago "github.com/segmentio/kafka-go"
)

dlqProducer := kafka.NewProducer(client) // reused across consumers
defer dlqProducer.Close()

consumer, err := kafka.NewConsumer(client, kafka.ConsumerOptions{
    Topic:         "user.events",
    Workers:       3,
    HandleTimeout: 30 * time.Second,
    ErrorPolicy:   kafka.PolicyDLQ,
    DLQProducer:   dlqProducer,
}, func(ctx context.Context, msg kafkago.Message) error {
    traceID := kafka.TraceIDFromContext(ctx)
    return processOrder(ctx, msg.Value, traceID)
})
if err != nil { /* fatal */ }

if err := consumer.Start(ctx); err != nil { /* fatal */ }
defer consumer.Stop()
```

### Guarantees

- **Consumer group** managed by Kafka; partitions auto-rebalanced.
- **Manual offset commit** after a successful handler (or after the error
  policy is satisfied).
- **Worker pool** for parallel handling. Records on the same partition are
  serialized by `kafka-go`; multiple workers consume different partitions
  in parallel.
- **Per-message timeout** (`HandleTimeout`).
- **Panic recovery** — never crashes the worker; routed through the error
  policy.
- **Trace propagation** — handler `ctx` is enriched with trace_id /
  request_id from headers.

### Error policies

| Policy | Behavior | When to use |
|---|---|---|
| `PolicyDLQ` (recommended) | Send record to `<topic>.dlq`; on success commit; on DLQ failure DO NOT commit (durable). | Default for production. |
| `PolicySkip` | Log error and commit; the message is dropped. | Acceptable data loss (telemetry). |
| `PolicyRetryLocal` | Don't commit; rely on rebalance/restart to redeliver. | Transient errors only — accepts poison-message risk. |

`PolicyDLQ` requires a `DLQProducer` (which can be the same producer used
for normal writes or a dedicated one).

### Idempotency requirement

Kafka delivery is **at-least-once**. Make handlers idempotent:

- Use `x-message-id` header (set by the producer) as a dedup key.
- Or use `(topic, partition, offset)` as a natural primary key in your
  staging table.
- Wrap side effects in `INSERT … ON CONFLICT DO NOTHING` or equivalent.

---

## Dead Letter Queue

Kafka has no broker-side DLX. The convention is:

- Source topic: `user.events`
- DLQ topic: `user.events.dlq` (regular topic — must be created by ops/IaC).

The consumer publishes failed records to the DLQ via `SendToDLQ`, copying
the original key, value, and trace headers. Three DLQ-specific headers are
added:

| Header | Value |
|---|---|
| `x-dlq-original-topic` | The source topic (useful for monolith DLQ handlers). |
| `x-dlq-error-reason` | `error.Error()` from the failing handler. |
| `x-dlq-attempts` | Reserved for future use; currently set by caller if needed. |

### Replay / monitoring

Attach a regular consumer to the DLQ topic for visibility:

```go
dlqConsumer, _ := kafka.NewConsumer(client, kafka.ConsumerOptions{
    Topic:       "user.events.dlq",
    GroupID:     "user-svc-dlq-monitor",
    ErrorPolicy: kafka.PolicySkip, // never re-DLQ a DLQ message
}, func(ctx context.Context, msg kafkago.Message) error {
    log.Printf("dead letter on %s offset=%d reason=%s",
        string(msg.Headers... /* x-dlq-original-topic */),
        msg.Offset,
        string(msg.Headers... /* x-dlq-error-reason */),
    )
    return nil
})
```

---

## Tracing & Observability

### Context helpers

```go
ctx = kafka.WithTraceID(ctx, "trace-abc")
ctx = kafka.WithRequestID(ctx, "req-123")

trace := kafka.TraceIDFromContext(ctx)
req   := kafka.RequestIDFromContext(ctx)
```

### How it flows

1. HTTP middleware sets request_id / trace_id in the request `ctx`.
2. **Producer** copies them into Kafka headers (`x-trace-id`,
   `x-request-id`, `x-message-id`). Trace ID is auto-generated when
   missing.
3. **Consumer** extracts them back into the handler's `ctx`.
4. Both producer and consumer log structured fields (trace_id, message_id,
   topic, partition, offset, duration_ms).

### Log fields produced

| Component | Fields |
|---|---|
| `kafka` | client init / dial events |
| `kafka.producer` | `topic`, `key`, `message_id`, `trace_id`, `duration_ms`, `size_bytes` |
| `kafka.consumer` | `topic`, `partition`, `offset`, `key`, `worker_id`, `message_id`, `trace_id`, `request_id`, `duration_ms`, `error?`, `panic?`, `dlq_error?` |

The logger is optional — passing `nil` disables internal logging entirely.

---

## Circuit Breaker

```go
import "golang-project-boilerplate/internal/shared/breaker"

cb := breaker.NewCircuitBreaker(cfg.Kafka.Breaker)
producer := kafka.NewProducerWithBreaker(client, cb)
```

The breaker counts these as failures:

- Broker unreachable / dial timeout.
- Write timeout.
- Topic ACL / quota errors after retries.

When `Open`, `Publish` returns immediately with `"circuit breaker is open"`.
This protects request-path latency when the broker is degraded — pair
with a fallback (DB-backed outbox or async queue) if you need durability.

---

## Graceful Shutdown

Resources are tracked with `*app.Lifecycle`, which closes everything in
LIFO order on SIGINT / SIGTERM.

```go
client, _ := kafka.NewClient(cfg.Kafka, appLog)
lifecycle.AddFunc("kafka-client", func() error { return client.Close() })

producer := kafka.NewProducer(client)
lifecycle.AddFunc("kafka-producer", func() error { return producer.Close() })

dlqProducer := kafka.NewProducer(client)
lifecycle.AddFunc("kafka-dlq", func() error { return dlqProducer.Close() })

consumer, _ := kafka.NewConsumer(client, opts, handler)
_ = consumer.Start(ctx)
lifecycle.AddFunc("kafka-consumer-user.events", func() error { consumer.Stop(); return nil })
```

### Golden rule

> A resource that **uses** another resource must be `Add`ed **after** it.

Because shutdown is LIFO:

```
Add order:    client → producer → dlq-producer → consumer
Close order:  consumer → dlq-producer → producer → client
```

The consumer stops first (drain + commit final offsets), then producers
flush their buffers, then the client closes idle transport connections.

---

## End-to-End Example

```go
package main

import (
    "context"
    "time"

    "golang-project-boilerplate/internal/app"
    "golang-project-boilerplate/internal/config"
    "golang-project-boilerplate/internal/shared/kafka"

    kafkago "github.com/segmentio/kafka-go"
)

func main() {
    cfg := config.NewConfig()
    fiberApp := config.NewFiber(cfg)
    lifecycle := app.Container(fiberApp, cfg)
    app.RunWithGracefulShutdown(fiberApp, cfg, lifecycle)
}

// Inside Container() you would wire:
func wireKafka(cfg *config.Config, lifecycle *app.Lifecycle) {
    client, err := kafka.NewClient(cfg.Kafka, nil)
    if err != nil { panic(err) }
    lifecycle.AddFunc("kafka-client", func() error { return client.Close() })

    producer := kafka.NewProducer(client)
    lifecycle.AddFunc("kafka-producer", func() error { return producer.Close() })

    // Produce
    ctx := kafka.WithTraceID(context.Background(), "trace-1")
    _ = producer.PublishJSON(ctx, "user.events", "user-42", map[string]any{
        "id":   42,
        "name": "kal",
    })

    // Consume with DLQ-on-error
    consumer, _ := kafka.NewConsumer(client, kafka.ConsumerOptions{
        Topic:         "user.events",
        Workers:       3,
        HandleTimeout: 30 * time.Second,
        ErrorPolicy:   kafka.PolicyDLQ,
        DLQProducer:   producer, // reuse the same producer
    }, func(ctx context.Context, msg kafkago.Message) error {
        return nil
    })
    _ = consumer.Start(context.Background())
    lifecycle.AddFunc("kafka-consumer-user.events", func() error { consumer.Stop(); return nil })
}
```

---

## Operational Notes

### What can fail at runtime

| Symptom | Likely cause | Where to look |
|---|---|---|
| `dial tcp: connection refused` | Broker unreachable / wrong host | Check `brokers`, network, DNS |
| `[3] Unknown Topic Or Partition` | Topic does not exist | Create topic via ops/IaC; `AllowAutoTopicCreation` is off by default |
| `[7] Request Timed Out` | `requiredacks: all` and replica is offline | Check broker ISR (in-sync replicas) |
| `[10] Message Too Large` | Record exceeds broker `message.max.bytes` | Compress or split payload |
| `[12] Offset Out Of Range` | `startoffset: earliest` on retention-pruned topic | Use `latest` or reset group offset |
| Consumer lag growing | Throughput < ingress; too few partitions or workers | Add partitions / workers |
| `commit failed` | Rebalance happened mid-handler | Acceptable; the message is redelivered |

### Things to monitor

- Consumer **lag** per group/topic (Kafka exporter → Prometheus).
- DLQ topic **depth** per source topic.
- Producer **error rate** (from logs `"publish failed"`).
- Handler `duration_ms` p95.
- Rebalance frequency.

### Topic creation

`AllowAutoTopicCreation` is **off**. Create topics via Terraform / Helm /
admin tooling with explicit:

- Partition count (governs max consumer parallelism).
- Replication factor (≥ 3 for production).
- Retention (size or time-based).
- DLQ topic with the same conventions.

### Choosing partition count

- Each partition is consumed by **at most one** worker in a group.
- Plan for **N partitions ≥ desired peak parallelism**.
- Starting too low forces a topic rebuild later. Better to over-provision
  modestly (e.g., 6–12 for medium-volume topics).

### Idempotent handler patterns

```go
func handle(ctx context.Context, msg kafkago.Message) error {
    msgID := /* read x-message-id from headers */
    if alreadyProcessed(msgID) {
        return nil // dedup hit; commit and move on
    }
    return tx.Do(func() error {
        process(msg)
        markProcessed(msgID)
        return nil
    })
}
```

---

## Differences vs. RabbitMQ Module

If you've used the RabbitMQ module first, here's how this one differs:

| Concept | RabbitMQ | Kafka |
|---|---|---|
| Exchange / routing | Exchange + routing key + bindings | Topic + partition (key-hash) |
| Reliability | Auto-reconnect + publisher confirms + mandatory | Library-managed; `acks=all` + retries + idempotent handler |
| DLQ | Broker-side DLX (queue arg) | App-side: produce to `<topic>.dlq` topic |
| Ordering | Per-queue (single consumer) | Per-partition |
| Scaling consumers | Add consumers; broker round-robins | Add consumers up to N partitions |
| Outbox | Module-provided (`rabbitmq/outbox`) | Not included in this module — use the rabbitmq/outbox table style with a Kafka producer if needed |
| At-most-once delivery | Possible via `noAck` | Not really — design for at-least-once + idempotency |

Pick RabbitMQ for fan-out / per-message routing flexibility and built-in
DLX. Pick Kafka for log-style streaming, replay, and high-throughput
ordered processing per key.
