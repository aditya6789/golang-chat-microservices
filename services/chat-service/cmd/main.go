package main

import (
	"net/http"

	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/services/chat-service/config"
	"realtime-chat-system/services/chat-service/internal/client"
	"realtime-chat-system/services/chat-service/internal/handler"
	"realtime-chat-system/services/chat-service/internal/repository"
	"realtime-chat-system/services/chat-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	rdb := infra.NewRedis()
	nc, _ := infra.NewNATS()
	repo := repository.New(rdb)
	members := &client.MembershipClient{BaseURL: cfg.MessageServiceURL}
	hub := service.NewHub(repo, nc, members)
	hub.StartMessageFanout()
	h := handler.New(hub, cfg.JWTSecret)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/ws", h.Connect)
	_ = r.Run(":" + cfg.Port)
}

