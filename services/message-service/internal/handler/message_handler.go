package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"realtime-chat-system/services/message-service/internal/linkpreview"
	"realtime-chat-system/services/message-service/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type previewRate struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
}

func newPreviewRate() *previewRate {
	return &previewRate{m: make(map[string]*rate.Limiter)}
}

// ~20 previews per minute per user, short burst.
func (p *previewRate) allow(uid string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	l, ok := p.m[uid]
	if !ok {
		l = rate.NewLimiter(rate.Limit(20.0/60.0), 8)
		p.m[uid] = l
	}
	return l.Allow()
}

type MessageHandler struct {
	svc   *service.MessageService
	og    *linkpreview.Service
	prate *previewRate
}

func New(svc *service.MessageService, og *linkpreview.Service) *MessageHandler {
	return &MessageHandler{svc: svc, og: og, prate: newPreviewRate()}
}

// LinkPreview GET /messages/link-preview?url= — must be registered before /messages/:chat_id.
func (h *MessageHandler) LinkPreview(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	if h.og == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "link preview unavailable"})
		return
	}
	if !h.prate.allow(uid) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit: try again later"})
		return
	}
	raw := strings.TrimSpace(c.Query("url"))
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url query required"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 14*time.Second)
	defer cancel()
	p, err := h.og.Fetch(ctx, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *MessageHandler) Create(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	var req struct {
		ChatID           string `json:"chat_id" binding:"required"`
		Content          string `json:"content"`
		MessageType      string `json:"message_type"`
		File             *struct {
			ObjectKey string `json:"object_key" binding:"required"`
			Filename  string `json:"filename" binding:"required"`
			MimeType  string `json:"mime_type" binding:"required"`
			SizeBytes int64  `json:"size_bytes" binding:"required"`
		} `json:"file"`
		IdempotencyKey   string `json:"idempotency_key" binding:"required"`
		ReplyToMessageID string `json:"reply_to_message_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mt := strings.TrimSpace(req.MessageType)
	if mt == "" {
		mt = "text"
	}
	var filePtr *service.FileAttachment
	if mt == "file" {
		if req.File == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file required for message_type file"})
			return
		}
		filePtr = &service.FileAttachment{
			ObjectKey: req.File.ObjectKey,
			Filename:  req.File.Filename,
			MimeType:  req.File.MimeType,
			SizeBytes: req.File.SizeBytes,
		}
	} else if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}
	var replyPtr *string
	if req.ReplyToMessageID != "" {
		replyPtr = &req.ReplyToMessageID
	}
	m, err := h.svc.Create(c.Request.Context(), req.ChatID, uid, req.IdempotencyKey, replyPtr, mt, req.Content, filePtr)
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

// PresignAttachment POST /chats/:chat_id/attachments/presign
func (h *MessageHandler) PresignAttachment(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	chatID := c.Param("chat_id")
	var req struct {
		Filename    string `json:"filename" binding:"required"`
		ContentType string `json:"content_type" binding:"required"`
		SizeBytes   int64  `json:"size_bytes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uploadURL, objectKey, headers, err := h.svc.PresignAttachment(c.Request.Context(), chatID, uid, req.Filename, req.ContentType, req.SizeBytes)
	if err != nil {
		switch err.Error() {
		case "forbidden: not a chat member":
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case "attachments not configured":
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"upload_url": uploadURL, "object_key": objectKey, "headers": headers})
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

// ToggleReaction POST /messages/:message_id/reactions — add or remove an emoji reaction.
func (h *MessageHandler) ToggleReaction(c *gin.Context) {
	uid := c.GetHeader("X-User-Id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	mid := c.Param("message_id")
	var req struct {
		Emoji string `json:"emoji" binding:"required"`
		Add   *bool  `json:"add"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	add := true
	if req.Add != nil {
		add = *req.Add
	}
	err := h.svc.ToggleReaction(c.Request.Context(), uid, mid, req.Emoji, add)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "forbidden: not a chat member":
			c.JSON(http.StatusForbidden, gin.H{"error": msg})
		case "message not found":
			c.JSON(http.StatusNotFound, gin.H{"error": msg})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

