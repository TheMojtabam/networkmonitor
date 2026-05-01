// Package ws is a WebSocket hub that pushes Snapshot messages to all
// connected clients. Subscribers register on /ws and receive a JSON
// Snapshot for every tick of the sampler.
package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Logger is the minimal logging interface used here.
type Logger interface {
	Printf(format string, args ...any)
}

// Hub broadcasts snapshots to a fan-out of WebSocket clients.
type Hub struct {
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	clients  map[*client]struct{}
	logger   Logger
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub constructs the hub. checkOrigin allows wiring CORS rules; pass
// nil to permit any origin (useful in dev).
func NewHub(logger Logger, checkOrigin func(r *http.Request) bool) *Hub {
	if checkOrigin == nil {
		checkOrigin = func(r *http.Request) bool { return true }
	}
	return &Hub{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin:     checkOrigin,
		},
		clients: map[*client]struct{}{},
		logger:  logger,
	}
}

// Handler upgrades HTTP to WebSocket and registers the new client.
func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := h.upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Printf("ws upgrade: %v", err)
			return
		}
		c := &client{conn: conn, send: make(chan []byte, 8)}
		h.mu.Lock()
		h.clients[c] = struct{}{}
		h.mu.Unlock()
		go h.writeLoop(c)
		go h.readLoop(c)
	}
}

// Pump consumes snapshots from `in` and broadcasts them. Blocks until ch closes.
func (h *Hub) Pump(in <-chan t.Snapshot) {
	for snap := range in {
		data, err := json.Marshal(map[string]any{
			"type": "snapshot",
			"data": snap,
		})
		if err != nil {
			continue
		}
		h.broadcast(data)
	}
}

// SendEvent pushes an arbitrary event (e.g. an alert firing) to all clients.
func (h *Hub) SendEvent(eventType string, payload any) {
	data, err := json.Marshal(map[string]any{
		"type": eventType,
		"data": payload,
	})
	if err != nil {
		return
	}
	h.broadcast(data)
}

func (h *Hub) broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Drop on slow client; they'll catch up on the next snapshot.
		}
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		_ = c.conn.Close()
	}
	h.mu.Unlock()
}

func (h *Hub) writeLoop(c *client) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				h.remove(c)
				return
			}
		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.remove(c)
				return
			}
		}
	}
}

func (h *Hub) readLoop(c *client) {
	defer h.remove(c)
	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		// We don't expect client messages; reading just lets us detect disconnects.
		if _, _, err := c.conn.NextReader(); err != nil {
			return
		}
	}
}
