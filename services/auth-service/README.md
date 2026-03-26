# Auth Service

Auth Service handles user registration, login, and JWT validation.

## Responsibilities

- Register new users
- Authenticate users using email/password
- Issue JWT access tokens
- Validate JWT token for protected APIs

## Tech

- Go + Gin
- PostgreSQL (`users` table)
- JWT (`github.com/golang-jwt/jwt/v5`)
- Password hashing with bcrypt

## Endpoints

- `GET /healthz`
- `GET /metrics`
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/validate`

## Request/Response Examples

### Register

`POST /auth/register`

```json
{
  "email": "user@example.com",
  "username": "chatuser",
  "password": "strongPass123"
}
```

Response:

```json
{
  "access_token": "<jwt>"
}
```

### Login

`POST /auth/login`

```json
{
  "email": "user@example.com",
  "password": "strongPass123"
}
```

## Important Files

- `cmd/main.go`: app initialization
- `internal/handler/auth_handler.go`: HTTP handlers
- `internal/service/auth_service.go`: business logic (register/login/validate)
- `internal/repository/user_repository.go`: user persistence
- `config/config.go`: env-based config

## Environment Variables

- `AUTH_SERVICE_PORT` (default: `8081`)
- `JWT_SECRET` (required in production)
- `JWT_TTL_MINUTES` (default: `60`)
- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`

## Run

```bash
go run ./services/auth-service/cmd
```

## Test

```bash
go test ./services/auth-service/...
```

## Notes for Production

- Use refresh tokens + token rotation
- Add brute-force protection for login endpoint
- Add email verification and password reset flows

