package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// SessionRecycler recycles MCP sessions for a user.
type SessionRecycler interface {
	RecycleUserSessions(ctx context.Context, userID uuid.UUID) int
}

// InstanceRestarter restarts K8s MCPInstances for a target.
type InstanceRestarter interface {
	RestartTarget(ctx context.Context, targetName string) (int, error)
}

// Handlers contains all API handlers
type Handlers struct {
	repo               *database.Repository
	jwtManager         *auth.JWTManager
	encryptor          *auth.TokenEncryptor
	sessionRecycler    SessionRecycler
	instanceRestarter  InstanceRestarter
}

// NewHandlers creates new API handlers
func NewHandlers(repo *database.Repository, jwtManager *auth.JWTManager, encryptor *auth.TokenEncryptor, sessionRecycler SessionRecycler, instanceRestarter InstanceRestarter) *Handlers {
	return &Handlers{
		repo:              repo,
		jwtManager:        jwtManager,
		encryptor:         encryptor,
		sessionRecycler:   sessionRecycler,
		instanceRestarter: instanceRestarter,
	}
}

// ==================== Auth Handlers ====================

// Register handles user registration
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req database.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "Password must be at least 6 characters")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Check if this is the first user (will become admin)
	userCount, err := h.repo.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to check user count")
		return
	}

	user, err := h.repo.CreateUser(r.Context(), req.Email, string(hash))
	if err != nil {
		if err == database.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "Email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// First registered user gets admin role
	if userCount == 0 {
		adminRole := "admin"
		user, err = h.repo.UpdateUser(r.Context(), user.ID, &database.UpdateUserRequest{Role: &adminRole})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to assign admin role")
			return
		}
	}

	// Generate JWT token with role and groups
	token, jti, err := h.jwtManager.GenerateToken(user.ID, user.Email, user.Role, user.Groups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Store API token
	_, err = h.repo.CreateAPIToken(r.Context(), user.ID, jti, "Default")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to store token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// Login handles user login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req database.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Generate JWT token with role and groups
	token, jti, err := h.jwtManager.GenerateToken(user.ID, user.Email, user.Role, user.Groups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Store API token
	_, err = h.repo.CreateAPIToken(r.Context(), user.ID, jti, "Login")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to store token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// GetCurrentUser returns the current authenticated user
func (h *Handlers) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ListAPITokens lists all API tokens for the current user
func (h *Handlers) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tokens, err := h.repo.GetAPITokensByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get tokens")
		return
	}

	if tokens == nil {
		tokens = []*database.APIToken{}
	}

	writeJSON(w, http.StatusOK, tokens)
}

// CreateAPIToken creates a new API token
func (h *Handlers) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req database.CreateAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	token, jti, err := h.jwtManager.GenerateToken(userID, user.Email, user.Role, user.Groups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	apiToken, err := h.repo.CreateAPIToken(r.Context(), userID, jti, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"api_token": apiToken,
		"token":     token,
	})
}

// RevokeAPIToken revokes an API token
func (h *Handlers) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid token ID")
		return
	}

	if err := h.repo.RevokeAPIToken(r.Context(), id, userID); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to revoke token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== User Management Handlers ====================

// ListUsers lists all users (admin only)
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	users, err := h.repo.GetAllUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get users")
		return
	}

	if users == nil {
		users = []*database.User{}
	}

	// Remove password hashes from response
	for _, u := range users {
		u.PasswordHash = ""
	}

	writeJSON(w, http.StatusOK, users)
}

// UpdateUser updates a user's role and groups (admin only)
func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req database.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.repo.UpdateUser(r.Context(), id, &req)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "User not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

// RecycleSessions recycles all MCP sessions for a user.
// POST /api/sessions/recycle — Recycle own sessions
// POST /api/users/{id}/recycle — Admin: recycle another user's sessions
func (h *Handlers) RecycleSessions(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Determine target user
	var targetUserID uuid.UUID
	idStr := chi.URLParam(r, "id")
	if idStr != "" {
		// Admin recycling another user's sessions
		role, _ := auth.GetUserRole(r.Context())
		if role != "admin" {
			writeError(w, http.StatusForbidden, "Admin access required")
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid user ID")
			return
		}
		targetUserID = id
	} else {
		// User recycling their own sessions
		targetUserID = callerID
	}

	count := h.sessionRecycler.RecycleUserSessions(r.Context(), targetUserID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recycled": count,
		"user_id":  targetUserID.String(),
	})
}

// ==================== Target Handlers ====================

