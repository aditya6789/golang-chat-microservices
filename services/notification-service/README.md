# Notification Service

Notification Service consumes chat events and triggers notification workflows (push/email/in-app adapters can be added).

## Responsibilities

- Subscribe to message-created events from NATS
- Transform event payload into notification domain model
- Execute notification pipeline (currently structured logging starter)

## Tech

- Go + Gin
- NATS consumer
- Structured logging (zap)

## Endpoints

- `GET /healthz`
- `GET /metrics`

## Current Event Contract

Subscribed subject:

- `chat.message.created`

Event shape (example):

```json
{
  "id": "message-id",
  "chat_id": "chat-id",
  "sender_id": "user-id",
  "content": "message text",
  "created_at": "2026-03-27T00:00:00Z"
}
```

## Important Files

- `cmd/main.go`: service startup and NATS subscription
- `internal/service/notification_service.go`: event handling logic
- `internal/repository/event_repository.go`: pub/sub abstraction
- `internal/model/notification.go`: internal notification model

## Environment Variables

- `NOTIFICATION_SERVICE_PORT` (default: `8085`)
- `NATS_URL`

## Run

```bash
go run ./services/notification-service/cmd
```

## Notes for Production

- Add retry + dead-letter queue strategy
- Add channel adapters (FCM/APNS/SMTP/WebPush)
- Add user-level notification preferences and quiet hours

