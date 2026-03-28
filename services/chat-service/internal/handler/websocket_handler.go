package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"realtime-chat-system/services/chat-service/internal/authjwt"
	"realtime-chat-system/services/chat-service/internal/model"
	"realtime-chat-system/services/chat-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	hub       *service.Hub
	jwtSecret []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func New(hub *service.Hub, jwtSecret string) *WebSocketHandler {
	return &WebSocketHandler{hub: hub, jwtSecret: []byte(jwtSecret)}
}

func connKey(userID, chatID string) string {
	return userID + ":" + chatID
}

func (h *WebSocketHandler) Connect(c *gin.Context) {
	userID, err := authjwt.SubjectFromRequest(h.jwtSecret, c.GetHeader("Authorization"), c.Query("access_token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	chatID := c.Query("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat_id required"})
		return
	}
	if !h.hub.VerifyMember(c.Request.Context(), chatID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this chat"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	key := connKey(userID, chatID)
	h.hub.Register(key, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.hub.StreamChat(ctx, key, chatID)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var e model.Event
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		e.ChatID = chatID
		e.SenderID = userID
		if e.Type == "" {
			e.Type = "message"
		}
		e.At = time.Now().UTC()
		_ = h.hub.HandleInbound(c.Request.Context(), e)
	}
	h.hub.Unregister(key)
	_ = conn.Close()
}

