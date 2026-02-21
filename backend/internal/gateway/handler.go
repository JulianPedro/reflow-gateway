package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/mcp"
	"github.com/reflow/gateway/internal/observability"
	"github.com/reflow/gateway/internal/telemetry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("reflow-gateway/gateway")

// Handler handles MCP Streamable HTTP requests.
//
// Streamable HTTP transport (MCP spec):
//   - POST /mcp  → JSON-RPC requests (initialize, tools/list, tools/call, etc.)
//   - GET  /mcp  → SSE stream for server-to-client notifications
//   - DELETE /mcp → Session termination
type Handler struct {
	sessionManager *SessionManager
	proxy          *Proxy
	repo           *database.Repository
	obsHub         *observability.Hub
}

// NewHandler creates a new MCP gateway handler
func NewHandler(sessionManager *SessionManager, proxy *Proxy, repo *database.Repository, obsHub *observability.Hub) *Handler {
	return &Handler{
		sessionManager: sessionManager,
		proxy:          proxy,
		repo:           repo,
		obsHub:         obsHub,
	}
}

// HandleMCP routes requests by HTTP method per the Streamable HTTP spec.
func (h *Handler) HandleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodGet:
		h.handleSSE(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePost handles all JSON-RPC requests via POST.
//
// Flow:
//  1. POST with method=initialize → creates session, connects to upstream targets, returns session ID
//  2. POST with Mcp-Session-Id header → routes request to appropriate upstream target
func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	// Auth is enforced by middleware; extract identity
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read and parse JSON-RPC request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeJSONRPCError(w, nil, mcp.ParseError, "Failed to read request body")
		return
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeJSONRPCError(w, nil, mcp.ParseError, "Invalid JSON")
		return
	}

	// Start span for handlePost
	ctx, span := tracer.Start(ctx, "handlePost",
		trace.WithAttributes(
			attribute.String("mcp.method", req.Method),
			attribute.String("mcp.user_id", userID.String()),
		),
	)
	defer span.End()

	// Inject trace_id into zerolog context
	injectTraceID(ctx)

	log.Debug().
		Str("method", req.Method).
		Str("user_id", userID.String()).
		Msg("Received MCP request")

	// Accept session ID from header (Streamable HTTP) or query param (SSE transport compat)
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}

	// --- Initialize: create or reuse session and connect to upstream targets ---
	if req.Method == mcp.MethodInitialize {
		h.handleInitialize(w, r, ctx, userID, sessionID, &req, body, startTime)
		return
	}

	// --- All other methods require an existing session ---
	if sessionID == "" {
		h.writeJSONRPCError(w, req.ID, mcp.InvalidRequest, "Session required. Send initialize first.")
		return
	}

	session, err := h.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		// Session not found or expired → client must re-initialize
		http.Error(w, "Session not found or expired", http.StatusNotFound)
		return
	}

	// Verify the session belongs to the authenticated user
	if session.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Auto-recycle if JWT claims changed (e.g., IdP updated groups/role)
	role, _ := auth.GetUserRole(ctx)
	groups, _ := auth.GetUserGroups(ctx)
	if session.NeedsRecycle(role, groups) {
		log.Info().
			Str("session_id", sessionID).
			Str("old_role", session.Role).
			Str("new_role", role).
			Msg("JWT claims changed, recycling session")
		session.Recycle(role, groups)
		// Session is now uninitialized — client must re-initialize
		h.writeJSONRPCError(w, req.ID, mcp.InvalidRequest, "Session recycled due to identity change. Please re-initialize.")
		return
	}

	// Handle notifications (no response needed)
	if req.IsNotification() {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Route request through proxy
	resp, err := h.proxy.HandleRequest(ctx, session, &req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		h.writeJSONRPCError(w, req.ID, mcp.InternalError, err.Error())
		return
	}

	w.Header().Set("Mcp-Session-Id", session.ID)
	h.writeJSON(w, resp)

	// Record metrics and emit activity
	duration := float64(time.Since(startTime).Milliseconds())
	telemetry.MCPRequestsTotal.Add(ctx, 1,
		otelmetric.WithAttributes(attribute.String("method", req.Method), attribute.String("status", "ok")),
	)
	telemetry.MCPRequestDuration.Record(ctx, duration,
		otelmetric.WithAttributes(attribute.String("method", req.Method)),
	)

	// Audit log
	h.logRequest(ctx, session.ID, &userID, req.Method, "", body, http.StatusOK, startTime)
}

