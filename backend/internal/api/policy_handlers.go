package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
)

// PolicyHandlers handles authorization policy operations
type PolicyHandlers struct {
	repo      *database.Repository
	encryptor *auth.TokenEncryptor
}

// NewPolicyHandlers creates new policy handlers
func NewPolicyHandlers(repo *database.Repository, encryptor *auth.TokenEncryptor) *PolicyHandlers {
	return &PolicyHandlers{
		repo:      repo,
		encryptor: encryptor,
	}
}

// ListPolicies returns all authorization policies
func (h *PolicyHandlers) ListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.repo.GetAllPolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get policies")
		return
	}

	if policies == nil {
		policies = []*database.AuthorizationPolicy{}
	}

	writeJSON(w, http.StatusOK, policies)
}

// CreatePolicy creates a new authorization policy
func (h *PolicyHandlers) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req database.CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Effect != "allow" && req.Effect != "deny" {
		writeError(w, http.StatusBadRequest, "Effect must be 'allow' or 'deny'")
		return
	}
	if req.ResourceType == "" {
		req.ResourceType = "all"
	}

	policy, err := h.repo.CreatePolicy(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create policy")
		return
	}

	writeJSON(w, http.StatusCreated, policy)
}

// GetPolicy returns a specific policy
func (h *PolicyHandlers) GetPolicy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	policy, err := h.repo.GetPolicyByID(r.Context(), id)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Policy not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get policy")
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

// UpdatePolicy updates an existing policy
func (h *PolicyHandlers) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	var req database.UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	policy, err := h.repo.UpdatePolicy(r.Context(), id, &req)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Policy not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update policy")
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

// DeletePolicy deletes a policy
func (h *PolicyHandlers) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	if err := h.repo.DeletePolicy(r.Context(), id); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Policy not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete policy")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddPolicySubject adds a subject to a policy
func (h *PolicyHandlers) AddPolicySubject(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	policyID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	var req database.CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SubjectType != "user" && req.SubjectType != "role" && req.SubjectType != "group" && req.SubjectType != "everyone" {
		writeError(w, http.StatusBadRequest, "Invalid subject type")
		return
	}

	subject, err := h.repo.AddPolicySubject(r.Context(), policyID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to add subject")
		return
	}

	writeJSON(w, http.StatusCreated, subject)
}

// DeletePolicySubject removes a subject from a policy
func (h *PolicyHandlers) DeletePolicySubject(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, _ := auth.GetUserRole(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	subjectIDStr := chi.URLParam(r, "subjectId")
	subjectID, err := uuid.Parse(subjectIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid subject ID")
		return
	}

	if err := h.repo.DeletePolicySubject(r.Context(), subjectID); err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Subject not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete subject")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
