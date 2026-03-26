# Chat Service

Chat Service handles realtime communication over WebSocket and fan-out using Redis pub/sub.

## Responsibilities

- Upgrade HTTP connections to WebSocket
- Maintain active connection pool (in-memory)
- Receive inbound chat events (message/typing/read-receipt)
- Broadcast events through Redis pub/sub channels
- Publish message events to NATS for async persistence

## Tech

- Go + Gin
- Gorilla WebSocket
- Redis pub/sub
- NATS (event bus)

## Endpoint

- `GET /healthz`
- `GET /metrics`
- `GET /ws?user_id=<uid>&chat_id=<cid>`

## Supported Event Shape

Example client payload:

```json
{
  "type": "message",
  "content": "Hello"
}
```

Other `type` values supported by design:

- `typing`
- `read_receipt`

Server enriches payload with `chat_id`, `sender_id`, and `at`.

## Redis Channel Convention

- Channel: `chat:<chat_id>`
- All clients connected to same chat receive realtime events from this channel

## NATS Subject Convention

- Outgoing persist event: `chat.message.persist`

## Reconnect and Liveness

- Read deadline + pong handler are configured to keep connections healthy
- Client should reconnect with exponential backoff and rejoin `chat_id`

## Important Files

- `cmd/main.go`: wiring of Redis, NATS, Gin
- `internal/handler/websocket_handler.go`: ws upgrade and inbound loop
- `internal/service/hub.go`: connection pool and pub/sub flow
- `internal/repository/redis_repository.go`: Redis operations

## Environment Variables

- `CHAT_SERVICE_PORT` (default: `8083`)
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `NATS_URL`

## Run

```bash
go run ./services/chat-service/cmd
```

## Notes for Production

- Replace in-memory connection map with distributed session strategy for horizontal scaling
- Add auth context extraction from JWT (currently query based for starter)
- Add per-user/per-chat connection limits and WS flood protection