// handleInitialize creates a new session (or reuses an existing one from SSE compat),
// connects to authorized upstream MCP servers, and returns the aggregated capabilities.
func (h *Handler) handleInitialize(w http.ResponseWriter, r *http.Request, ctx context.Context, userID uuid.UUID, existingSessionID string, req *mcp.JSONRPCRequest, body []byte, startTime time.Time) {
	ctx, span := tracer.Start(ctx, "handleInitialize",
		trace.WithAttributes(
			attribute.String("mcp.user_id", userID.String()),
		),
	)
	defer span.End()

	role, _ := auth.GetUserRole(ctx)
	groups, _ := auth.GetUserGroups(ctx)

	span.SetAttributes(attribute.String("mcp.role", role))

	var session *Session
	var err error

	// Reuse existing session if available (e.g., created by SSE compat GET)
	if existingSessionID != "" {
		session, err = h.sessionManager.GetSession(ctx, existingSessionID)
		if err == nil && session.UserID == userID {
			log.Debug().
				Str("session_id", existingSessionID).
				Msg("Reusing existing session for initialize")
		} else {
			session = nil // fall through to create new
		}
	}

	if session == nil {
		session, err = h.sessionManager.CreateSession(ctx, userID, role, groups)
		if err != nil {
			span.SetStatus(codes.Error, "Failed to create session")
			h.writeJSONRPCError(w, req.ID, mcp.InternalError, "Failed to create session")
			return
		}
		// Emit session created event
		if h.obsHub != nil {
			h.obsHub.EmitSession(observability.SessionEvent{
				Event:     "created",
				SessionID: session.ID,
				UserID:    userID.String(),
			})
		}
	}

	var params mcp.InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeJSONRPCError(w, req.ID, mcp.InvalidParams, "Invalid initialize params")
		return
	}

	log.Info().
		Str("session_id", session.ID).
		Str("user_id", userID.String()).
		Str("role", role).
		Strs("groups", groups).
		Str("client_name", params.ClientInfo.Name).
		Msg("Initializing MCP session")

	result, err := h.proxy.InitializeSession(ctx, session, &params)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		h.writeJSONRPCError(w, req.ID, mcp.InternalError, err.Error())
		return
	}

	// Return session ID in header so the client can use it for subsequent requests
	w.Header().Set("Mcp-Session-Id", session.ID)
	w.Header().Set("MCP-Protocol-Version", mcp.MCPProtocolVersion)
	h.writeJSONRPCResponse(w, req.ID, result)

	// Metrics
	duration := float64(time.Since(startTime).Milliseconds())
	telemetry.MCPRequestsTotal.Add(ctx, 1,
		otelmetric.WithAttributes(attribute.String("method", "initialize"), attribute.String("status", "ok")),
	)
	telemetry.MCPRequestDuration.Record(ctx, duration,
		otelmetric.WithAttributes(attribute.String("method", "initialize")),
	)
	h.emitRequestActivity(ctx, startTime, userID.String(), "initialize", "", "", "ok")

	h.logRequest(ctx, session.ID, &userID, req.Method, "", body, http.StatusOK, startTime)
}

