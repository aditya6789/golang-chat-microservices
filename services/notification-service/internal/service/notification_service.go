package service

import (
	"encoding/json"

	"realtime-chat-system/services/notification-service/internal/model"
	"realtime-chat-system/services/notification-service/internal/repository"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type NotificationService struct {
	repo *repository.EventRepository
	log  *zap.Logger
}

func New(repo *repository.EventRepository, log *zap.Logger) *NotificationService {
	return &NotificationService{repo: repo, log: log}
}

func (s *NotificationService) Start() error {
	_, err := s.repo.Subscribe("chat.message.created", func(msg *nats.Msg) {
		var n model.Notification
		if err := json.Unmarshal(msg.Data, &n); err == nil {
			s.log.Info("notification event", zap.Any("event", n))
		}
	})
	return err
}

