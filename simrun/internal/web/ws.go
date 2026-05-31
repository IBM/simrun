package web

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub maintains active WebSocket clients and broadcasts messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// Client represents a single WebSocket connection.
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	send  chan []byte
	mu    sync.RWMutex
	runID string // filter: only receive events for this run
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run processes register/unregister/broadcast events.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			var toRemove []*Client
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					toRemove = append(toRemove, client)
				}
			}
			h.mu.RUnlock()

			if len(toRemove) > 0 {
				h.mu.Lock()
				for _, client := range toRemove {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// BroadcastToRun sends a message to all clients subscribed to a specific run.
func (h *Hub) BroadcastToRun(runID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Error("Failed to marshal WebSocket message")
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.mu.RLock()
		rid := client.runID
		client.mu.RUnlock()
		if rid == runID || rid == "" {
			select {
			case client.send <- data:
			default:
				// Client buffer full, skip
			}
		}
	}
}

// ServeWS upgrades an HTTP connection to a WebSocket and registers the client.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("WebSocket upgrade failed")
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump handles incoming messages from the WebSocket client.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.WithError(err).Debug("WebSocket read error")
			}
			break
		}

		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue // Ignore invalid messages
		}

		switch msg.Type {
		case "subscribe":
			var sub struct {
				RunID string `json:"runId"`
			}
			if err := json.Unmarshal(msg.Data, &sub); err == nil {
				c.mu.Lock()
				c.runID = sub.RunID
				c.mu.Unlock()
			}
		case "ping":
			c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
	}
}

// writePump sends messages from the send channel to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
