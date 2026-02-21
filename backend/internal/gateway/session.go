package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/mcp"
	"github.com/reflow/gateway/internal/stdio"
	"github.com/reflow/gateway/internal/telemetry"
	"github.com/rs/zerolog/log"
)

// ToolMapping maps a tool name to its upstream target
type ToolMapping struct {
	TargetID   uuid.UUID
	TargetName string
	ToolName   string // original (unprefixed) tool name
}

// ResourceMapping maps a resource URI to its upstream target
type ResourceMapping struct {
	TargetID   uuid.UUID
	TargetName string
	URI        string // original (unprefixed) URI
}

// PromptMapping maps a prompt name to its upstream target
type PromptMapping struct {
	TargetID   uuid.UUID
	TargetName string
	PromptName string // original (unprefixed) prompt name
}

// Session represents an active MCP session
type Session struct {
	ID           string
	UserID       uuid.UUID
	Role         string
	Groups       []string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	clients      map[string]mcp.MCPClient // map[targetName]MCPClient
	mu           sync.RWMutex
	initialized  bool
	capabilities *mcp.ServerCapabilities
	targetIDs    map[string]uuid.UUID       // targetName -> targetID
	toolMap      map[string]ToolMapping     // prefixedName -> mapping
	resourceMap  map[string]ResourceMapping // prefixedURI -> mapping
	promptMap    map[string]PromptMapping   // prefixedName -> mapping
}

