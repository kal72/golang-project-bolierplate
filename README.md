# Golang Project Boilerplate

## Description

A Golang boilerplate following Clean Architecture principles, with a gRPC delivery layer, dependency injection via Google Wire, and a rich set of shared utilities (circuit breaker, query builder, pagination, structured logging, test kit, etc.).

## Tech Stack

- Golang v1.26
- gRPC + Protocol Buffers
- Google Wire (compile-time DI)
- MySQL (via GORM)
- Redis

## Framework & Library

- gRPC: https://github.com/grpc/grpc-go
- go-grpc-middleware (interceptors): https://github.com/grpc-ecosystem/go-grpc-middleware
- Google Wire (DI): https://github.com/google/wire
- GoFiber (optional HTTP): https://github.com/gofiber/fiber
- GORM (ORM): https://github.com/go-gorm/gorm
- Viper (Configuration): https://github.com/spf13/viper
- Go Playground Validator: https://github.com/go-playground/validator
- Logrus + Lumberjack (Logger / rotation): https://github.com/sirupsen/logrus
- JWT: https://github.com/golang-jwt/jwt
- go-redis: https://github.com/redis/go-redis

## Project Structure

```
.
├── cmd/web/                # application entry point
├── di/                     # Google Wire dependency graph
├── proto/                  # .proto definitions
├── gen/                    # generated protobuf / gRPC code
├── db/                     # database migrations / SQL
├── config.json             # runtime configuration
└── internal/
    ├── app/                # application bootstrap & graceful shutdown
    ├── config/             # Viper, logger, GORM, Redis, Fiber, validator
    ├── delivery/grpc/
    │   ├── server/         # gRPC server setup & interceptor chain
    │   ├── handler/        # gRPC handlers (e.g. PingService)
    │   └── middleware/     # unary & stream interceptors (logging, etc.)
    ├── usecase/            # business logic
    ├── repository/         # data access
    ├── model/              # request/response/domain models
    └── shared/             # cross-cutting utilities
        ├── breaker/        # circuit breaker, retry, fallback
        ├── query/          # filter & query builder
        ├── pagination/
        ├── logger/
        ├── errorhandler/
        ├── gosafe/         # safe goroutine wrapper
        ├── response/
        ├── general/
        ├── constant/
        └── testkit/        # mocks & helpers for unit tests
```

## Configuration

All configuration is in `config.json`.

| Key                | Description                          |
| ------------------ | ------------------------------------ |
| `app.host`         | Bind host                            |
| `app.port`         | HTTP port (Fiber)                    |
| `app.grpcPort`     | gRPC port (default `50001`)          |
| `log.path`         | Log file path                        |
| `log.stdout`       | Also write logs to stdout            |
| `database.*`       | MySQL credentials and pool settings  |

## Install Tools

Install required CLI tools (protoc plugins, wire):

```shell
make install-tools
```

You also need `protoc` itself:

```shell
brew install protobuf
```

## Code Generation

### Generate protobuf / gRPC stubs

```shell
make proto
```

Output goes to `gen/`.

### Generate Wire DI code

```shell
make wire
```

This regenerates `di/wire_gen.go` from `di/wire.go`.

## Database Migration

All database SQL files live under the `db/` folder. MySQL is provisioned automatically via `docker compose`.

## Run Application

```shell
go run cmd/web/main.go
```

The gRPC server starts on `app.grpcPort` (default `50001`).

### Test the Ping service

Using `grpcurl`:

```shell
grpcurl -plaintext -d '{"message":"hello"}' localhost:50001 ping.v1.PingService/Ping
```

Expected response:

```json
{
  "message": "pong",
  "hostname": "<hostname>",
  "timestamp": 1730000000000
}
```

## Lint

```shell
make lint
```

Requires [`golangci-lint`](https://golangci-lint.run/).

## Response Status Codes

| Status | Description                | HTTP Status |
| ------ | -------------------------- | ----------- |
| 99     | Internal server error      | 500         |
| 00     | Operation success          | 200         |
| 01     | Data not found in database | 200         |
| 04     | Request validation error   | 400         |
