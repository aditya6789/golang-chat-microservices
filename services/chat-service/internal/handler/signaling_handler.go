package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"realtime-chat-system/services/chat-service/internal/authjwt"
	"realtime-chat-system/services/chat-service/internal/client"
	"realtime-chat-system/services/chat-service/internal/service"

	"github.com/gin-gonic/gin"
)

var signalingAllowed = map[string]struct{}{
	"call_invite":  {},
	"call_accept":  {},
	"call_reject":  {},
	"call_end":     {},
	"call_offer":   {},
	"call_answer":  {},
	"call_ice":     {},
}

type SignalingHandler struct {
	hub        *service.SignalingHub
	friendship *client.FriendshipClient
	jwtSecret  []byte
}

func NewSignaling(hub *service.SignalingHub, friendship *client.FriendshipClient, jwtSecret string) *SignalingHandler {
	return &SignalingHandler{hub: hub, friendship: friendship, jwtSecret: []byte(jwtSecret)}
}

func (h *SignalingHandler) Connect(c *gin.Context) {
	userID, err := authjwt.SubjectFromRequest(h.jwtSecret, c.GetHeader("Authorization"), c.Query("access_token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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
	sess := h.hub.Register(userID, conn)
	defer h.hub.Unregister(userID, sess)
	defer conn.Close()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if !h.forward(c.Request.Context(), userID, data) {
			errPayload, _ := json.Marshal(map[string]string{
				"type":    "error",
				"code":    "signaling_rejected",
				"message": "not allowed or invalid message",
			})
			_ = sess.WriteText(errPayload)
		}
	}
}

func (h *SignalingHandler) forward(ctx context.Context, fromUserID string, data []byte) bool {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	typ, _ := raw["type"].(string)
	if _, ok := signalingAllowed[typ]; !ok {
		return false
	}
	toID, _ := raw["to_user_id"].(string)
	if toID == "" {
		return false
	}
	callID, _ := raw["call_id"].(string)
	if callID == "" {
		return false
	}
	if !h.friendship.AreFriends(ctx, fromUserID, toID) {
		return false
	}
	delete(raw, "from_user_id")
	raw["from_user_id"] = fromUserID
	out, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	h.hub.SendToUser(toID, out)
	return true
}
