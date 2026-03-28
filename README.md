# Realtime Chat System (Golang Microservices)

Production-style starter for a realtime chat platform using clean architecture and microservices.

## Services

- `services/api-gateway`: request routing, auth middleware, rate limiting, `X-User-Id` / `X-Request-Id` injection
- `services/auth-service`: register/login, refresh tokens, JWT validation
- `services/user-service`: user profile + Redis presence heartbeat (self-only via `X-User-Id`)
- `services/chat-service`: WebSocket (JWT), membership check, Redis pub/sub, NATS persist + receipts
- `services/message-service`: chats/members APIs, message persistence, read receipts, NATS consumers
- `services/notification-service`: NATS consumer for notification events

## Tech Stack

- Go + Gin
- Gorilla WebSocket
- PostgreSQL
- Redis (presence + pub/sub)
- NATS (event-driven communication)
- Docker + Docker Compose
- Prometheus-compatible `/metrics` endpoint on all services

## Folder Structure

Each service follows:

- `cmd/`
- `internal/handler/`
- `internal/service/`
- `internal/repository/`
- `internal/model/`
- `pkg/`
- `config/`
- `Dockerfile`

## Event Flow

1. Client opens WebSocket on gateway: `GET /ws?chat_id=...&access_token=<JWT>` (or `Authorization: Bearer` where supported).
2. `chat-service` validates JWT, verifies chat membership via `message-service`, then upgrades the connection.
3. Inbound events (`message`, `typing`, `read_receipt`) are published to Redis channel `chat:<chat_id>` for realtime fan-out.
4. `message` → NATS `chat.message.persist` (with `idempotency_key`) → `message-service` stores row (member-checked).
5. `read_receipt` → NATS `chat.receipt.persist` → `message-service` upserts `message_receipts`.
6. After insert, `message-service` publishes `chat.message.created` → `notification-service` consumes.

## Core Endpoints (via API Gateway `:8080`)

Auth (public):

- `POST /auth/register` → `access_token`, `refresh_token`
- `POST /auth/login` → `access_token`, `refresh_token`
- `POST /auth/refresh` → new token pair (rotation)

Protected (require `Authorization: Bearer`):

- `GET /users/:id` — only `:id` matching JWT subject
- `POST /users/:id/heartbeat` — same
- `POST /chats` — create direct (`member_ids`: one other user) or group (`name` + `member_ids`)
- `GET /chats` — list chats for current user
- `POST /chats/:chat_id/members` — add member (group only)
- `GET /chats/:chat_id/members` — list members if you are in the chat
- `POST /messages` — body: `chat_id`, `content`, `idempotency_key` (sender = JWT user)
- `GET /messages/:chat_id?limit=20&offset=0`
- `GET /ws?chat_id=<uuid>&access_token=<JWT>`
- `GET /docs/swagger.yaml`

Downstream services receive `X-User-Id` and `X-Request-Id` on proxied requests.

## Database Schema

- `migrations/001_init.sql` — `users`, `chats`, `chat_members`, `messages`, `message_receipts`
- `migrations/002_refresh_tokens.sql` — `refresh_tokens`

## Local Development

### Pehli baar — step by step (saari services + frontend)

1. **Install karo:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Compose included) aur **Go 1.22+** (sirf test UI serve karne ke liye).

2. **Repo folder me jao** (jahan `docker-compose.yml` hai):

   ```bash
   cd path/to/realtime-chat-system
   ```

3. **Env file:** `docker-compose.yml` har service par `env_file: .env.example` use karta hai, isliye **copy zaroori nahi**. Custom values ke liye:

   ```bash
   cp .env.example .env
   ```

   Windows (PowerShell): `Copy-Item .env.example .env` — phir compose me `env_file: .env` tum khud badal sakte ho ya `docker compose --env-file .env up --build`.

4. **Saari backend services ek saath** (Postgres, Redis, NATS + 6 Go services):

   ```bash
   docker compose up --build
   ```

   Pehli baar Postgres `migrations/` folder se `001` aur `002` SQL chala deta hai. **Purana volume** ho to nayi migration manually: `docker compose exec postgres psql -U chat_user -d chat_db -f` … ya volume hata ke fresh (`docker compose down -v` — **data wipe**).

5. **Verify:** jab logs me errors na hon, health URLs open karo:

   | Service        | URL                          |
   | -------------- | ---------------------------- |
   | API Gateway    | http://localhost:8080/healthz |
   | Auth           | http://localhost:8081/healthz |
   | User           | http://localhost:8082/healthz |
   | Chat (WS)      | http://localhost:8083/healthz |
   | Message        | http://localhost:8084/healthz |
   | Notification   | http://localhost:8085/healthz |

6. **Frontend (Orbit Chat UI)** — **naya terminal**, repo root se (Docker wala chalta rehne do):

   ```bash
   go run ./cmd/serve-frontend
   ```

7. **Browser:** `http://127.0.0.1:8888` kholo. Sign in / Create account → sidebar se **Direct** / **Group** → chat select karo → message bhejo (WebSocket realtime). API URL badalne ke liye **⚙ Settings** (default `http://localhost:8080`).

8. **Direct chat test:** do users chahiye — ek normal window, ek **Incognito**; dono register → ek user ka UUID (profile) doosre se **New direct chat** me paste karo.

**Rokna:** terminal me `Ctrl+C`; containers band: `docker compose down` (volume rakhna ho to `-v` mat lagao).

### Short recap

```bash
docker compose up --build
# dusri window:
go run ./cmd/serve-frontend
# browser: http://127.0.0.1:8888  (Orbit Chat UI)  →  API http://localhost:8080
```

## Testing

```bash
go test ./...
```

## Notes

- **Kafka** is not wired; NATS is the event bus. Quality-heavy tests and DevOps (CI/K8s) are out of scope unless you ask.
- For existing PostgreSQL volumes, apply new SQL under `migrations/` manually if the container already ran older init scripts.
