package service

import (
	"context"
	"time"

	"realtime-chat-system/services/user-service/internal/model"
	"realtime-chat-system/services/user-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

type UserService struct {
	repo  *repository.UserRepository
	redis *redis.Client
}

func New(repo *repository.UserRepository, redis *redis.Client) *UserService {
	return &UserService{repo: repo, redis: redis}
}

func (s *UserService) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	p, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	p.Online = s.redis.Exists(ctx, "presence:"+userID).Val() == 1
	return p, nil
}

func (s *UserService) SetOnline(ctx context.Context, userID string) error {
	return s.redis.Set(ctx, "presence:"+userID, "1", 90*time.Second).Err()
}

