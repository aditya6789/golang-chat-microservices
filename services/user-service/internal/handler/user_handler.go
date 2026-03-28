package handler

import (
	"errors"
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

func (h *UserHandler) SearchUsers(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	q := c.Query("q")
	list, err := h.svc.SearchUsers(c.Request.Context(), uid, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *UserHandler) ListFriends(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	list, err := h.svc.ListFriends(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *UserHandler) AddFriend(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.AddFriend(c.Request.Context(), uid, req.UserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSelfFriend):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
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

