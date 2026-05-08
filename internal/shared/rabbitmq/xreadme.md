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
│  ┌──────────┐   ┌─────────────┐   ┌─────────────────────────┐   │
│  │ HTTP     │   │  Use case   │   │   Outbox Repository     │   │
│  │ Handler  │──▶│  (business) │──▶│ (writes inside DB tx)   │   │
│  └──────────┘   └─────────────┘   └─────────────────────────┘   │
│                       │                       │                  │
│                       ▼                       ▼                  │
│              ┌────────────────┐      ┌────────────────┐          │
│              │   Publisher    │      │ Outbox Relayer │          │
│              └────────┬───────┘      └────────┬───────┘          │
│                       │                       │                  │
│                       └────────┬──────────────┘                  │
│                                ▼                                 │
│                  ┌──────────────────────────┐                    │
│                  │   rabbitmq.Connection    │                    │
│                  │  ┌────────────────────┐  │                    │
│                  │  │  Circuit Breaker   │  │                    │
│                  │  │  (Executor iface)  │  │                    │
│                  │  └────────────────────┘  │                    │
│                  │  ┌────────────────────┐  │                    │
│                  │  │  Channel Pool      │  │                    │
│                  │  │  [ch1][ch2]...[chN]│  │                    │
│                  │  │  (confirm mode)    │  │                    │
│                  │  └────────────────────┘  │                    │
│                  │  ┌────────────────────┐  │                    │
│                  │  │  Admin Channel     │  │                    │
│                  │  │  (consumer/topo)   │  │                    │
│                  │  └────────────────────┘  │                    │
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
| `connection.go` | Wraps `*amqp.Connection` with auto-reconnect, a publish channel pool (confirm mode), a dedicated admin channel (consumer/topology), and an optional circuit breaker via the `Executor` interface. |
| `publisher.go` | High-level publish API (`PublishJSON`, `PublishToQueue`). Circuit breaker is inherited from Connection. |
| `consumer.go` | Worker-pool consumer with auto-restart on reconnect, panic recovery, and ack/nack policy. |
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
    "channelpoolsize": 5,
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
| `tls` | Uses `amqp://` (plain). Set `true` for `amqps://`. |
| `channelpoolsize` | Defaults to 5 parallel publish channels. |
| `breaker.failurethreshold` | Connection is created without a circuit breaker. |
| `outbox.enabled` | Outbox relayer is not started. |
| `app.shutdowntimeout` | Falls back to 15s. |
| Any duration field | Falls back to internal default (see code). |

---

## Connection Lifecycle

`Connection` is the heart of the module. It owns one `*amqp.Connection`, a
**publish channel pool**, and one **admin channel** — all rebuilt automatically
on failure.

```go
import "golang-project-boilerplate/internal/shared/rabbitmq"

conn, err := rabbitmq.NewConnection(cfg.RabbitMQ, appLog)
if err != nil { /* fatal */ }
defer conn.Close()
```

### Publish Channel Pool

Each `Publish` call acquires one channel from the pool, waits for the broker
confirm, then returns it. Unlike a single channel behind a global mutex, the
pool allows many goroutines to publish **in parallel**:

```
Goroutine A  ──[acquire ch1]──[send]──[wait confirm]──[release ch1]──►
Goroutine B  ──[acquire ch2]──[send]──[wait confirm]──[release ch2]──►
Goroutine C  ──[acquire ch3]──[send]──[wait confirm]──[release ch3]──►
```

Pool size is controlled by `channelpoolsize` (default 5). Goroutines that
exceed the pool size block until a channel becomes available or the context
is cancelled.

On reconnect, the pool is drained and refilled with fresh channels automatically
— no changes required in caller code.

### Admin Channel

A separate channel (without confirm mode) is reserved for consumers and
topology helpers. It does not compete with the publish pool, so declare and
consume operations never block an in-flight publish.

### What it gives you

- **Auto-reconnect** with exponential backoff (`reconnectdelay` → `maxreconnectdelay`).
- **Publisher confirms** enabled on every pool channel (`channel.Confirm(false)`).
- **Mandatory + return** detection: unroutable messages surface as errors instead of being silently dropped.
- **Per-publish timeout** (`publishtimeout`) protects callers from a stuck broker.
- **Pool rebuilt after reconnect**, including confirm mode on each new channel.

### What it does NOT do

- It does not declare topology. Call `DeclareQueue` / `DeclareQueueWithDLQ`
  at startup before publishing or consuming.
- It does not buffer messages during disconnects. While reconnecting, `Publish`
  returns an error — pair with the outbox pattern if you need durability across outages.

---

## Publisher

```go
publisher := rabbitmq.NewPublisher(conn)

// JSON to a custom exchange + routing key
err := publisher.PublishJSON(ctx, "events.topic", "user.created", payload)

// JSON directly to a queue via the default exchange
err := publisher.PublishToQueue(ctx, "user.events", payload)
```

### Behavior

