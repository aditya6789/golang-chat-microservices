package handler

import (
	"net/http"

	"realtime-chat-system/services/api-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

type GatewayHandler struct {
	svc *service.GatewayService
}

func New(svc *service.GatewayService) *GatewayHandler { return &GatewayHandler{svc: svc} }

func (h *GatewayHandler) AuthProxy(c *gin.Context)    { h.svc.Proxy("http://auth-service:8081").ServeHTTP(c.Writer, c.Request) }
func (h *GatewayHandler) UserProxy(c *gin.Context)    { h.svc.Proxy("http://user-service:8082").ServeHTTP(c.Writer, c.Request) }
func (h *GatewayHandler) MessageProxy(c *gin.Context) { h.svc.Proxy("http://message-service:8084").ServeHTTP(c.Writer, c.Request) }
func (h *GatewayHandler) ChatProxy(c *gin.Context)    { h.svc.Proxy("http://chat-service:8083").ServeHTTP(c.Writer, c.Request) }

func (h *GatewayHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

