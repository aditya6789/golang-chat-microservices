package main

import (
	"context"
	"net/http"
	"time"

	"realtime-chat-system/pkg/infra"
	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/pkg/logger"
	"realtime-chat-system/services/auth-service/config"
	"realtime-chat-system/services/auth-service/internal/handler"
	"realtime-chat-system/services/auth-service/internal/repository"
	"realtime-chat-system/services/auth-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	log, _ := logger.New("auth-service")
	defer log.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := infra.NewPostgres(ctx)
	if err != nil {
		panic(err)
	}

	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, cfg.JWTSecret, cfg.JWTTTLMinute)
	h := handler.NewAuthHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.GET("/auth/validate", h.Validate)

	log.Info("auth-service running")
	if err := r.Run(":" + cfg.Port); err != nil {
		panic(err)
	}
}

