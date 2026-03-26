package main

import (
	"net/http"

	"realtime-chat-system/pkg/httpx"
	"realtime-chat-system/services/api-gateway/config"
	"realtime-chat-system/services/api-gateway/internal/handler"
	"realtime-chat-system/services/api-gateway/internal/repository"
	"realtime-chat-system/services/api-gateway/internal/service"
	"realtime-chat-system/services/api-gateway/pkg"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	authRepo := repository.NewAuthRepository()
	svc := service.New(authRepo)
	h := handler.New(svc)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), httpx.ErrorMiddleware(), pkg.RateLimit(100, 200))
	r.GET("/healthz", h.Health)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.Any("/auth/*path", h.AuthProxy)

	protected := r.Group("/")
	protected.Use(pkg.AuthRequired(svc))
	protected.Any("/users/*path", h.UserProxy)
	protected.Any("/messages/*path", h.MessageProxy)
	protected.Any("/chat/*path", h.ChatProxy)
	protected.Any("/ws", h.ChatProxy)
	r.GET("/docs/swagger.yaml", func(c *gin.Context) {
		c.File("docs/swagger.yaml")
	})

	_ = r.Run(":" + cfg.Port)
	_ = http.ErrServerClosed
}

