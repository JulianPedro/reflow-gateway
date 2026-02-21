package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
	"github.com/rs/zerolog/log"
)

// EnvHandlers handles environment configuration operations
type EnvHandlers struct {
	repo              *database.Repository
	encryptor         *auth.TokenEncryptor
	instanceRestarter InstanceRestarter
}

// NewEnvHandlers creates new env handlers
func NewEnvHandlers(repo *database.Repository, encryptor *auth.TokenEncryptor, instanceRestarter InstanceRestarter) *EnvHandlers {
	return &EnvHandlers{
		repo:              repo,
		encryptor:         encryptor,
		instanceRestarter: instanceRestarter,
	}
}

// ListEnvConfigs returns all env configs for a target
func (h *EnvHandlers) ListEnvConfigs(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	configs, err := h.repo.GetAllEnvConfigsForTarget(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get env configs")
		return
	}

	if configs == nil {
		configs = []*database.TargetEnvConfig{}
	}

	writeJSON(w, http.StatusOK, configs)
}

// extractScopeFromPath extracts the scope type from the request path
// Expected paths: /targets/{id}/env/default, /targets/{id}/env/role/{value}, etc.
func extractScopeFromPath(r *http.Request) string {
	path := r.URL.Path
	parts := strings.Split(path, "/")

	// Find "env" in the path and get the next part
	for i, part := range parts {
		if part == "env" && i+1 < len(parts) {
			scope := parts[i+1]
			// Valid scope types
			if scope == "default" || scope == "role" || scope == "group" || scope == "user" {
				return scope
			}
		}
	}
	return ""
}

// GetEnvConfigsByScope returns env configs for a specific scope
func (h *EnvHandlers) GetEnvConfigsByScope(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	scopeType := extractScopeFromPath(r)
	if scopeType == "" {
		writeError(w, http.StatusBadRequest, "Invalid scope type")
		return
	}

	scopeValue := chi.URLParam(r, "scopeValue")
	var scopeValuePtr *string
	if scopeValue != "" && scopeType != "default" {
		scopeValuePtr = &scopeValue
	}

	configs, err := h.repo.GetEnvConfigs(r.Context(), targetID, scopeType, scopeValuePtr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get env configs")
		return
	}

	if configs == nil {
		configs = []*database.TargetEnvConfig{}
	}

	writeJSON(w, http.StatusOK, configs)
}

// SetEnvConfig creates or updates an env config
func (h *EnvHandlers) SetEnvConfig(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	scopeType := extractScopeFromPath(r)
	if scopeType == "" {
		writeError(w, http.StatusBadRequest, "Invalid scope type")
		return
	}

	scopeValue := chi.URLParam(r, "scopeValue")

	var req database.SetEnvConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if req.Value == "" {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	// Determine scope value pointer
	var scopeValuePtr *string
	if scopeValue != "" && scopeType != "default" {
		scopeValuePtr = &scopeValue
	}

	// Encrypt the value
	encryptedValue, err := h.encryptor.Encrypt(req.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt value")
		return
	}

	err = h.repo.SetEnvConfig(r.Context(), targetID, scopeType, scopeValuePtr, req.Key, encryptedValue, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to set env config")
		return
	}

	// Fetch the updated config to return
	configs, err := h.repo.GetEnvConfigs(r.Context(), targetID, scopeType, scopeValuePtr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get updated config")
		return
	}

	// Auto-restart K8s instances so pods pick up new env
	h.restartK8sInstances(r.Context(), targetID)

	// Find the config we just set
	for _, c := range configs {
		if c.EnvKey == req.Key {
			writeJSON(w, http.StatusOK, c)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteEnvConfig deletes an env config
func (h *EnvHandlers) DeleteEnvConfig(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	scopeType := extractScopeFromPath(r)
	if scopeType == "" {
		writeError(w, http.StatusBadRequest, "Invalid scope type")
		return
	}

	scopeValue := chi.URLParam(r, "scopeValue")
	envKey := chi.URLParam(r, "key")

	var scopeValuePtr *string
	if scopeValue != "" && scopeType != "default" {
		scopeValuePtr = &scopeValue
	}

	if err := h.repo.DeleteEnvConfig(r.Context(), targetID, scopeType, scopeValuePtr, envKey); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Env config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete env config")
		return
	}

	// Auto-restart K8s instances so pods pick up removed env
	h.restartK8sInstances(r.Context(), targetID)

	w.WriteHeader(http.StatusNoContent)
}

// BulkSetEnvConfigs sets multiple env configs at once
func (h *EnvHandlers) BulkSetEnvConfigs(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	scopeType := extractScopeFromPath(r)
	if scopeType == "" {
		writeError(w, http.StatusBadRequest, "Invalid scope type")
		return
	}

	scopeValue := chi.URLParam(r, "scopeValue")

	var req struct {
		Configs map[string]string `json:"configs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Configs) == 0 {
		writeError(w, http.StatusBadRequest, "configs is required")
		return
	}

	var scopeValuePtr *string
	if scopeValue != "" && scopeType != "default" {
		scopeValuePtr = &scopeValue
	}

	for key, value := range req.Configs {
		encryptedValue, err := h.encryptor.Encrypt(value)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to encrypt value")
			return
		}

		err = h.repo.SetEnvConfig(r.Context(), targetID, scopeType, scopeValuePtr, key, encryptedValue, nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to set env config")
			return
		}
	}

	// Auto-restart K8s instances so pods pick up new env
	h.restartK8sInstances(r.Context(), targetID)

	// Return the updated configs
	configs, err := h.repo.GetEnvConfigs(r.Context(), targetID, scopeType, scopeValuePtr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get updated configs")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}

// ResolveEnvConfigs returns the resolved env configs for a user
func (h *EnvHandlers) ResolveEnvConfigs(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid target ID")
		return
	}

	// Get user info from context
	userID, _ := auth.GetUserID(r.Context())
	role, _ := auth.GetUserRole(r.Context())
	groups, _ := auth.GetUserGroups(r.Context())

	configs, err := h.repo.ResolveEnvConfigsForTarget(r.Context(), targetID, userID, role, groups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to resolve env configs")
		return
	}

	// Convert to response format (without values for security)
	type envConfigResponse struct {
		Key        string `json:"key"`
		ScopeType  string `json:"scope_type"`
		ScopeValue string `json:"scope_value,omitempty"`
	}

	response := make([]envConfigResponse, 0, len(configs))
	for key, info := range configs {
		response = append(response, envConfigResponse{
			Key:        key,
			ScopeType:  info.Source,
			ScopeValue: info.ScopeValue,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

// restartK8sInstances restarts K8s instances for a target if it's a kubernetes transport.
// Called automatically after env config mutations so pods pick up new config.
func (h *EnvHandlers) restartK8sInstances(ctx context.Context, targetID uuid.UUID) {
	if h.instanceRestarter == nil {
		return
	}

	target, err := h.repo.GetTargetByID(ctx, targetID)
	if err != nil || target.TransportType != "kubernetes" {
		return
	}

	deleted, err := h.instanceRestarter.RestartTarget(ctx, target.Name)
	if err != nil {
		log.Warn().Err(err).Str("target", target.Name).Msg("Failed to auto-restart K8s instances after env change")
		return
	}
	if deleted > 0 {
		log.Info().Str("target", target.Name).Int("deleted", deleted).Msg("Auto-restarted K8s instances after env change")
	}
}
