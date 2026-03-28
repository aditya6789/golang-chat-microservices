package handler

import (
	"net/http"
	"strconv"

	"realtime-chat-system/services/message-service/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct{ svc *service.MessageService }

func New(svc *service.MessageService) *MessageHandler { return &MessageHandler{svc: svc} }

func (h *MessageHandler) Create(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	var req struct {
		ChatID         string `json:"chat_id" binding:"required"`
		Content        string `json:"content" binding:"required"`
		IdempotencyKey string `json:"idempotency_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.svc.Create(c.Request.Context(), req.ChatID, uid, req.Content, req.IdempotencyKey)
	if err != nil {
		if err.Error() == "forbidden: not a chat member" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *MessageHandler) History(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	chatID := c.Param("chat_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := h.svc.History(c.Request.Context(), chatID, uid, limit, offset)
	if err != nil {
		if err.Error() == "forbidden: not a chat member" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

