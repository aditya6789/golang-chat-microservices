# API Gateway Service

API Gateway is the single entry point for clients. It routes requests to downstream services, enforces authentication, and applies basic rate limiting.

## Responsibilities

- Route `/auth/*` requests to `auth-service`
- Route protected traffic (`/users/*`, `/messages/*`, `/chat/*`, `/ws`, `/ws/signaling`) to corresponding services
- Validate JWT by calling `auth-service /auth/validate`
- Apply rate limiting middleware
- Expose health and metrics endpoints

## Tech

- Go + Gin
- Reverse proxy (`net/http/httputil`)
- Rate limiter (`golang.org/x/time/rate`)
- Prometheus compatible metrics endpoint

## Endpoints

- `GET /healthz`
- `GET /metrics`
- `GET /docs/swagger.yaml`
- `ANY /auth/*path` (public)
- `ANY /users/*path` (protected)
- `ANY /messages` and `ANY /messages/*path` (protected)
- `ANY /chats` and `ANY /chats/*path` (protected)
- `ANY /chat/*path` (protected)
- `ANY /ws` and `ANY /ws/signaling` (protected proxy to chat-service WebSocket endpoints: chat and WebRTC signaling)

On protected routes, the gateway validates JWT then sets **`X-User-Id`** (JWT `sub`) and **`X-Request-Id`** on the outbound proxied request.

## Authentication Flow

1. Client sends `Authorization: Bearer <token>`
2. Gateway calls `auth-service:8081/auth/validate`
3. If valid, request is proxied to target service
4. If invalid, `401 Unauthorized`

## Important Files

- `cmd/main.go`: server bootstrapping and route grouping
- `internal/handler/gateway_handler.go`: proxy handlers
- `internal/service/gateway_service.go`: auth + proxy utilities
- `internal/repository/auth_repository.go`: validate token via auth-service
- `pkg/middleware.go`: auth and rate-limit middleware

## Environment Variables

- `API_GATEWAY_PORT` (default: `8080`)

## Run

```bash
go run ./services/api-gateway/cmd
```

## Docker

```bash
docker build -f services/api-gateway/Dockerfile -t api-gateway .
```

## Notes for Production

- Replace in-memory/global limiter with distributed limiter (Redis-based) for multi-instance deployment
- Add request ID propagation and structured access logs
- Add circuit breaker and retry policies for downstream service calls