// handleSSE handles GET /mcp.
//
// Two modes:
//  1. With Mcp-Session-Id header → SSE notification stream (Streamable HTTP spec)
//  2. Without session → backward-compatible SSE transport mode:
//     creates session and sends "endpoint" event telling the client where to POST.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "handleSSE")
	defer span.End()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.Header.Get("Mcp-Session-Id")
	var session *Session
	var err error

	if sessionID != "" {
		// Streamable HTTP: notification stream for existing session
		session, err = h.sessionManager.GetSession(ctx, sessionID)
		if err != nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		if session.UserID != userID {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	} else {
		// SSE transport compat: create a session for clients that GET first
		role, _ := auth.GetUserRole(ctx)
		groups, _ := auth.GetUserGroups(ctx)
		session, err = h.sessionManager.CreateSession(ctx, userID, role, groups)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		sessionID = session.ID
		if h.obsHub != nil {
			h.obsHub.EmitSession(observability.SessionEvent{
				Event:     "created",
				SessionID: session.ID,
				UserID:    userID.String(),
			})
		}
	}

	w.Header().Set("Mcp-Session-Id", session.ID)

	sseWriter, err := mcp.NewSSEWriter(w)
	if err != nil {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// If session was just created (no prior session ID), send the endpoint event
	// so SSE-transport clients know where to POST their JSON-RPC messages.
	if r.Header.Get("Mcp-Session-Id") == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		endpointURL := scheme + "://" + r.Host + "/mcp?session_id=" + session.ID

		if err := sseWriter.WriteEndpoint(endpointURL); err != nil {
			log.Error().Err(err).Msg("Failed to send endpoint event")
			return
		}

		log.Info().
			Str("session_id", sessionID).
			Str("endpoint", endpointURL).
			Msg("SSE transport: sent endpoint event")
	} else {
		log.Info().Str("session_id", sessionID).Msg("SSE notification stream opened")
	}

	// Keep alive until client disconnects
	<-ctx.Done()

	sseWriter.Close()
	log.Info().Str("session_id", sessionID).Msg("SSE stream closed")
}

// handleDelete terminates a session per the Streamable HTTP spec.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "handleDelete")
	defer span.End()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}

	session, err := h.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if session.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.sessionManager.DeleteSession(ctx, sessionID); err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	if h.obsHub != nil {
		h.obsHub.EmitSession(observability.SessionEvent{
			Event:     "deleted",
			SessionID: sessionID,
			UserID:    userID.String(),
		})
	}

	w.WriteHeader(http.StatusNoContent)
	log.Info().Str("session_id", sessionID).Msg("Session terminated by client")
}

// --- helpers ---

func (h *Handler) emitRequestActivity(ctx context.Context, startTime time.Time, userID, method, target, tool, status string) {
	if h.obsHub == nil {
		return
	}
	traceID := ""
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		traceID = span.SpanContext().TraceID().String()
	}
	h.obsHub.EmitActivity(observability.ActivityEvent{
		Timestamp:  startTime,
		UserID:     userID,
		Method:     method,
		Target:     target,
		Tool:       tool,
		DurationMS: float64(time.Since(startTime).Milliseconds()),
		Status:     status,
		TraceID:    traceID,
	})
}

func (h *Handler) writeJSONRPCResponse(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	resp, err := mcp.NewSuccessResponse(id, result)
	if err != nil {
		h.writeJSONRPCError(w, id, mcp.InternalError, "Failed to marshal response")
		return
	}
	h.writeJSON(w, resp)
}

func (h *Handler) writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	resp := mcp.NewErrorResponse(id, code, message)
	h.writeJSON(w, resp)
}

func (h *Handler) writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("Failed to write JSON response")
	}
}

func (h *Handler) logRequest(ctx context.Context, sessionID string, userID *uuid.UUID, method, targetName string, body []byte, status int, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	reqLog := &database.RequestLog{
		SessionID:      sessionID,
		UserID:         userID,
		Method:         method,
		TargetName:     targetName,
		RequestBody:    body,
		ResponseStatus: status,
		DurationMS:     int(duration),
	}

	if err := h.repo.CreateRequestLog(ctx, reqLog); err != nil {
		// Don't fail the request if logging fails
		_ = err
	}
}

// injectTraceID adds the OTel trace_id to the zerolog context for log correlation.
func injectTraceID(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		log.Ctx(ctx).UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("trace_id", span.SpanContext().TraceID().String())
		})
	}
}
