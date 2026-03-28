package main

import (
	"context"
	"net/http"
	"time"

	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/services/user-service/config"
	"realtime-chat-system/services/user-service/internal/handler"
	"realtime-chat-system/services/user-service/internal/repository"
	"realtime-chat-system/services/user-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, _ := infra.NewPostgres(ctx)
	rdb := infra.NewRedis()
	repo := repository.New(db)
	svc := service.New(repo, rdb)
	h := handler.New(svc)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/users/search", h.SearchUsers)
	r.GET("/users/friends", h.ListFriends)
	r.POST("/users/friends", h.AddFriend)
	r.GET("/friends", h.ListFriends)
	r.POST("/friends", h.AddFriend)
	r.GET("/users/:id", h.Profile)
	r.POST("/users/:id/heartbeat", h.Heartbeat)
	_ = r.Run(":" + cfg.Port)
}

