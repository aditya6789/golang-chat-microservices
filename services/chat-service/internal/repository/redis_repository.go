package repository

import (
	"context"
	"encoding/json"
	"time"

	"realtime-chat-system/services/chat-service/internal/model"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct{ client *redis.Client }

func New(client *redis.Client) *RedisRepository { return &RedisRepository{client: client} }

func (r *RedisRepository) Publish(ctx context.Context, chatID string, e model.Event) error {
	b, _ := json.Marshal(e)
	return r.client.Publish(ctx, "chat:"+chatID, b).Err()
}

func (r *RedisRepository) Subscribe(ctx context.Context, chatID string) *redis.PubSub {
	return r.client.Subscribe(ctx, "chat:"+chatID)
}

func (r *RedisRepository) SetOnline(ctx context.Context, userID string) error {
	return r.client.Set(ctx, "presence:"+userID, "1", 90*time.Second).Err()
}

