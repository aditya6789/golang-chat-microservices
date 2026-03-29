# Message Service

Message Service is responsible for durable message storage and history retrieval.

## Responsibilities

- Persist chat messages in PostgreSQL
- Support idempotent writes using `idempotency_key`
- Provide paginated history APIs
- Consume async persist events from NATS
- Emit post-persist events for downstream consumers (notifications, analytics)

## Tech

- Go + Gin
- PostgreSQL
- NATS event streaming style communication

## Endpoints

- `GET /healthz`
- `GET /metrics`
- `GET /internal/chats/:chat_id/membership?user_id=` — service-to-service membership check (not exposed via gateway)
- `POST /chats` — create chat (`X-User-Id` = creator)
- `GET /chats` — list chats for `X-User-Id`
- `POST /chats/:chat_id/members` — add member (group only)
- `GET /chats/:chat_id/members` — list members (must be in chat)
- `POST /messages` — sender is always `X-User-Id` (body no longer accepts `sender_id`)
- `GET /messages/:chat_id?limit=20&offset=0` — requires membership; each item may include `read_by` from `message_receipts`

## Idempotency

`POST /messages` expects:

```json
{
  "chat_id": "chat-uuid",
  "sender_id": "user-uuid",
  "content": "Hi",
  "idempotency_key": "client-generated-unique-key"
}
```

DB constraint on `idempotency_key` prevents duplicate inserts when clients retry.

## NATS Subjects

- Consumed: `chat.message.persist`, `chat.receipt.persist`
- Published: `chat.message.created`

## Important Files

- `cmd/main.go`: API + NATS subscription bootstrap
- `internal/handler/message_handler.go`: HTTP handlers
- `internal/service/message_service.go`: business logic + event publish
- `internal/repository/message_repository.go`: SQL persistence and pagination

## Environment Variables

- `MESSAGE_SERVICE_PORT` (default: `8084`)
- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`
- `NATS_URL`

## Run

```bash
go run ./services/message-service/cmd
```

## Notes for Production

- Add transactional outbox for stronger delivery guarantees
- Add retention policy and archival strategy for old messages
- Add full-text search indexing for message content

