package service

import (
	"sync"

	"github.com/gorilla/websocket"
)

// SignalingHub tracks WebSocket connections per user (multiple tabs allowed).
type SignalingHub struct {
	mu     sync.RWMutex
	byUser map[string][]*SignalingSession
}

// SignalingSession is one WebSocket connection for signaling (e.g. one browser tab).
type SignalingSession struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *SignalingSession) WriteText(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

func NewSignalingHub() *SignalingHub {
	return &SignalingHub{byUser: make(map[string][]*SignalingSession)}
}

func (h *SignalingHub) Register(userID string, c *websocket.Conn) *SignalingSession {
	sc := &SignalingSession{conn: c}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byUser[userID] = append(h.byUser[userID], sc)
	return sc
}

func (h *SignalingHub) Unregister(userID string, sc *SignalingSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.byUser[userID]
	out := list[:0]
	for _, x := range list {
		if x != sc {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		delete(h.byUser, userID)
	} else {
		h.byUser[userID] = out
	}
}

// SendToUser delivers a text message to every connection for that user.
func (h *SignalingHub) SendToUser(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sc := range h.byUser[userID] {
		_ = sc.WriteText(data)
	}
}
