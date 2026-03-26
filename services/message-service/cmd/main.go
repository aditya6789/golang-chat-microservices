package main

import (
	"context"
	"net/http"
	"time"

	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/services/message-service/config"
	"realtime-chat-system/services/message-service/internal/handler"
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
	svc := service.New(repo, nc)
	h := handler.New(svc)
	_, _ = nc.Subscribe("chat.message.persist", func(msg *nats.Msg) {
		_ = svc.PersistFromEvent(context.Background(), msg.Data)
	})

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/messages", h.Create)
	r.GET("/messages/:chat_id", h.History)
	_ = r.Run(":" + cfg.Port)
}

