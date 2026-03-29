package main

import (
	"context"
	"net/http"
	"time"

	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/services/message-service/config"
	"realtime-chat-system/services/message-service/internal/attachment"
	"realtime-chat-system/services/message-service/internal/handler"
	"realtime-chat-system/services/message-service/internal/linkpreview"
	"realtime-chat-system/services/message-service/internal/repository"
	"realtime-chat-system/services/message-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, _ := infra.NewPostgres(ctx)
	nc, _ := infra.NewNATS()

	repo := repository.New(db)
	chatRepo := repository.NewChatRepository(db)
	receiptRepo := repository.NewReceiptRepository(db)
	reactionRepo := repository.NewReactionRepository(db)
	s3cfg := cfg.S3
	store, err := attachment.NewStore(ctx, attachment.Config{
		Endpoint:     s3cfg.Endpoint,
		Region:       s3cfg.Region,
		AccessKey:    s3cfg.AccessKey,
		SecretKey:    s3cfg.SecretKey,
		Bucket:       s3cfg.Bucket,
		PublicBase:   s3cfg.PublicBase,
		UsePathStyle: s3cfg.UsePathStyle,
		MaxBytes:     s3cfg.MaxBytes,
		PresignTTL:   s3cfg.PresignTTL,
	})
	if err != nil {
		panic(err)
	}
	svc := service.New(repo, chatRepo, receiptRepo, reactionRepo, store, nc)
	og := linkpreview.NewService(15*time.Minute, 2000)
	h := handler.New(svc, og)
	ch := handler.NewChatHandler(chatRepo)
	_, _ = nc.Subscribe("chat.message.persist", func(msg *nats.Msg) {
		_ = svc.PersistFromEvent(context.Background(), msg.Data)
	})
	_, _ = nc.Subscribe("chat.receipt.persist", func(msg *nats.Msg) {
		_ = svc.PersistReceiptFromEvent(context.Background(), msg.Data)
	})
	_, _ = nc.Subscribe("chat.reaction.persist", func(msg *nats.Msg) {
		_ = svc.PersistReactionFromEvent(context.Background(), msg.Data)
	})

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/internal/chats/:chat_id/membership", ch.InternalMembership)
	r.POST("/chats/direct", ch.EnsureDirect)
	r.POST("/chats", ch.Create)
	r.GET("/chats", ch.ListMine)
	r.POST("/chats/:chat_id/members", ch.AddMember)
	r.GET("/chats/:chat_id/members", ch.ListMembers)
	r.POST("/chats/:chat_id/attachments/presign", h.PresignAttachment)
	r.POST("/messages", h.Create)
	r.GET("/messages/link-preview", h.LinkPreview)
	r.POST("/messages/:message_id/reactions", h.ToggleReaction)
	r.GET("/messages/:chat_id", h.History)
	_ = r.Run(":" + cfg.Port)
}

