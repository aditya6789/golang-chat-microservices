# Chat Service

Chat Service handles realtime communication over WebSocket and fan-out using Redis pub/sub.

## Responsibilities

- Upgrade HTTP connections to WebSocket
- Maintain active connection pool (in-memory)
- Receive inbound chat events (message/typing/read-receipt/reaction)
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
- `GET /ws?chat_id=<cid>&access_token=<JWT>` (or `Authorization: Bearer` on upgrade where the client supports it)

JWT subject is the user id; `user_id` query is **not** trusted. Membership is verified against `message-service` before upgrade.

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
- `reaction` — `{ "message_id", "emoji", "reaction_action": "add" | "remove" }` (persisted via NATS; fan-out after `chat.reaction.updated`)

Server enriches payload with `chat_id`, `sender_id`, and `at`.

## Redis Channel Convention

- Channel: `chat:<chat_id>`
- All clients connected to same chat receive realtime events from this channel

## NATS Subject Convention

- Outgoing persist event: `chat.message.persist` (includes `idempotency_key`)
- Read receipts: `chat.receipt.persist`
- Reactions: `chat.reaction.persist` → message-service → `chat.reaction.updated` (subscribed here for Redis fan-out)

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
- `JWT_SECRET` (must match `auth-service`)
- `MESSAGE_SERVICE_URL` (default: `http://localhost:8084`) — used for membership checks
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `NATS_URL`

## Run

```bash
go run ./services/chat-service/cmd
```

## Notes for Production

- Replace in-memory connection map with distributed session strategy for horizontal scaling
- Add per-user/per-chat connection limits and WS flood protection
- Secure `internal` membership API (mTLS or shared service secret) if `message-service` is exposed beyond the private network

