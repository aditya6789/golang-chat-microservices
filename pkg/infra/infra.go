package infra

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func MustEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func NewPostgres(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		MustEnv("POSTGRES_USER", "chat_user"),
		MustEnv("POSTGRES_PASSWORD", "chat_password"),
		MustEnv("POSTGRES_HOST", "localhost"),
		MustEnv("POSTGRES_PORT", "5432"),
		MustEnv("POSTGRES_DB", "chat_db"),
	)
	return pgxpool.New(ctx, dsn)
}

func NewRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     MustEnv("REDIS_ADDR", "localhost:6379"),
		Password: MustEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

func NewNATS() (*nats.Conn, error) {
	return nats.Connect(MustEnv("NATS_URL", nats.DefaultURL))
}