// SessionManager manages MCP sessions
type SessionManager struct {
	repo            *database.Repository
	sessions        map[string]*Session
	mu              sync.RWMutex
	sessionTimeout  time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(repo *database.Repository, timeout, cleanupInterval time.Duration) *SessionManager {
	sm := &SessionManager{
		repo:            repo,
		sessions:        make(map[string]*Session),
		sessionTimeout:  timeout,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go sm.cleanupLoop()

	return sm
}

// CreateSession creates a new MCP session
func (sm *SessionManager) CreateSession(ctx context.Context, userID uuid.UUID, role string, groups []string) (*Session, error) {
	sessionID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(sm.sessionTimeout)

	// Store in database
	_, err := sm.repo.CreateMCPSession(ctx, sessionID, userID, expiresAt)
	if err != nil {
		return nil, err
	}

	if groups == nil {
		groups = []string{}
	}

	session := &Session{
		ID:          sessionID,
		UserID:      userID,
		Role:        role,
		Groups:      groups,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		clients:     make(map[string]mcp.MCPClient),
		targetIDs:   make(map[string]uuid.UUID),
		toolMap:     make(map[string]ToolMapping),
		resourceMap: make(map[string]ResourceMapping),
		promptMap:   make(map[string]PromptMapping),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	telemetry.MCPSessionsActive.Add(ctx, 1)

	log.Info().
		Str("session_id", sessionID).
		Str("user_id", userID.String()).
		Str("role", role).
		Strs("groups", groups).
		Msg("Created new MCP session")

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if exists {
		// Check if expired
		if time.Now().After(session.ExpiresAt) {
			sm.DeleteSession(ctx, sessionID)
			return nil, database.ErrNotFound
		}

		// Update activity
		session.mu.Lock()
		session.ExpiresAt = time.Now().Add(sm.sessionTimeout)
		session.mu.Unlock()

		go sm.repo.UpdateMCPSessionActivity(ctx, sessionID)

		return session, nil
	}

	// Try to load from database
	dbSession, err := sm.repo.GetMCPSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Check if expired
	if time.Now().After(dbSession.ExpiresAt) {
		sm.repo.DeleteMCPSession(ctx, sessionID)
		return nil, database.ErrNotFound
	}

	// Get user to retrieve role and groups
	user, err := sm.repo.GetUserByID(ctx, dbSession.UserID)
	role := "user"
	groups := []string{}
	if err == nil {
		role = user.Role
		groups = user.Groups
	}

	session = &Session{
		ID:          dbSession.ID,
		UserID:      dbSession.UserID,
		Role:        role,
		Groups:      groups,
		CreatedAt:   dbSession.CreatedAt,
		ExpiresAt:   dbSession.ExpiresAt,
		clients:     make(map[string]mcp.MCPClient),
		targetIDs:   make(map[string]uuid.UUID),
		toolMap:     make(map[string]ToolMapping),
		resourceMap: make(map[string]ResourceMapping),
		promptMap:   make(map[string]PromptMapping),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	return session, nil
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	session, exists := sm.sessions[sessionID]
	if exists {
		// Close HTTP clients only; STDIO processes are managed by StdioManager
		session.mu.Lock()
		for _, client := range session.clients {
			if _, isStdio := client.(*stdio.Process); !isStdio {
				client.Close()
			}
		}
		session.mu.Unlock()
		delete(sm.sessions, sessionID)
		telemetry.MCPSessionsActive.Add(ctx, -1)
	}
	sm.mu.Unlock()

	// Delete from database
	return sm.repo.DeleteMCPSession(ctx, sessionID)
}

// GetClient gets or creates an MCP client for a target within a session
func (s *Session) GetClient(targetName string) mcp.MCPClient {
	s.mu.RLock()
	client := s.clients[targetName]
	s.mu.RUnlock()
	return client
}

// SetClient sets an MCP client for a target within a session
func (s *Session) SetClient(targetName string, client mcp.MCPClient) {
	s.mu.Lock()
	s.clients[targetName] = client
	s.mu.Unlock()
}

// GetAllClients returns all clients in the session
func (s *Session) GetAllClients() map[string]mcp.MCPClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make(map[string]mcp.MCPClient)
	for k, v := range s.clients {
		clients[k] = v
	}
	return clients
}

// SetTargetID stores the target ID for a target name
func (s *Session) SetTargetID(targetName string, targetID uuid.UUID) {
	s.mu.Lock()
	s.targetIDs[targetName] = targetID
	s.mu.Unlock()
}

// GetTargetID returns the target ID for a target name
func (s *Session) GetTargetID(targetName string) (uuid.UUID, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.targetIDs[targetName]
	return id, ok
}

// SetToolMapping stores a tool mapping
func (s *Session) SetToolMapping(prefixedName string, mapping ToolMapping) {
	s.mu.Lock()
	s.toolMap[prefixedName] = mapping
	s.mu.Unlock()
}

// GetToolMapping retrieves a tool mapping
func (s *Session) GetToolMapping(prefixedName string) (ToolMapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.toolMap[prefixedName]
	return m, ok
}

// ClearToolMappings resets tool mappings
func (s *Session) ClearToolMappings() {
	s.mu.Lock()
	s.toolMap = make(map[string]ToolMapping)
	s.mu.Unlock()
}

// SetResourceMapping stores a resource mapping
func (s *Session) SetResourceMapping(prefixedURI string, mapping ResourceMapping) {
	s.mu.Lock()
	s.resourceMap[prefixedURI] = mapping
	s.mu.Unlock()
}

// GetResourceMapping retrieves a resource mapping
func (s *Session) GetResourceMapping(prefixedURI string) (ResourceMapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.resourceMap[prefixedURI]
	return m, ok
}

// ClearResourceMappings resets resource mappings
func (s *Session) ClearResourceMappings() {
	s.mu.Lock()
	s.resourceMap = make(map[string]ResourceMapping)
	s.mu.Unlock()
}

// SetPromptMapping stores a prompt mapping
func (s *Session) SetPromptMapping(prefixedName string, mapping PromptMapping) {
	s.mu.Lock()
	s.promptMap[prefixedName] = mapping
	s.mu.Unlock()
}

// GetPromptMapping retrieves a prompt mapping
func (s *Session) GetPromptMapping(prefixedName string) (PromptMapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.promptMap[prefixedName]
	return m, ok
}

// ClearPromptMappings resets prompt mappings
func (s *Session) ClearPromptMappings() {
	s.mu.Lock()
	s.promptMap = make(map[string]PromptMapping)
	s.mu.Unlock()
}

// SetInitialized marks the session as initialized
func (s *Session) SetInitialized(caps *mcp.ServerCapabilities) {
	s.mu.Lock()
	s.initialized = true
	s.capabilities = caps
	s.mu.Unlock()
}

// IsInitialized returns whether the session has been initialized
func (s *Session) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// GetCapabilities returns the aggregated capabilities
func (s *Session) GetCapabilities() *mcp.ServerCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.capabilities
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanup()
		case <-sm.stopCleanup:
			return
		}
	}
}