- Always sets `DeliveryMode: persistent`.
- Always sets `MessageId` (UUID) and `Timestamp`.
- Always injects `x-trace-id` (and optional `x-request-id`) headers from `ctx`.
- Generates a fresh trace ID if `ctx` does not carry one yet.
- Returns an error if:
  - The broker NACKs the message.
  - The message is unroutable (mandatory + no bound queue).
  - Publisher confirm timeout elapses.
  - The connection is currently disconnected.
  - The circuit breaker is open (if configured on Connection).

### Circuit Breaker

The circuit breaker is configured at the `Connection` level, not on the
Publisher. This ensures every publish path — including the outbox relayer that
calls `conn.Publish()` directly — is protected consistently.

```go
import "golang-project-boilerplate/internal/shared/breaker"

cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
conn, err := rabbitmq.NewConnectionWithBreaker(cfg.RabbitMQ, appLog, cb)

// Plain publisher — breaker is automatically active via conn
publisher := rabbitmq.NewPublisher(conn)
```

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
    RequeueOnError: false, // false → routed to DLQ if configured
}, func(ctx context.Context, msg amqp.Delivery) error {
    traceID := rabbitmq.TraceIDFromContext(ctx)
    return processMessage(ctx, msg.Body, traceID)
})

if err := consumer.Start(ctx); err != nil { /* fatal */ }
defer consumer.Stop()
```

### Guarantees

- **Ack on nil error**, **Nack on non-nil error or panic**.
- **Worker pool** — N goroutines share one delivery channel; RabbitMQ load-balances via prefetch.
- **Per-message timeout** through `HandleTimeout`.
- **Panic recovery** — never crashes the worker; nacks with the configured requeue policy.
- **Auto-restart on reconnect** — when the channel closes, workers exit cleanly and a new consume session starts after the connection recovers.
- **Trace propagation** — handler receives a `ctx` enriched with `trace_id` / `request_id` from message headers.

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

All helpers are idempotent. Call them at startup before starting any consumer.

---

## Dead Letter Queue

`DeclareQueueWithDLQ` is a one-shot helper that creates the full DLQ topology
in a single call:

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

The DLQ message carries an `x-death` header with the original queue, the reason,
and the redelivery count — useful for replay or alerting.

### Replaying or monitoring

Attach a regular consumer to `<queue>.dlq`:

```go
dlqConsumer := rabbitmq.NewConsumer(conn, rabbitmq.ConsumerOptions{
    Queue:   "user.events.dlq",
    Workers: 1,
}, func(ctx context.Context, msg amqp.Delivery) error {
    log.Printf("dead letter: %s reason=%v", msg.Body, msg.Headers["x-death"])
    return nil // ack to avoid requeue loops
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
   automatically. If trace ID is missing, a new UUID is generated.
3. **Consumer** extracts them from `msg.Headers` back into the handler's `ctx`.
4. **Logger** is invoked with structured fields by both publisher and consumer.

### Log fields produced by this module

| Component | Fields |
|---|---|
| `rabbitmq` | connection / reconnect events |
| `rabbitmq.publisher` | `exchange`, `routing_key`, `message_id`, `trace_id`, `duration_ms`, `size_bytes` |
| `rabbitmq.consumer` | `queue`, `worker_id`, `message_id`, `trace_id`, `request_id`, `duration_ms`, `error?`, `panic?` |
| `rabbitmq.outbox` | `id`, `attempts`, `retry_in`, `error?` |
| `lifecycle` | `resource`, `duration_ms`, `error?` |

The logger is optional — passing `nil` disables all internal logging.

---

## Circuit Breaker

The circuit breaker operates at the `Connection` level so that **all publish
paths are protected** — both direct publishes via `Publisher` and background
publishes via the outbox `Relayer` that calls `conn.Publish()` directly.

### Dependency inversion

`Connection` does not import the `breaker` package. Instead it defines its own
minimal interface:

```go
type Executor interface {
    Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error)
}
```

`*breaker.CircuitBreaker` satisfies this interface implicitly (Go duck typing),
so the `rabbitmq` package remains fully decoupled from the `breaker` package.

### Usage

```go
import "golang-project-boilerplate/internal/shared/breaker"

cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
conn, err := rabbitmq.NewConnectionWithBreaker(cfg.RabbitMQ, appLog, cb)

publisher := rabbitmq.NewPublisher(conn) // breaker is automatically active
```

### State machine

```
         failure ratio ≥ threshold
Closed ──────────────────────────────► Open
  ▲                                      │
  │ success in HalfOpen                  │ after timeout
  │                                      ▼
  └──────────────────────────────── HalfOpen
                                  (probe N requests)
```

### What counts as a failure

- Channel unavailable (during reconnect window).
- Publisher confirm timeout.
- Mandatory return (unroutable message).
- Broker NACK.

When `Open`, publish returns immediately with `"circuit breaker is open"` — no
broker round-trip. When `HalfOpen`, a small number of probes is allowed
(`MaxHalfOpenReq`); one success closes the breaker, one failure re-opens it.

### Configuration

| Field | Default | Description |
|---|---|---|
| `failurethreshold` | `0.5` | Failure ratio that trips the breaker (0.0–1.0) |
| `minrequests` | `10` | Minimum requests before the threshold is evaluated |
| `timeout` | `30s` | How long the breaker stays Open before moving to HalfOpen |
| `maxhalfopenreq` | `3` | Maximum probe requests allowed in HalfOpen state |

---

## Outbox Pattern

Solves the dual-write problem: you cannot atomically commit to the database
**and** publish to RabbitMQ. The outbox stores the event in the same DB
transaction as the business data; a background relayer then ships it to the broker.

### 1. Create the table

The schema is documented in [`outbox/entity.go`](outbox/entity.go). For MySQL:

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

If the publish would fail, the row stays `pending` and the relayer retries it.
The user row and the event are always in sync.

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

### Guarantees

- **Horizontal scale safe**: `FetchPending` uses `FOR UPDATE SKIP LOCKED`, so
  multiple relayer instances cannot pick the same row.
- **At-least-once**: a crash between a successful publish and `MarkSent` will
  re-publish the event on the next tick. Consumers must be idempotent.
- **Bounded retries**: exceeding `MaxAttempts` moves the row to `failed`. Monitor these rows.

### Trade-offs

- Adds latency between DB commit and broker visibility (≤ `PollInterval`).
- Consumers must dedup using `message_id` (= outbox row ID).
- Failed rows accumulate — alert on `WHERE status = 'failed'`.

---

## Graceful Shutdown

Resources are tracked with `*app.Lifecycle`, which closes everything in LIFO
order on SIGINT / SIGTERM.

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

The consumer stops first (no new handlers run), then the outbox relayer (no
new publishes), then the connection (clean broker disconnect), and finally the
logger (all preceding shutdown logs are flushed).

---

## End-to-End Example

```go
package main

import (
    "context"
    "log"
    "time"

    "golang-project-boilerplate/internal/app"
    "golang-project-boilerplate/internal/config"
    "golang-project-boilerplate/internal/shared/breaker"
    "golang-project-boilerplate/internal/shared/rabbitmq"
    "golang-project-boilerplate/internal/shared/rabbitmq/outbox"

    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    cfg := config.NewConfig()
    fiberApp := config.NewFiber(cfg)
    lifecycle := app.Container(fiberApp, cfg)
    app.RunWithGracefulShutdown(fiberApp, cfg, lifecycle)
}

// Inside Container() (see internal/app/container.go):
func wireRabbitMQ(cfg *config.Config, appLog *logger.Logger, lifecycle *app.Lifecycle) {
    // Connection with circuit breaker
    cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
    conn, err := rabbitmq.NewConnectionWithBreaker(
        config.ToRabbitMQConfig(cfg.RabbitMQ), appLog, cb,
    )
    if err != nil {
        log.Fatal(err)
    }
    lifecycle.AddFunc("rabbitmq", func() error { return conn.Close() })

    // Topology
    _, _ = rabbitmq.DeclareQueueWithDLQ(conn, rabbitmq.DLQOptions{
        Queue:         "user.events",
        MessageTTL:    5 * time.Minute,
        DLQMessageTTL: 7 * 24 * time.Hour,
    })

    // Publisher — breaker is active automatically via conn
    publisher := rabbitmq.NewPublisher(conn)
    ctx := rabbitmq.WithTraceID(context.Background(), "trace-1")
    _ = publisher.PublishToQueue(ctx, "user.events", map[string]any{
        "id":   42,
        "name": "kal",
    })

    // Outbox relayer
    relayer := outbox.NewRelayer(db, conn, appLog, outboxRelayerOpts(cfg.RabbitMQ.Outbox))
    relayer.Start(ctx)
    lifecycle.AddFunc("outbox-relayer", func() error { relayer.Stop(); return nil })

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
| `rabbitmq channel unavailable` | Publishing during reconnect window | Wait, or rely on the outbox pattern |
| `publish confirmation timeout` | Slow broker or network issue | Tune `publishtimeout`; check broker load |
| `message unroutable` | Mandatory flag + no queue bound for the routing key | Verify topology with `DeclareQueue` / `BindQueue` |
| `publish nacked by broker` | Broker resource limits or queue argument conflict | Check the management UI |
| `circuit breaker is open` | Too many consecutive publish failures | Check broker connectivity; breaker will self-probe after `timeout` |
| Consumer keeps restarting | Queue does not exist or missing `consume` permission | Declare queue at startup |
| Pending outbox rows keep growing | Relayer not running, broker down, or breaker open | Check logs for `rabbitmq.outbox` |

### Things to monitor

- `outbox WHERE status = 'failed'` count.
- DLQ depth per queue.
- Consumer `duration_ms` p95 — handler latency.
- Reconnect frequency in logs.
- Circuit breaker state (`Open` → check broker connectivity).

### Choosing requeue vs. DLQ

- **`RequeueOnError: true`** — transient errors that may resolve quickly
  (locked row, brief network blip). Risk: poison messages loop forever.
- **`RequeueOnError: false`** + DLQ — permanent failures. Inspect and replay manually.

A common pattern is to start with `false` + DLQ, and let the handler itself
implement bounded in-process retries before returning an error.
