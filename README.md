# Realtime Chat System — Golang Microservices

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)
[![Status](https://img.shields.io/badge/status-active-success)]()
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](./CONTRIBUTING.md)

**Production-style realtime chat** I built with Go microservices: JWT auth, WebSockets, friend discovery, direct and group chats, and an event-driven core. Everything sits behind an API gateway—not a single monolith binary.

I structured it this way to practice how a real chat product splits **auth**, **users**, **messaging**, and **realtime** into separate services, with Postgres as the source of truth, Redis for presence and fan-out, and NATS for async work. Docker Compose ties the stack together for local runs.

> ⭐ If you find this project useful, consider giving it a star — it really helps!

---

## Table of contents

- [Features](#features)
- [Future ideas](#future-ideas)
- [Live demo](#live-demo)
- [Screenshots](#screenshots)
- [Architecture](#architecture)
- [Design decisions](#design-decisions)
- [Scaling & operations](#scaling--operations)
- [Tech stack](#tech-stack)
- [Repository layout](#repository-layout)
- [How data flows](#how-data-flows)
- [API overview](#api-overview)
- [Configuration](#configuration)
- [Local setup (Docker + UI)](#local-setup-docker--ui)
- [Database migrations](#database-migrations)
- [Security & production hardening](#security--production-hardening)
- [Observability](#observability)
- [Testing](#testing)
- [Infra and tooling I have not added yet](#infra-and-tooling-i-have-not-added-yet)
- [What I deliberately left out](#what-i-deliberately-left-out)
- [Contributing](#contributing)
- [FAQ](#faq)
- [License](#license)
- [Author](#author)

---

## Features

| Area | What you get |
|------|----------------|
| **Auth** | Register / login, JWT access tokens, refresh token rotation (Postgres-backed) |
| **Realtime** | WebSocket connection per chat; typing indicators; messages fan-out via **Redis pub/sub** |
| **Persistence** | Messages (including **`message_type` `text` \| `file`**), read receipts, and **emoji reactions** via **NATS** consumers in `message-service` |
| **Attachments** | **Presigned PUT** to **S3-compatible** storage (MinIO in Compose); `POST /chats/:id/attachments/presign` then `POST /messages` with `message_type: file` |
| **Social** | Search users by **email / username**; **friends**; **direct chats** only between friends; **groups** only with friends (and add-member restricted to friends) |
| **Gateway** | Single entrypoint, **JWT validation**, **`X-User-Id` / `X-Request-Id`** injection, **rate limiting**, **CORS** for browser clients |
| **UI** | **Orbit Chat** (Next.js app + static SPA via `go run ./cmd/serve-frontend`); **read receipts**; **reply threading**; **emoji reactions**; **file attachments**; **Open Graph link previews** on URLs in text messages |
| **Ops** | **Docker Compose** for all infra + services; **`/healthz`** and **Prometheus-style `/metrics`** on services |

---

## Future ideas

Most of the following is still a **personal backlog**; a few items (e.g. read receipts in the UI) are implemented and called out inline. They extend what I already have (Go services, Postgres, Redis, NATS, WebSockets today; WebRTC later for realtime media).

### Chat and UX

- ~~**Read receipts in the UI**~~ — **Done:** Orbit Chat shows delivered/read on your own bubbles; history includes `read_by` from `message_receipts`; `read_receipt` events over the WebSocket update peers live.
- ~~**Reply threading**~~ — **Done:** `messages.reply_to_message_id` + `reply_to` preview in REST/history and on WebSocket payloads; Orbit Chat **Reply** control and quoted strip above the new text; order remains `created_at` ascending in the thread view.
- ~~**Reactions**~~ — **Done:** `message_reactions` in Postgres; `chat.reaction.persist` / `chat.reaction.updated` over NATS; Orbit Chat sends `{type:"reaction", message_id, emoji, reaction_action}` on the WebSocket; `POST /messages/:id/reactions` as fallback; history includes `reactions` per message.
- **Edit and delete** — soft delete, `edited_at`, realtime sync to other clients.
- **Mentions** — `@username` resolution and light notifications.
- **Unread counts** — per-chat read cursors or a small aggregate API for the sidebar.

### Social and rooms

- **Friend requests** — pending / accept / reject instead of only instant mutual friends.
- **Block and report** — basic moderation hooks.
- **Group admin roles** — promote, kick, transfer ownership.
- **Invite links** — token-based joins while keeping the same trust rules (e.g. friends-only groups).

### Media and rich content

- ~~**Attachments**~~ — **Done:** presigned uploads (S3 or MinIO), `messages.message_type` `file`, `POST /chats/:chat_id/attachments/presign`, MinIO + bucket in Docker Compose; Orbit Chat **+** control uploads then saves a file message with image preview when applicable.
- ~~**Link previews**~~ — **Done:** `GET /messages/link-preview?url=` (Open Graph + fallbacks); in-memory TTL cache and per-user rate limit in `message-service`; SSRF-safe fetch; Orbit Chat shows a card under the first `http(s)` URL in a text bubble.

### Voice and video (native, first-party)

- **Voice and video calling** — built in-house on **WebRTC** (browser / mobile WebRTC APIs), with **signaling** over my own services (no third-party calling SDKs or paid CPaaS like Twilio/Agora for the core path). I would own the offer/answer/ICE flow, session state, and friend-gated call setup the same way as DMs today. For tough NAT scenarios I would run **self-hosted STUN/TURN** (e.g. coturn) under my control—not an external “video API,” just standard infra.
- **Optional extras later** — group calls, screen share, quality adaptation—still on the same native WebRTC stack.

### Reliability and scale

- **NATS JetStream** — durable consumers, replay, DLQ for bad messages.
- **Tighter idempotency on WebSockets** — client ids aligned with the REST idempotency key pattern.
- **Cursor-based history** — `before` / `after` instead of offset-only at higher volume.

### Security

- **End-to-end encryption (E2EE)** — for messages and, once calls exist, for media where feasible: **keys stay on clients**, servers only route ciphertext (or opaque blobs). That implies real crypto work—identity keys, device keys, rotation, lost-device recovery, and careful threat modeling—not just “TLS to the gateway.”
- **Stronger sessions** — device binding, sign-out-everywhere, stricter refresh handling.
- **OAuth2 / OIDC** — social or workplace login through `auth-service`.
- **mTLS** between services if I ever deploy to a stricter network.

### Observability and operations

- **OpenTelemetry** — traces from gateway → services → DB.
- **JSON logs** — always carry `X-Request-Id`.
- **CI** — `go test`, linters, maybe a Compose smoke job on every push.

### Notifications

- **Push / email** — actually connect `notification-service` to FCM, APNS, SES, etc.

### Testing and docs

- **E2E** — Playwright (or similar) against Compose: sign up → friend → DM → send.
- **Swagger UI** — tiny page over the existing `docs/swagger.yaml`.

**What I will probably pick up first:** CI, then edit/delete, then friend requests—roughly in that order.

---

## Live demo

**Hosted demo:** Coming soon 🚀

**Local:** Follow [Local setup](#local-setup-docker--ui): UI at http://127.0.0.1:8888, API at http://localhost:8080.

---

## Screenshots

Screenshots will be added soon.

---

## Architecture

High-level: the **browser** talks only to the **API gateway** (REST + WebSocket). The gateway forwards to internal services; **PostgreSQL** is the system of record; **Redis** handles presence and realtime fan-out; **NATS** decouples write-side events (persist messages, notifications).

```mermaid
flowchart LR
  subgraph Client
    B[Browser / SPA]
  end
  subgraph Gateway
    GW[API Gateway :8080]
  end
  subgraph Services
    A[auth-service]
    U[user-service]
    M[message-service]
    C[chat-service]
    N[notification-service]
  end
  subgraph Data
    PG[(PostgreSQL)]
    R[(Redis)]
    NATS[(NATS)]
  end
  B -->|REST + WS| GW
  GW --> A
  GW --> U
  GW --> M
  GW --> C
  A --> PG
  U --> PG
  U --> R
  M --> PG
  M --> NATS
  C --> R
  C --> M
  N --> NATS
```

### Service responsibilities

| Service | Port (default) | Role |
|---------|----------------|------|
| **api-gateway** | 8080 | Reverse proxy; auth middleware; rate limit; CORS; WebSocket upgrade to chat-service |
| **auth-service** | 8081 | Register, login, refresh; issues JWTs |
| **user-service** | 8082 | Profile (self-only), presence heartbeat (Redis), **user search**, **friends** APIs |
| **chat-service** | 8083 | **WebSocket** hub; membership check against message-service; Redis pub/sub; NATS publish for persist/receipts |
| **message-service** | 8084 | Chats, members, messages, receipts; **NATS consumers** for durable writes |
| **notification-service** | 8085 | Subscribes to notification-related NATS subjects (extensible) |

### Communication styles (REST vs realtime vs events)

- **REST (through gateway):** auth, users, chats, messages, friends, search.
- **WebSocket:** `GET /ws?chat_id=...&access_token=...` — realtime messages and typing; HTTP fallback `POST /messages` still works.
- **Redis:** channel `chat:<chat_id>` for live fan-out; presence keys for online status.
- **NATS:** e.g. `chat.message.persist`, `chat.receipt.persist`, `chat.message.created` — async, service decoupling.

---

## Design decisions

These are the trade-offs I had in mind when I split the system this way:

| Choice | Rationale |
|--------|-----------|
| **Microservices** | Separates **auth**, **identity/presence**, **durable messaging**, and **realtime transport** so each can scale and fail independently; matches how chat products evolve in production (different SLAs per path). |
| **Redis (pub/sub + presence)** | Low-latency fan-out to all sockets in a room without hitting Postgres on every keystroke; presence is ephemeral by nature—TTL-backed keys fit better than relational rows for “online now”. |
| **NATS (not direct HTTP for writes)** | Decouples **chat-service** from **message-service** availability: spikes or slow DB don’t block the WS loop; consumers retry at their pace; easy to add workers or swap persistence without changing the hot path. |
| **PostgreSQL** | Single source of truth for users, chats, messages, friendships, refresh tokens—ACID where it matters. |
| **API gateway** | One TLS termination and auth story for browsers; injects **`X-User-Id`** so downstream services stay simple and don’t re-parse JWTs everywhere. |

---

## Scaling & operations

- **Horizontal scale:** Run **multiple `chat-service` instances** behind a load balancer with **sticky sessions** (or shared Redis so any instance can publish/subscribe the same `chat:<id>` channels). **message-service** and **auth-service** scale statelessly behind the gateway once Postgres and NATS handle throughput.
- **Bottlenecks:** Postgres (writes + history reads), Redis (channel fan-out), NATS (subject throughput). Connection pool tuning, read replicas for history, and JetStream retention matter if I ever need replayable pipelines.
- **State:** User/chat/message state lives in **Postgres**; **Redis** is cache/ephemeral; **NATS** is transit—design consumers to be idempotent (e.g. message idempotency keys already supported on `POST /messages`).

---

## Tech stack

- **Go 1.24+**, **Gin**, **Gorilla WebSocket**
- **PostgreSQL 16**, **Redis 7**, **NATS 2** (JetStream-capable image)
- **Docker Compose** for local full stack
- **JWT** (shared secret; validated at gateway and chat-service)

---

## Repository layout

Monorepo with shared `pkg/` and per-service modules:

```
realtime-chat-system/
├── cmd/serve-frontend/     # tiny static file server for the SPA
├── docs/                   # swagger.yaml (gateway route)
├── frontend/               # Orbit Chat UI (HTML/CSS/JS)
├── migrations/             # Postgres SQL (001…003…)
├── pkg/                    # shared infra helpers (httpx, etc.)
└── services/
    ├── api-gateway/
    ├── auth-service/
    ├── user-service/
    ├── chat-service/
    ├── message-service/
    └── notification-service/
```

Each service follows a common internal layout: `cmd/`, `internal/handler|service|repository|model/`, `config/`, `Dockerfile`.

---

## How data flows

1. Client obtains JWT via `POST /auth/login` or `POST /auth/register` (through gateway).
2. **REST** calls include `Authorization: Bearer <token>`; gateway sets **`X-User-Id`** for downstream services.
3. Client opens **WebSocket** on the gateway URL with `chat_id` + token; **chat-service** validates JWT and checks **chat membership** via message-service before upgrade.
4. **Inbound WS events** (`message`, `typing`, `read_receipt`) publish to **Redis** for live fan-out; **message** / receipts are also sent to **NATS** for **message-service** to persist. **File messages** typically use **REST** (`presign` → browser **PUT** to object storage → `POST /messages` with `message_type: file`); the persisted row is fan-out over **NATS** like text messages.
5. After persistence, **notification-service** can react to **`chat.message.created`** (extend for push/email).

---

## API overview

Base URL (local): **`http://localhost:8080`**

### Public

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/auth/register` | Create user; returns tokens |
| POST | `/auth/login` | Login; returns tokens |
| POST | `/auth/refresh` | Rotate refresh token |

### Protected (`Authorization: Bearer`)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/users/:id` | Own profile only (`:id` must match JWT `sub`) |
| POST | `/users/:id/heartbeat` | Presence ping (Redis) |
| GET | `/users/search?q=` | Search by email/username (min 2 chars) |
| GET | `/users/friends` | List friends |
| POST | `/users/friends` | Body `{"user_id":"<uuid>"}` — add friend |
| POST | `/chats` | Create **direct** (friends only) or **group** (members must be friends) |
| POST | `/chats/direct` | Body `{"other_user_id":"<uuid>"}` — get or create DM (friends only) |
| GET | `/chats` | List my chats |
| POST | `/chats/:chat_id/members` | Add member (**group**; must be friend) |
| GET | `/chats/:chat_id/members` | Members (+ username/email) |
| POST | `/chats/:chat_id/attachments/presign` | Body `filename`, `content_type`, `size_bytes` — returns `upload_url`, `object_key`, `headers` for **PUT** |
| POST | `/messages` | Send message: `content` + optional `reply_to_message_id`, or **`message_type: "file"`** + **`file`** (`object_key`, `filename`, `mime_type`, `size_bytes`) after upload; idempotency key supported |
| GET | `/messages/link-preview?url=` | Open Graph metadata for a public `http(s)` URL (cached; rate-limited per user) |
| GET | `/messages/:chat_id` | History (`limit`, `offset`) — includes `message_type` and file JSON in `content` when `file` |
| GET | `/ws` | WebSocket (query: `chat_id`, `access_token`) |
| GET | `/docs/swagger.yaml` | OpenAPI-style description |

> **Note:** I route friend APIs through **`/users/friends`** in the SPA so an older gateway build still works; newer gateways can also expose `POST /friends` directly.

### Example: login + open DM (curl)

```bash
# Login
curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"yourpassword"}'

# Use access_token from response:
export TOKEN="<access_token>"

# Add friend then ensure direct chat (or use UI “Add & chat”)
curl -s -X POST http://localhost:8080/users/friends \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"user_id":"<friend-uuid>"}'

curl -s -X POST http://localhost:8080/chats/direct \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"other_user_id":"<friend-uuid>"}'
```

---

## Configuration

- **`.env.example`** — template for JWT TTL, Postgres, Redis, NATS, **S3 / MinIO** (`S3_ENDPOINT`, `S3_PUBLIC_BASE_URL`, keys, `S3_BUCKET`), ports. Compose references it by default; copy to `.env` when you need overrides.
- **Important vars:** `JWT_SECRET` (change in any real deployment), `POSTGRES_*`, `REDIS_ADDR`, `NATS_URL`, **`S3_*`** for attachments, per-service ports. **`S3_PUBLIC_BASE_URL`** must be reachable from the **browser** (e.g. `http://localhost:9000` when MinIO is published on 9000).

Never commit `.env` with real secrets; keep `.env.example` non-sensitive.

---

## Local setup (Docker + UI)

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (includes Compose)
- **Go 1.24+** (for `go run ./cmd/serve-frontend` and `go test`)

### 1) Start backend

```bash
cd realtime-chat-system
docker compose up --build
```

### 2) Run the web UI (second terminal)

```bash
go run ./cmd/serve-frontend
```

Open **`http://127.0.0.1:8888`**. In **Settings**, set API base to **`http://localhost:8080`** if needed.

### 3) Health checks

| Service | URL |
|---------|-----|
| API Gateway | http://localhost:8080/healthz |
| Auth | http://localhost:8081/healthz |
| User | http://localhost:8082/healthz |
| Chat | http://localhost:8083/healthz |
| Message | http://localhost:8084/healthz |
| Notification | http://localhost:8085/healthz |

### Stop

- `Ctrl+C` in compose terminal, then `docker compose down`  
- Wipe DB volume (destructive): `docker compose down -v`

---

## Database migrations

Files under `migrations/` are mounted into Postgres `docker-entrypoint-initdb.d` — they run **only on first init** of an empty data volume.

| File | Contents |
|------|----------|
| `001_init.sql` | users, chats, chat_members, messages, message_receipts |
| `002_refresh_tokens.sql` | refresh_tokens |
| `003_friendships.sql` | friendships (required for friends / friend-gated chats) |
| `004_reply_to_message.sql` | `messages.reply_to_message_id` (optional FK to parent message) |
| `005_message_reactions.sql` | `message_reactions` (per user per emoji per message) |
| `006_message_type.sql` | `messages.message_type` (`text` \| `file`) for attachments |

**Existing volume?** Apply new SQL manually, e.g.:

```bash
docker compose exec -T postgres psql -U chat_user -d chat_db -f /docker-entrypoint-initdb.d/003_friendships.sql
docker compose exec -T postgres psql -U chat_user -d chat_db -f /docker-entrypoint-initdb.d/004_reply_to_message.sql
docker compose exec -T postgres psql -U chat_user -d chat_db -f /docker-entrypoint-initdb.d/005_message_reactions.sql
docker compose exec -T postgres psql -U chat_user -d chat_db -f /docker-entrypoint-initdb.d/006_message_type.sql
```

---

## Security & production hardening

| Layer | What this repo does | Production follow-up |
|-------|---------------------|----------------------|
| **JWT** | HS256-style shared secret; validated at **gateway** (REST) and **chat-service** (WebSocket); `sub` = user id | Rotate **`JWT_SECRET`**, short access TTL, asymmetric keys (RS256) if multiple issuers |
| **Identity on wire** | Gateway sets **`X-User-Id`** and **`X-Request-Id`** on proxied requests | Ensure internal network is trusted; never expose downstream ports publicly without mTLS |
| **Rate limiting** | Token bucket on gateway (`api-gateway` middleware) | Tune per route; add IP/user-based limits at edge (CDN / WAF) |
| **CORS** | Enabled for browser SPA | I would restrict **`AllowOrigins`** to my real frontend origin in production |
| **Data** | Passwords hashed in auth layer; friend-gated DMs/groups in message-service | Encrypt at rest (managed Postgres), audit logs, secret manager for env |
| **Transport** | Plain HTTP in Compose | Terminate **TLS** at ingress / reverse proxy; HSTS in production |

I treat this as a **strong architecture demo**, not something I would expose to the internet as-is—real production would add **structured logging**, **distributed tracing**, and **alerting** on `/healthz` and error rates.

---

## Observability

| Signal | Where | Notes |
|--------|--------|------|
| **Liveness** | `GET /healthz` on each service | Use for orchestrator probes (K8s `livenessProbe` / Docker health) |
| **Metrics** | `GET /metrics` (Prometheus exposition) | Scrape per service; correlate with gateway latency and DB pool stats |
| **Request correlation** | `X-Request-Id` set at gateway | I would propagate this in logs across services once logging is structured |
| **Realtime health** | WS connect success + Redis/NATS connectivity | Monitor chat-service restarts and NATS consumer lag |

---

## Testing

```bash
go test ./...
```

---

## Infra and tooling I have not added yet

I have not set any of this up in the repo; it is the sort of work I would do when polishing for production or interviews:

- A **public hosted demo** and **screenshots** under `docs/screenshots/`.
- **GitHub Actions** (or similar) for `go test`, lint, optional Compose smoke—no workflow files checked in right now.
- **Hosted API docs** — beyond the raw file, I only ship `docs/swagger.yaml` and `GET /docs/swagger.yaml` on the gateway in local runs.
- **JetStream / DLQ** on top of plain NATS pub/sub for persistence consumers.
- **Kafka** as an alternative bus—only if I ever need to align with a Kafka-first environment.

---

## What I deliberately left out

- **Kafka** in the default path—I standardised on **NATS** for this project.
- **Kubernetes / Helm** — I run everything with Compose locally; K8s would be a separate effort.
- **Managed OAuth** — not wired; the codebase is JWT + email/password first.

---

## Contributing

Issues and PRs are welcome. Please read **[CONTRIBUTING.md](./CONTRIBUTING.md)** for how I like branches, commits, and tests run before a PR.

---

## FAQ

**Why not one Go binary?**  
I wanted to practice **service boundaries** and **independent scaling**; the cost is more moving parts and ops surface, which is intentional for this learning project.

**Could Redis or NATS be swapped?**  
Yes, but the adapters would need rewriting: I use Redis for fan-out and NATS for async persistence—same roles, different products, if someone forks and swaps them.

**`friendships` table missing after `docker compose`?**  
Postgres only runs `migrations/` on a **fresh** volume. On an old volume, run `003_friendships.sql` yourself—see [Database migrations](#database-migrations).

---

## License

This project is licensed under the MIT License — see the [LICENSE](./LICENSE) file for details.

---

## Author

Aditya Paswan
