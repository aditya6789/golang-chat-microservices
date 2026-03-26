package main

import (
	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/pkg/logger"
	"realtime-chat-system/services/notification-service/config"
	"realtime-chat-system/services/notification-service/internal/handler"
	"realtime-chat-system/services/notification-service/internal/repository"
	"realtime-chat-system/services/notification-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	log, _ := logger.New("notification-service")
	nc, _ := infra.NewNATS()
	repo := repository.New(nc)
	svc := service.New(repo, log)
	_ = svc.Start()

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", handler.Health)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	_ = r.Run(":" + cfg.Port)
}

