package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"realtime-chat-system/services/chat-service/internal/model"
	"realtime-chat-system/services/chat-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	hub *service.Hub
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func New(hub *service.Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

func (h *WebSocketHandler) Connect(c *gin.Context) {
	userID := c.Query("user_id")
	chatID := c.Query("chat_id")
	if userID == "" || chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and chat_id required"})
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
	h.hub.Register(userID, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.hub.StreamChat(ctx, userID, chatID)

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
	h.hub.Unregister(userID)
	_ = conn.Close()
}

