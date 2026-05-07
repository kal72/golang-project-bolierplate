# RabbitMQ Module

Production-ready RabbitMQ integration for the boilerplate. Provides reliable
publishing, consuming, dead-letter queues, observability, circuit-breaker
protection, and the transactional outbox pattern.

## Table of Contents

1. [Architecture](#architecture)
2. [Configuration](#configuration)
3. [Connection Lifecycle](#connection-lifecycle)
4. [Publisher](#publisher)
5. [Consumer](#consumer)
6. [Topology Helpers](#topology-helpers)
7. [Dead Letter Queue](#dead-letter-queue)
8. [Tracing & Observability](#tracing--observability)
9. [Circuit Breaker](#circuit-breaker)
10. [Outbox Pattern](#outbox-pattern)
11. [Graceful Shutdown](#graceful-shutdown)
12. [End-to-End Example](#end-to-end-example)
13. [Operational Notes](#operational-notes)

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      Application Container                       │
│                                                                  │
│  ┌──────────┐   ┌─────────────┐   ┌─────────────────────────┐    │
│  │ HTTP     │   │  Use case   │   │   Outbox Repository     │    │
│  │ Handler  │──▶│  (business) │──▶│ (writes inside DB tx)   │    │
│  └──────────┘   └─────────────┘   └─────────────────────────┘    │
│                       │                       │                  │
│                       ▼                       ▼                  │
│              ┌────────────────┐      ┌────────────────┐          │
│              │   Publisher    │      │ Outbox Relayer │          │
│              │ (+ breaker)    │      │ (background)   │          │
│              └────────┬───────┘      └────────┬───────┘          │
│                       │                       │                  │
│                       └────────┬──────────────┘                  │
│                                ▼                                 │
│                  ┌──────────────────────────┐                    │
│                  │   rabbitmq.Connection    │                    │
│                  │ (auto-reconnect + confirm│                    │
│                  │  + mandatory + return)   │                    │
│                  └──────────────┬───────────┘                    │
│                                 │                                │
│                                 ▼                                │
│                  ┌──────────────────────────┐                    │
│                  │       RabbitMQ           │                    │
│                  │   ┌──────┐    ┌──────┐   │                    │
│                  │   │ EXCH │───▶│QUEUE │   │                    │
│                  │   └──────┘    └──┬───┘   │                    │
│                  │                  │       │                    │
│                  │   ┌──────┐    ┌──▼───┐   │                    │
│                  │   │ DLX  │◀───│ DLQ  │   │                    │
│                  │   └──────┘    └──────┘   │                    │
│                  └──────────────┬───────────┘                    │
│                                 │                                │
│                                 ▼                                │
│                       ┌──────────────────┐                       │
│                       │ Consumer (auto-  │                       │
│                       │ restart + worker │                       │
│                       │ pool + tracing)  │                       │
│                       └──────────────────┘                       │
└──────────────────────────────────────────────────────────────────┘
```

### Files

| File | Purpose |
|---|---|
| `connection.go` | Wraps `*amqp.Connection` with auto-reconnect, confirm mode, mandatory + return handling. Single serialized publish path. |
| `publisher.go` | High-level publish API (`PublishJSON`, `PublishToQueue`). Optional circuit breaker. |
| `consumer.go` | Worker-pool consumer with auto-restart on reconnect, panic recovery, ack/nack policy. |
| `topology.go` | Idempotent helpers to declare exchanges, queues, and bindings. |
| `dlq.go` | One-shot helper to declare a queue + DLX + DLQ pair. |
| `tracing.go` | Trace ID / Request ID propagation through AMQP headers. |
| `outbox/entity.go` | GORM model + SQL schema for the outbox table. |
| `outbox/repository.go` | Insert / fetch / mark sent / mark retry. Uses `FOR UPDATE SKIP LOCKED`. |
| `outbox/relayer.go` | Background poller that publishes pending rows with exponential backoff. |

---

## Configuration

All knobs live in `config.json` under `rabbitmq`. Zero values fall back to
sensible defaults — existing configs keep working without changes.

```json
{
  "app": { "shutdowntimeout": 15 },
  "rabbitmq": {
    "username": "guest",
    "password": "guest",
    "host": "rabbitmq",
    "port": 5672,
    "vhost": "/",
    "tls": false,
    "connecttimeout": 30,
    "heartbeat": 10,
    "prefetchcount": 10,
    "reconnectdelay": 2,
    "maxreconnectdelay": 30,
    "publishtimeout": 10,
    "breaker": {
      "failurethreshold": 0.5,
      "minrequests": 10,
      "timeout": "30s",
      "maxhalfopenreq": 3
    },
    "outbox": {
      "enabled": false,
      "pollinterval": 2,
      "batchsize": 50,
      "maxattempts": 10,
      "basebackoff": 5,
      "maxbackoff": 300
    }
  }
}
```

### Toggles

| Setting | Effect when 0 / false |
|---|---|
| `tls` | Uses `amqp://` (plain). Set true for `amqps://`. |
| `breaker.failurethreshold` | Publisher created without circuit breaker. |
| `outbox.enabled` | Outbox relayer is not started. |
| `app.shutdowntimeout` | Falls back to 15s. |
| Any duration field | Falls back to internal default (see code). |

---

## Connection Lifecycle

`Connection` is the heart of the module. It owns one `*amqp.Connection` and
one `*amqp.Channel`, both rebuilt automatically on failure.

```go
import "golang-project-boilerplate/internal/shared/rabbitmq"

conn, err := rabbitmq.NewConnection(cfg.RabbitMQ, appLog)
if err != nil { /* fatal */ }
defer conn.Close()
```

### What it gives you

- **Auto-reconnect** with exponential backoff (`reconnectdelay` → `maxreconnectdelay`).
- **Publisher confirms** enabled on every channel (`channel.Confirm(false)`).
- **Mandatory + return** detection: unroutable messages surface as errors instead
  of being silently dropped.
- **Per-publish timeout** (`publishtimeout`) protects callers from stuck brokers.
- **Channel re-create after reconnect**, including `Qos` and confirm mode.

### What it does NOT do

- It does not declare topology for you. Call `DeclareQueue` / `DeclareQueueWithDLQ`
  during startup before publishing or consuming.
- It does not buffer messages during disconnects. While reconnecting,
  `Publish` returns `"rabbitmq channel unavailable"` — pair with the outbox
  pattern if you need durability across outages.

---

## Publisher

```go
publisher := rabbitmq.NewPublisher(conn)

// JSON to a custom exchange + routing key
err := publisher.PublishJSON(ctx, "events.topic", "user.created", payload)

// JSON to a queue via the default exchange (persistent + RK = queue name)
err := publisher.PublishToQueue(ctx, "user.events", payload)
```

### Behavior

- Always sets `DeliveryMode: persistent`.
- Always sets `MessageId` (UUID) and `Timestamp`.
- Always injects `x-trace-id` (and optional `x-request-id`) headers from `ctx`.
- Generates a fresh trace ID if `ctx` does not carry one yet.
- Returns an error if:
  - The broker NACKs the message.
  - The message is unroutable (mandatory + no queue).
  - Publisher confirm timeout elapses.
  - The connection is currently disconnected.

### With circuit breaker

```go
import "golang-project-boilerplate/internal/shared/breaker"

cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
publisher := rabbitmq.NewPublisherWithBreaker(conn, cb)
```

When the breaker is `Open`, publish calls fail fast with
`"circuit breaker is open"` — no broker round-trip. Pair with the outbox
pattern to avoid losing events while the breaker recovers.

---

## Consumer

```go
import (
    amqp "github.com/rabbitmq/amqp091-go"
    "golang-project-boilerplate/internal/shared/rabbitmq"
)

consumer := rabbitmq.NewConsumer(conn, rabbitmq.ConsumerOptions{
    Queue:          "user.events",
    ConsumerTag:    "user-svc",
    Workers:        5,
    HandleTimeout:  30 * time.Second,
    RequeueOnError: false, // false → goes to DLQ if configured
}, func(ctx context.Context, msg amqp.Delivery) error {
    traceID := rabbitmq.TraceIDFromContext(ctx)
    return processMessage(ctx, msg.Body, traceID)
})

if err := consumer.Start(ctx); err != nil { /* fatal */ }
defer consumer.Stop()
```

### Guarantees

- **Ack on nil error**, **Nack on non-nil error or panic**.
- **Worker pool** — N goroutines share one delivery channel; RabbitMQ load
  balances via prefetch.
- **Per-message timeout** through `HandleTimeout`.
- **Panic recovery** — never crashes the worker; nack with the configured
  requeue policy.
- **Auto-restart on reconnect** — when the channel closes, workers exit
  cleanly and a new consume session is started after the connection
  recovers.
- **Trace propagation** — handler receives a `ctx` enriched with trace_id /
  request_id from message headers.

### Idempotency requirement

Delivery is **at-least-once**. Make handlers idempotent by:

- Using `msg.MessageId` (or your own UUID in the payload) as a dedup key.
- Wrapping side effects in `INSERT … ON DUPLICATE KEY UPDATE` or equivalent.
- Storing processed IDs in a dedup table with a TTL.

---

## Topology Helpers

```go
// Declare an exchange
err := rabbitmq.DeclareExchange(conn, rabbitmq.ExchangeOptions{
    Name:    "events.topic",
    Kind:    "topic",
    Durable: true,
})

// Declare a queue
_, err = rabbitmq.DeclareQueue(conn, rabbitmq.QueueOptions{
    Name:    "user.events",
    Durable: true,
})

// Bind queue to exchange
err = rabbitmq.BindQueue(conn, "user.events", "events.topic", "user.*", nil)
```

All helpers are idempotent. Call them at startup before `Start`-ing any
consumer.

---

## Dead Letter Queue

`DeclareQueueWithDLQ` is a one-shot helper that creates the full DLQ
topology in one call:

```go
_, err := rabbitmq.DeclareQueueWithDLQ(conn, rabbitmq.DLQOptions{
    Queue:         "user.events",
    MessageTTL:    5 * time.Minute,
    DLQMessageTTL: 7 * 24 * time.Hour,
})
```

This creates:

| Entity | Default name |
|---|---|
| Main queue | `user.events` |
| Dead-letter exchange (DLX) | `user.events.dlx` (direct, durable) |
| Dead-letter queue (DLQ) | `user.events.dlq` (durable) |
| Binding | DLQ ⟵ DLX (routing key = `user.events`) |

A message is routed to the DLQ when:

- The consumer calls `Nack(requeue=false)` (default with `RequeueOnError: false`).
- The message expires due to `MessageTTL`.
- The main queue exceeds `MaxLength` (overflow).

The DLQ message has an `x-death` header containing the original queue, the
reason, and the redelivery count — useful for replay or alerting.

### Replaying or monitoring

Attach a regular consumer to `<queue>.dlq` for visibility:

```go
dlqConsumer := rabbitmq.NewConsumer(conn, rabbitmq.ConsumerOptions{
    Queue:   "user.events.dlq",
    Workers: 1,
}, func(ctx context.Context, msg amqp.Delivery) error {
    log.Printf("dead letter: %s reason=%v", msg.Body, msg.Headers["x-death"])
    return nil // ack to avoid loops
})
```

---

## Tracing & Observability

### Context helpers

```go
ctx = rabbitmq.WithTraceID(ctx, "trace-abc")
ctx = rabbitmq.WithRequestID(ctx, "req-123")

trace := rabbitmq.TraceIDFromContext(ctx)   // "trace-abc"
req   := rabbitmq.RequestIDFromContext(ctx) // "req-123"
```

### How it flows

1. **HTTP middleware** sets the request ID / trace ID in the request `ctx`.
2. **Publisher** copies them into AMQP headers (`x-trace-id`, `x-request-id`)
   automatically. If trace ID is missing, it generates one (UUID).
3. **Consumer** extracts them from `msg.Headers` back into the handler's `ctx`.
4. **Logger** is invoked with structured fields (`trace_id`, `request_id`,
   `message_id`, `queue`, `exchange`, `duration_ms`, …) by both publisher
   and consumer.

### Log fields produced by this module

| Component | Fields |
|---|---|
| `rabbitmq` | connection / reconnect events |
| `rabbitmq.publisher` | `exchange`, `routing_key`, `message_id`, `trace_id`, `duration_ms`, `size_bytes` |
| `rabbitmq.consumer` | `queue`, `worker_id`, `message_id`, `trace_id`, `request_id`, `duration_ms`, `error?`, `panic?` |
| `rabbitmq.outbox` | `id`, `attempts`, `retry_in`, `error?` |
| `lifecycle` | `resource`, `duration_ms`, `error?` |

The logger is optional — passing `nil` disables internal logging entirely.

---

## Circuit Breaker

The publisher uses the existing `internal/shared/breaker.CircuitBreaker`.

```go
cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
publisher := rabbitmq.NewPublisherWithBreaker(conn, cb)
```

The breaker counts these as failures:

- Connection / channel unavailable.
- Publisher confirm timeout.
- Mandatory return (unroutable).
- Broker NACK.

When `Open`, publish returns immediately. When `HalfOpen`, a small number
of probes is allowed (`MaxHalfOpenReq`); a single success closes the
breaker, a failure re-opens it.

The breaker is per-publisher — different events can have different
policies (e.g., notifications vs. payments).

---

## Outbox Pattern

Solves the dual-write problem: you cannot atomically commit to the
database **and** publish to RabbitMQ. The outbox stores the event in the
same DB transaction as the business data, then a relayer ships it to the
broker.

### 1. Create the table

The schema is documented in
[`outbox/entity.go`](outbox/entity.go). For MySQL:

```sql
CREATE TABLE outbox (
  id VARCHAR(36) PRIMARY KEY,
  exchange VARCHAR(255) NOT NULL DEFAULT '',
  routing_key VARCHAR(255) NOT NULL,
  content_type VARCHAR(64) NOT NULL DEFAULT 'application/json',
  payload LONGBLOB NOT NULL,
  trace_id VARCHAR(64) NOT NULL DEFAULT '',
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  next_attempt_at DATETIME NOT NULL,
  sent_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_outbox_status_next (status, next_attempt_at)
);
```

### 2. Enqueue inside the same transaction

```go
import "golang-project-boilerplate/internal/shared/rabbitmq/outbox"

outboxRepo := outbox.NewRepository(db)

err := db.Transaction(func(tx *gorm.DB) error {
    if err := userRepo.Create(tx, &user); err != nil { return err }

    return outboxRepo.EnqueueJSON(tx,
        "user.events",   // exchange
        "user.created",  // routing key
        UserCreatedEvent{ID: user.ID},
        rabbitmq.TraceIDFromContext(ctx),
        rabbitmq.RequestIDFromContext(ctx),
    )
})
```

If the publish would fail, the row stays as `pending` and the relayer
retries it. The user row and the event are always in sync.

### 3. Start the relayer at boot

```go
relayer := outbox.NewRelayer(db, conn, appLog, outbox.RelayerOptions{
    PollInterval: 2 * time.Second,
    BatchSize:    50,
    MaxAttempts:  10,
    BaseBackoff:  5 * time.Second,
    MaxBackoff:   5 * time.Minute,
})
relayer.Start(ctx)
defer relayer.Stop()
```

Or wire from config: `cfg.RabbitMQ.Outbox.Enabled = true` triggers it
automatically in `internal/app/container.go` (see the commented section).

### Guarantees

- **Horizontal scale safe**: `FetchPending` uses `FOR UPDATE SKIP LOCKED`,
  so multiple relayer instances cannot pick the same row.
- **At-least-once**: a crash between publish-success and `MarkSent`
  re-publishes the event on the next tick. Consumers must be idempotent.
- **Bounded retries**: `MaxAttempts` exceeded → status `failed`. Monitor
  these rows.

### Trade-offs

- Adds latency between DB commit and broker visibility (≤ `PollInterval`).
- Requires consumers to dedup (use `message_id` = outbox row ID).
- Failed rows accumulate — alert on `WHERE status = 'failed'`.

---

## Graceful Shutdown

Resources are tracked with `*app.Lifecycle`, which closes everything in
LIFO order on SIGINT / SIGTERM.

```go
lifecycle := NewLifecycle(appLog)
lifecycle.AddFunc("logger", func() error { appLog.Close(); return nil })
lifecycle.AddFunc("rabbitmq", func() error { return conn.Close() })
lifecycle.AddFunc("outbox-relayer", func() error { relayer.Stop(); return nil })
lifecycle.AddFunc("consumer-user.events", func() error { consumer.Stop(); return nil })

// On shutdown:
ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancel()
lifecycle.Shutdown(ctx)
```

### Golden rule

> A resource that **uses** another resource must be `Add`ed **after** it.

Because shutdown is LIFO:

```
Add order:    logger → rabbitmq → outbox-relayer → consumer
Close order:  consumer → outbox-relayer → rabbitmq → logger
```

The consumer stops first (so no new handlers run), then the outbox
relayer (no new publishes), then the connection (clean broker disconnect),
finally the logger (so all preceding shutdown logs are flushed).

---

## End-to-End Example

```go
package main

import (
    "context"
    "time"

    "golang-project-boilerplate/internal/app"
    "golang-project-boilerplate/internal/config"
    "golang-project-boilerplate/internal/shared/rabbitmq"

    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    cfg := config.NewConfig()
    fiberApp := config.NewFiber(cfg)
    lifecycle := app.Container(fiberApp, cfg)
    app.RunWithGracefulShutdown(fiberApp, cfg, lifecycle)
}

// Inside Container() (see internal/app/container.go) you would wire:
func wireExample(cfg *config.Config, conn *rabbitmq.Connection, lifecycle *app.Lifecycle) {
    // Topology
    _, _ = rabbitmq.DeclareQueueWithDLQ(conn, rabbitmq.DLQOptions{
        Queue:         "user.events",
        MessageTTL:    5 * time.Minute,
        DLQMessageTTL: 7 * 24 * time.Hour,
    })

    // Producer
    publisher := rabbitmq.NewPublisher(conn)
    ctx := rabbitmq.WithTraceID(context.Background(), "trace-1")
    _ = publisher.PublishToQueue(ctx, "user.events", map[string]any{
        "id":   42,
        "name": "kal",
    })

    // Consumer
    consumer := rabbitmq.NewConsumer(conn, rabbitmq.ConsumerOptions{
        Queue:          "user.events",
        ConsumerTag:    "user-svc",
        Workers:        5,
        HandleTimeout:  30 * time.Second,
        RequeueOnError: false,
    }, func(ctx context.Context, msg amqp.Delivery) error {
        // handler logic
        return nil
    })
    _ = consumer.Start(context.Background())
    lifecycle.AddFunc("consumer-user.events", func() error { consumer.Stop(); return nil })
}
```

---

## Operational Notes

### What can fail at runtime

| Symptom | Likely cause | Where to look |
|---|---|---|
| `rabbitmq channel unavailable` | Publishing during reconnect window | Wait or rely on outbox / circuit breaker |
| `publish confirmation timeout` | Slow broker or network | Tune `publishtimeout`; check broker load |
| `message unroutable` | Mandatory + no bound queue for the routing key | Verify topology with `DeclareQueue` / `BindQueue` |
| `publish nacked by broker` | Broker resource limits, queue arg conflict | Check management UI |
| Consumer keeps restarting | Queue does not exist or `consume()` permission denied | Declare queue at startup |
| Pending rows in outbox grow | Relayer not running, broker down, breaker open | Check logs for `rabbitmq.outbox` |

### Things to monitor

- `outbox WHERE status = 'failed'` count.
- DLQ depth per queue.
- Consumer `duration_ms` p95 — handler latency.
- Reconnect frequency in logs.

### Choosing requeue vs. DLQ

- **`RequeueOnError: true`** — transient errors that may resolve quickly
  (locked row, brief network blip). Risk: poison messages loop forever.
- **`RequeueOnError: false`** + DLQ — permanent failures. Inspect and
  replay manually.

A common pattern is to start with `false` + DLQ, and let the handler
itself implement bounded in-process retries before returning an error.
