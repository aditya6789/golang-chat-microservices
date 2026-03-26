# User Service

User Service manages user profile reads and online presence heartbeat.

## Responsibilities

- Fetch user profile from PostgreSQL
- Resolve online/offline status using Redis presence keys
- Update presence heartbeat for active users

## Tech

- Go + Gin
- PostgreSQL for user profile data
- Redis for presence (`presence:<user_id>`)

## Endpoints

- `GET /healthz`
- `GET /metrics`
- `GET /users/:id`
- `POST /users/:id/heartbeat`

## Presence Strategy

- Client periodically calls `POST /users/:id/heartbeat`
- Service writes key: `presence:<id>` with TTL (90s)
- Profile response includes `online: true/false` based on key existence

## Important Files

- `cmd/main.go`: bootstrapping, routes
- `internal/handler/user_handler.go`: profile + heartbeat endpoints
- `internal/service/user_service.go`: presence logic
- `internal/repository/user_repository.go`: DB access

## Environment Variables

- `USER_SERVICE_PORT` (default: `8082`)
- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`
- `REDIS_ADDR`
- `REDIS_PASSWORD`

## Run

```bash
go run ./services/user-service/cmd
```

## Notes for Production

- Add profile update APIs and validation
- Add distributed cache for frequently requested profile reads
- Add richer presence states (`online`, `away`, `busy`, `offline`)