// ListTargets lists all targets
func (h *Handlers) ListTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.repo.GetAllTargets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get targets")
		return
	}

	if targets == nil {
		targets = []*database.Target{}
	}

	writeJSON(w, http.StatusOK, targets)
}

// CreateTarget creates a new target
func (h *Handlers) CreateTarget(w http.ResponseWriter, r *http.Request) {
	var req database.CreateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	if req.AuthType == "" {
		req.AuthType = "none"
	}

	if req.TransportType == "" {
		req.TransportType = "streamable-http"
	}

	// Validate transport-specific requirements
	if req.TransportType == "stdio" {
		if req.Command == "" {
			writeError(w, http.StatusBadRequest, "Command is required for STDIO transport")
			return
		}
	} else if req.TransportType == "kubernetes" {
		if req.Image == "" {
			writeError(w, http.StatusBadRequest, "Image is required for Kubernetes transport")
			return
		}
	} else {
		if req.URL == "" {
			writeError(w, http.StatusBadRequest, "URL is required for HTTP/SSE transport")
			return
		}
	}

	// Validate statefulness
	if req.Statefulness != "" {
		valid := map[string]bool{"stateless": true, "stateful": true}
		if !valid[req.Statefulness] {
			writeError(w, http.StatusBadRequest, "Invalid statefulness value")
			return
		}
	}

	// Validate isolation boundary
	if req.IsolationBoundary != "" {
		valid := map[string]bool{"shared": true, "per_group": true, "per_role": true, "per_user": true}
		if !valid[req.IsolationBoundary] {
			writeError(w, http.StatusBadRequest, "Invalid isolation_boundary value")
			return
		}
	}

	target, err := h.repo.CreateTarget(r.Context(), &req)
	if err != nil {
		if err == database.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "Target name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create target")
		return
	}

	writeJSON(w, http.StatusCreated, target)
}

// GetTarget gets a target by ID
func (h *Handlers) GetTarget(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	target, err := h.repo.GetTargetByID(r.Context(), id)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	writeJSON(w, http.StatusOK, target)
}

// UpdateTarget updates a target
func (h *Handlers) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	var req database.UpdateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	target, err := h.repo.UpdateTarget(r.Context(), id, &req)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		if err == database.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "Target name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update target")
		return
	}

	writeJSON(w, http.StatusOK, target)
}

// DeleteTarget deletes a target
func (h *Handlers) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	if err := h.repo.DeleteTarget(r.Context(), id); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete target")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestartInstances deletes all K8s MCPInstances for a target,
// forcing them to be recreated with updated config on next connection.
func (h *Handlers) RestartInstances(w http.ResponseWriter, r *http.Request) {
	if h.instanceRestarter == nil {
		writeError(w, http.StatusBadRequest, "Kubernetes transport is not enabled")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	target, err := h.repo.GetTargetByID(r.Context(), id)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	if target.TransportType != "kubernetes" {
		writeError(w, http.StatusBadRequest, "Target is not a Kubernetes transport")
		return
	}

	deleted, err := h.instanceRestarter.RestartTarget(r.Context(), target.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to restart instances")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"message": fmt.Sprintf("Restarted %d instance(s) for target %s", deleted, target.Name),
	})
}

// GetTargetTokenConfig gets the token configuration for a target
func (h *Handlers) GetTargetTokenConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	target, err := h.repo.GetTargetByID(r.Context(), targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	config := &database.TargetTokenConfig{
		TargetID:   target.ID,
		TargetName: target.Name,
		HasDefault: target.DefaultEncryptedToken != nil && *target.DefaultEncryptedToken != "",
	}

	// Get user tokens
	userTokens, _ := h.repo.GetAllUserTargetTokensForTarget(r.Context(), targetID)
	for _, ut := range userTokens {
		user, err := h.repo.GetUserByID(r.Context(), ut.UserID)
		if err == nil {
			config.UserTokens = append(config.UserTokens, database.UserTokenInfo{
				UserID:    ut.UserID,
				UserEmail: user.Email,
				UpdatedAt: ut.UpdatedAt,
			})
		}
	}

	// Get role tokens
	roleTokens, _ := h.repo.GetAllRoleTargetTokensForTarget(r.Context(), targetID)
	for _, rt := range roleTokens {
		config.RoleTokens = append(config.RoleTokens, database.RoleTokenInfo{
			Role:      rt.Role,
			UpdatedAt: rt.UpdatedAt,
		})
	}

	// Get group tokens
	groupTokens, _ := h.repo.GetAllGroupTargetTokensForTarget(r.Context(), targetID)
	for _, gt := range groupTokens {
		config.GroupTokens = append(config.GroupTokens, database.GroupTokenInfo{
			GroupName: gt.GroupName,
			UpdatedAt: gt.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, config)
}

// ==================== User Target Token Handlers ====================

// GetUserTargetToken checks if a token is set for a target (does not return the actual token)
func (h *Handlers) GetUserTargetToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	token, err := h.repo.GetUserTargetToken(r.Context(), userID, targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"has_token": false,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"has_token":  true,
		"created_at": token.CreatedAt,
		"updated_at": token.UpdatedAt,
	})
}

