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
	var req struct {
		ChatID         string `json:"chat_id" binding:"required"`
		SenderID       string `json:"sender_id" binding:"required"`
		Content        string `json:"content" binding:"required"`
		IdempotencyKey string `json:"idempotency_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.svc.Create(c.Request.Context(), req.ChatID, req.SenderID, req.Content, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *MessageHandler) History(c *gin.Context) {
	chatID := c.Param("chat_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := h.svc.History(c.Request.Context(), chatID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

