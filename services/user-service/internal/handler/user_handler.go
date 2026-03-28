package handler

import (
	"net/http"

	"realtime-chat-system/services/user-service/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct{ svc *service.UserService }

func New(svc *service.UserService) *UserHandler { return &UserHandler{svc: svc} }

func (h *UserHandler) Profile(c *gin.Context) {
	id := c.Param("id")
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	if uid != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot access other user profile"})
		return
	}
	p, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) Heartbeat(c *gin.Context) {
	id := c.Param("id")
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	if uid != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot heartbeat for another user"})
		return
	}
	if err := h.svc.SetOnline(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "online"})
}