// SetUserTargetToken sets a token for a target
func (h *Handlers) SetUserTargetToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	var req database.SetTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "Token is required")
		return
	}

	// Verify target exists
	_, err = h.repo.GetTargetByID(r.Context(), targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	// Encrypt token
	encryptedToken, err := h.encryptor.Encrypt(req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt token")
		return
	}

	if err := h.repo.SetUserTargetToken(r.Context(), userID, targetID, encryptedToken); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to set token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Token set successfully",
	})
}

// DeleteUserTargetToken deletes a token for a target
func (h *Handlers) DeleteUserTargetToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	if err := h.repo.DeleteUserTargetToken(r.Context(), userID, targetID); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Role Target Token Handlers ====================

// SetRoleTargetToken sets a token for a role (admin only)
func (h *Handlers) SetRoleTargetToken(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	var req database.SetRoleTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Role == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "Role and token are required")
		return
	}

	// Verify target exists
	_, err = h.repo.GetTargetByID(r.Context(), targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	// Encrypt token
	encryptedToken, err := h.encryptor.Encrypt(req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt token")
		return
	}

	if err := h.repo.SetRoleTargetToken(r.Context(), req.Role, targetID, encryptedToken); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to set token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Role token set successfully",
	})
}

// DeleteRoleTargetToken deletes a role's token for a target (admin only)
func (h *Handlers) DeleteRoleTargetToken(w http.ResponseWriter, r *http.Request) {
	userRole, _ := auth.GetUserRole(r.Context())
	if userRole != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	role := chi.URLParam(r, "role")
	if role == "" {
		writeError(w, http.StatusBadRequest, "Role is required")
		return
	}

	if err := h.repo.DeleteRoleTargetToken(r.Context(), role, targetID); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Group Target Token Handlers ====================

// SetGroupTargetToken sets a token for a group (admin only)
func (h *Handlers) SetGroupTargetToken(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	var req database.SetGroupTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.GroupName == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "Group name and token are required")
		return
	}

	// Verify target exists
	_, err = h.repo.GetTargetByID(r.Context(), targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	// Encrypt token
	encryptedToken, err := h.encryptor.Encrypt(req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt token")
		return
	}

	if err := h.repo.SetGroupTargetToken(r.Context(), req.GroupName, targetID, encryptedToken); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to set token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Group token set successfully",
	})
}

// DeleteGroupTargetToken deletes a group's token for a target (admin only)
func (h *Handlers) DeleteGroupTargetToken(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	groupName := chi.URLParam(r, "group")
	if groupName == "" {
		writeError(w, http.StatusBadRequest, "Group name is required")
		return
	}

	if err := h.repo.DeleteGroupTargetToken(r.Context(), groupName, targetID); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Default Target Token Handlers ====================

// SetDefaultTargetToken sets the default token for a target (admin only)
func (h *Handlers) SetDefaultTargetToken(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	var req database.SetDefaultTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "Token is required")
		return
	}

	// Verify target exists
	_, err = h.repo.GetTargetByID(r.Context(), targetID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Target not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get target")
		return
	}

	// Encrypt token
	encryptedToken, err := h.encryptor.Encrypt(req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt token")
		return
	}

	if err := h.repo.SetTargetDefaultToken(r.Context(), targetID, encryptedToken); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to set token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Default token set successfully",
	})
}

// DeleteDefaultTargetToken deletes the default token for a target (admin only)
func (h *Handlers) DeleteDefaultTargetToken(w http.ResponseWriter, r *http.Request) {
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	if err := h.repo.DeleteTargetDefaultToken(r.Context(), targetID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Logs Handlers ====================

// ListRequestLogs lists request logs
func (h *Handlers) ListRequestLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	logs, err := h.repo.GetRequestLogs(r.Context(), &userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get logs")
		return
	}

	if logs == nil {
		logs = []*database.RequestLog{}
	}

	writeJSON(w, http.StatusOK, logs)
}

// ==================== Helper Functions ====================

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
