package observability

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// EventType identifies the kind of event pushed over WebSocket.
type EventType string

const (
	EventActivity EventType = "activity"
	EventMetrics  EventType = "metrics"
	EventSession  EventType = "session"
	EventError    EventType = "error"
)

// Event is a message sent to WebSocket clients.
type Event struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// ActivityEvent represents a single MCP request activity.
type ActivityEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email,omitempty"`
	Method     string    `json:"method"`
	Target     string    `json:"target,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	DurationMS float64   `json:"duration_ms"`
	Status     string    `json:"status"` // "ok" or "error"
	TraceID    string    `json:"trace_id,omitempty"`
}

// SessionEvent represents a session lifecycle event.
type SessionEvent struct {
	Event     string   `json:"event"` // "created", "deleted", "recycled"
	SessionID string   `json:"session_id"`
	UserID    string   `json:"user_id"`
	Targets   []string `json:"targets,omitempty"`
}

// ErrorEvent represents an error in the gateway.
type ErrorEvent struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
	Target    string    `json:"target,omitempty"`
	ErrorType string    `json:"error_type"`
	Message   string    `json:"message"`
}

// Hub manages WebSocket connections and broadcasts events to admin clients.
type Hub struct {
	clients    map[*wsClient]struct{}
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
	aggregator *Aggregator
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub creates a new WebSocket hub with an aggregator.
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*wsClient]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		aggregator: NewAggregator(),
	}
	go h.run()
	go h.metricsLoop()
	return h
}

// GetAggregator returns the metric aggregator for reading stats.
func (h *Hub) GetAggregator() *Aggregator {
	return h.aggregator
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
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
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// metricsLoop periodically broadcasts aggregated metrics.
func (h *Hub) metricsLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.RLock()
		count := len(h.clients)
		h.mu.RUnlock()
		if count == 0 {
			continue
		}
		snapshot := h.aggregator.Snapshot()
		h.Publish(Event{Type: EventMetrics, Data: snapshot})
	}
}

// Publish sends an event to all connected WebSocket clients.
func (h *Hub) Publish(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	select {
	case h.broadcast <- data:
	default:
		// Drop if broadcast buffer is full
	}
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and registers it.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 64),
	}

	h.register <- client

	go h.writePump(client)
	go h.readPump(client)
}

func (h *Hub) writePump(c *wsClient) {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (h *Hub) readPump(c *wsClient) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()
	// Read messages (subscribe/unsubscribe); for now just keep alive.
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

// --- Convenience methods to emit specific events ---

// EmitActivity publishes an activity event and records it in the aggregator.
func (h *Hub) EmitActivity(e ActivityEvent) {
	h.aggregator.Record(e)
	h.Publish(Event{Type: EventActivity, Data: e})
}

// EmitSession publishes a session lifecycle event.
func (h *Hub) EmitSession(e SessionEvent) {
	h.Publish(Event{Type: EventSession, Data: e})
}

// EmitError publishes an error event.
func (h *Hub) EmitError(e ErrorEvent) {
	h.Publish(Event{Type: EventError, Data: e})
}