func (sm *SessionManager) cleanup() {
	ctx := context.Background()

	// Cleanup in-memory sessions
	sm.mu.Lock()
	now := time.Now()
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			session.mu.Lock()
			for _, client := range session.clients {
				// Skip STDIO processes; they're managed by StdioManager
				if _, isStdio := client.(*stdio.Process); !isStdio {
					client.Close()
				}
			}
			session.mu.Unlock()
			delete(sm.sessions, id)
			telemetry.MCPSessionsActive.Add(ctx, -1)
			log.Debug().Str("session_id", id).Msg("Cleaned up expired session")
		}
	}
	sm.mu.Unlock()

	// Cleanup database sessions
	count, err := sm.repo.CleanupExpiredSessions(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup expired sessions from database")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("Cleaned up expired sessions from database")
	}
}

// Recycle resets a session's upstream connections and updates identity context.
// HTTP clients are closed; STDIO processes are left to StdioManager.
func (s *Session) Recycle(newRole string, newGroups []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close HTTP clients only (STDIO managed by StdioManager)
	for _, client := range s.clients {
		if _, isStdio := client.(*stdio.Process); !isStdio {
			client.Close()
		}
	}

	// Reset session state
	s.clients = make(map[string]mcp.MCPClient)
	s.targetIDs = make(map[string]uuid.UUID)
	s.toolMap = make(map[string]ToolMapping)
	s.resourceMap = make(map[string]ResourceMapping)
	s.promptMap = make(map[string]PromptMapping)
	s.initialized = false
	s.capabilities = nil

	// Update identity context
	s.Role = newRole
	s.Groups = newGroups

	log.Info().
		Str("session_id", s.ID).
		Str("new_role", newRole).
		Strs("new_groups", newGroups).
		Msg("Session recycled")
}

// NeedsRecycle returns true if the given role/groups differ from the session's stored values.
func (s *Session) NeedsRecycle(role string, groups []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Role != role {
		return true
	}

	if groups == nil {
		groups = []string{}
	}
	sessionGroups := s.Groups
	if sessionGroups == nil {
		sessionGroups = []string{}
	}

	if len(sessionGroups) != len(groups) {
		return true
	}

	// Build a set for comparison (order-independent)
	groupSet := make(map[string]struct{}, len(sessionGroups))
	for _, g := range sessionGroups {
		groupSet[g] = struct{}{}
	}
	for _, g := range groups {
		if _, ok := groupSet[g]; !ok {
			return true
		}
	}

	return false
}

// RecycleUserSessions recycles all sessions belonging to a user, returning the count.
func (sm *SessionManager) RecycleUserSessions(ctx context.Context, userID uuid.UUID) int {
	// Load fresh user data from DB
	user, err := sm.repo.GetUserByID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to load user for session recycle")
		return 0
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.UserID == userID {
			session.Recycle(user.Role, user.Groups)
			count++
		}
	}

	if count > 0 {
		log.Info().
			Str("user_id", userID.String()).
			Int("recycled", count).
			Msg("Recycled user sessions")
	}

	return count
}

// Stop stops the session manager cleanup goroutine
func (sm *SessionManager) Stop() {
	close(sm.stopCleanup)
}
