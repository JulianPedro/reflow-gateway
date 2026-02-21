package gateway

import (
	"context"
	"sync"

	"github.com/reflow/gateway/internal/mcp"
	"github.com/rs/zerolog/log"
)

// SSEHub manages SSE connections for a session
type SSEHub struct {
	sessionID   string
	connections map[string]*SSEConnection
	mu          sync.RWMutex
	broadcast   chan *mcp.SSEEvent
	done        chan struct{}
}

// SSEConnection represents a single SSE connection
type SSEConnection struct {
	ID     string
	Writer *mcp.SSEWriter
	Done   chan struct{}
}

// NewSSEHub creates a new SSE hub for a session
func NewSSEHub(sessionID string) *SSEHub {
	hub := &SSEHub{
		sessionID:   sessionID,
		connections: make(map[string]*SSEConnection),
		broadcast:   make(chan *mcp.SSEEvent, 100),
		done:        make(chan struct{}),
	}

	go hub.run()

	return hub
}

func (h *SSEHub) run() {
	for {
		select {
		case event := <-h.broadcast:
			h.mu.RLock()
			for id, conn := range h.connections {
				if err := conn.Writer.WriteEvent(event); err != nil {
					log.Error().Err(err).Str("connection", id).Msg("Failed to write SSE event")
					// Mark for removal
					go h.RemoveConnection(id)
				}
			}
			h.mu.RUnlock()
		case <-h.done:
			return
		}
	}
}

// AddConnection adds a new SSE connection
func (h *SSEHub) AddConnection(id string, writer *mcp.SSEWriter) *SSEConnection {
	conn := &SSEConnection{
		ID:     id,
		Writer: writer,
		Done:   make(chan struct{}),
	}

	h.mu.Lock()
	h.connections[id] = conn
	h.mu.Unlock()

	log.Debug().
		Str("session_id", h.sessionID).
		Str("connection_id", id).
		Msg("Added SSE connection")

	return conn
}

// RemoveConnection removes an SSE connection
func (h *SSEHub) RemoveConnection(id string) {
	h.mu.Lock()
	if conn, ok := h.connections[id]; ok {
		close(conn.Done)
		delete(h.connections, id)
	}
	h.mu.Unlock()

	log.Debug().
		Str("session_id", h.sessionID).
		Str("connection_id", id).
		Msg("Removed SSE connection")
}

// Broadcast sends an event to all connections
func (h *SSEHub) Broadcast(event *mcp.SSEEvent) {
	select {
	case h.broadcast <- event:
	default:
		log.Warn().Str("session_id", h.sessionID).Msg("Broadcast channel full, dropping event")
	}
}

// BroadcastNotification broadcasts a JSON-RPC notification
func (h *SSEHub) BroadcastNotification(notification *mcp.JSONRPCNotification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for id, conn := range h.connections {
		if err := conn.Writer.WriteJSONRPCNotification(notification); err != nil {
			log.Error().Err(err).Str("connection", id).Msg("Failed to write notification")
		}
	}
}

// Close closes the hub and all connections
func (h *SSEHub) Close() {
	close(h.done)

	h.mu.Lock()
	for id, conn := range h.connections {
		close(conn.Done)
		delete(h.connections, id)
	}
	h.mu.Unlock()
}

// ConnectionCount returns the number of active connections
func (h *SSEHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// SSEManager manages SSE hubs for all sessions
type SSEManager struct {
	hubs map[string]*SSEHub
	mu   sync.RWMutex
}

// NewSSEManager creates a new SSE manager
func NewSSEManager() *SSEManager {
	return &SSEManager{
		hubs: make(map[string]*SSEHub),
	}
}

// GetOrCreateHub gets or creates an SSE hub for a session
func (m *SSEManager) GetOrCreateHub(sessionID string) *SSEHub {
	m.mu.RLock()
	hub, exists := m.hubs[sessionID]
	m.mu.RUnlock()

	if exists {
		return hub
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if hub, exists = m.hubs[sessionID]; exists {
		return hub
	}

	hub = NewSSEHub(sessionID)
	m.hubs[sessionID] = hub
	return hub
}

// RemoveHub removes and closes an SSE hub
func (m *SSEManager) RemoveHub(sessionID string) {
	m.mu.Lock()
	if hub, exists := m.hubs[sessionID]; exists {
		hub.Close()
		delete(m.hubs, sessionID)
	}
	m.mu.Unlock()
}

// BroadcastToSession broadcasts an event to all connections in a session
func (m *SSEManager) BroadcastToSession(sessionID string, event *mcp.SSEEvent) {
	m.mu.RLock()
	hub, exists := m.hubs[sessionID]
	m.mu.RUnlock()

	if exists {
		hub.Broadcast(event)
	}
}

// UpstreamSSEHandler handles SSE events from upstream servers
type UpstreamSSEHandler struct {
	sseManager *SSEManager
	proxy      *Proxy
}

// NewUpstreamSSEHandler creates a new upstream SSE handler
func NewUpstreamSSEHandler(sseManager *SSEManager, proxy *Proxy) *UpstreamSSEHandler {
	return &UpstreamSSEHandler{
		sseManager: sseManager,
		proxy:      proxy,
	}
}

// HandleUpstreamEvents listens for events from an upstream server and broadcasts them
func (h *UpstreamSSEHandler) HandleUpstreamEvents(ctx context.Context, sessionID string, targetName string, events <-chan *mcp.SSEEvent) {
	hub := h.sseManager.GetOrCreateHub(sessionID)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			// Optionally transform or filter events here
			hub.Broadcast(event)
		}
	}
}
