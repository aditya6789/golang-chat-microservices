package handler

import (
	"net/http"

	"realtime-chat-system/services/message-service/internal/repository"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chats *repository.ChatRepository
}

func NewChatHandler(chats *repository.ChatRepository) *ChatHandler {
	return &ChatHandler{chats: chats}
}

func headerUser(c *gin.Context) string {
	return c.GetHeader("X-User-Id")
}

func (h *ChatHandler) Create(c *gin.Context) {
	uid := headerUser(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	var req struct {
		Type       string   `json:"type" binding:"required,oneof=direct group"`
		Name       *string  `json:"name"`
		MemberIDs  []string `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Type == "group" {
		for _, mid := range req.MemberIDs {
			if mid == "" || mid == uid {
				continue
			}
			ok, err := h.chats.AreFriends(c.Request.Context(), uid, mid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "group members must be your friends: " + mid})
				return
			}
		}
	}
	if req.Type == "direct" {
		if len(req.MemberIDs) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direct chat requires exactly one other user in member_ids"})
			return
		}
		ok, err := h.chats.AreFriends(c.Request.Context(), uid, req.MemberIDs[0])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "direct chats are only allowed between friends"})
			return
		}
	}
	id, err := h.chats.CreateChat(c.Request.Context(), req.Type, req.Name, uid, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *ChatHandler) EnsureDirect(c *gin.Context) {
	uid := headerUser(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	var req struct {
		OtherUserID string `json:"other_user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ok, err := h.chats.AreFriends(c.Request.Context(), uid, req.OtherUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "you must be friends before opening a direct chat"})
		return
	}
	id, created, err := h.chats.GetOrCreateDirectChat(c.Request.Context(), uid, req.OtherUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "created": created})
}

func (h *ChatHandler) ListMine(c *gin.Context) {
	uid := headerUser(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	list, err := h.chats.ListByUser(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *ChatHandler) AddMember(c *gin.Context) {
	uid := headerUser(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	chatID := c.Param("chat_id")
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID != uid {
		ok, err := h.chats.AreFriends(c.Request.Context(), uid, req.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only add friends to a group"})
			return
		}
	}
	if err := h.chats.AddMember(c.Request.Context(), chatID, uid, req.UserID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ChatHandler) ListMembers(c *gin.Context) {
	uid := headerUser(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	chatID := c.Param("chat_id")
	ok, err := h.chats.IsMember(c.Request.Context(), chatID, uid)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	members, err := h.chats.ListMembers(c.Request.Context(), chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": members})
}

func (h *ChatHandler) InternalMembership(c *gin.Context) {
	chatID := c.Param("chat_id")
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query required"})
		return
	}
	ok, err := h.chats.IsMember(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"member": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": true})
}
