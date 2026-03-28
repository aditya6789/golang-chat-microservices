package service

import (
	"context"
	"errors"
	"time"

	"realtime-chat-system/services/user-service/internal/model"
	"realtime-chat-system/services/user-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

var (
	ErrSelfFriend   = errors.New("cannot add yourself as a friend")
	ErrUserNotFound = errors.New("user not found")
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

func (s *UserService) SearchUsers(ctx context.Context, selfID, q string) ([]model.UserProfile, error) {
	list, err := s.repo.SearchUsers(ctx, selfID, q, 20)
	if err != nil {
		return nil, err
	}
	out := make([]model.UserProfile, 0, len(list))
	for _, p := range list {
		cp := p
		cp.Online = s.redis.Exists(ctx, "presence:"+cp.ID).Val() == 1
		out = append(out, cp)
	}
	return out, nil
}

func (s *UserService) AddFriend(ctx context.Context, selfID, otherID string) error {
	if selfID == otherID {
		return ErrSelfFriend
	}
	ok, err := s.repo.UserExists(ctx, otherID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUserNotFound
	}
	return s.repo.AddFriendship(ctx, selfID, otherID)
}

func (s *UserService) ListFriends(ctx context.Context, userID string) ([]model.UserProfile, error) {
	list, err := s.repo.ListFriends(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.UserProfile, 0, len(list))
	for _, p := range list {
		cp := p
		cp.Online = s.redis.Exists(ctx, "presence:"+cp.ID).Val() == 1
		out = append(out, cp)
	}
	return out, nil
}
