package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/reflow/gateway/internal/telemetry"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var mcpTracer = otel.Tracer("reflow-gateway/mcp-client")

// TransportType represents the MCP transport type
type TransportType string

const (
	TransportStreamableHTTP TransportType = "streamable-http"
	TransportSSE            TransportType = "sse"
)

// Client represents an MCP client that connects to upstream servers.
// Supports both Streamable HTTP and legacy SSE transports, with auto-detection.
type Client struct {
	httpClient    *http.Client
	url           string
	authToken     string
	authHeader    string
	customHeaders map[string]string
	transportType TransportType

	mu           sync.RWMutex
	sessionID    string
	initialized  bool
	capabilities *ServerCapabilities
	serverInfo   *ServerInfo

	// SSE transport state
	messageEndpoint string
	sseResponses    map[string]chan *JSONRPCResponse
	sseCancel       context.CancelFunc
	sseDone         chan struct{} // closed when SSE reader exits

	// Request ID counter
	nextID int64
	idMu   sync.Mutex
}

// ClientConfig holds configuration for creating an MCP client
type ClientConfig struct {
	URL           string
	AuthToken     string
	AuthHeader    string
	Timeout       time.Duration
	CustomHeaders map[string]string
	TransportType TransportType
}

// NewClient creates a new MCP client
func NewClient(cfg ClientConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	customHeaders := cfg.CustomHeaders
	if customHeaders == nil {
		customHeaders = make(map[string]string)
	}

	transportType := cfg.TransportType
	if transportType == "" {
		transportType = TransportStreamableHTTP
	}

	return &Client{
		httpClient:    &http.Client{Timeout: timeout},
		url:           cfg.URL,
		authToken:     cfg.AuthToken,
		authHeader:    cfg.AuthHeader,
		customHeaders: customHeaders,
		transportType: transportType,
		sseResponses:  make(map[string]chan *JSONRPCResponse),
		sseDone:       make(chan struct{}),
	}
}

// allocID returns a unique JSON-RPC request ID
func (c *Client) allocID() json.RawMessage {
	c.idMu.Lock()
	c.nextID++
	id := c.nextID
	c.idMu.Unlock()
	return json.RawMessage(fmt.Sprintf("%d", id))
}

// Initialize sends an initialize request to the upstream MCP server.
// It auto-detects the transport if Streamable HTTP fails by falling back to SSE.
func (c *Client) Initialize(ctx context.Context, params *InitializeParams) (*InitializeResult, error) {
	ctx, span := mcpTracer.Start(ctx, "mcp.upstream.Initialize",
		trace.WithAttributes(attribute.String("url", c.url)),
	)
	defer span.End()

	// If explicitly configured as SSE, use it directly
	if c.transportType == TransportSSE {
		if err := c.connectSSE(ctx); err != nil {
			return nil, fmt.Errorf("SSE connect failed: %w", err)
		}
		return c.doInitialize(ctx, params)
	}

	// Try Streamable HTTP first
	result, err := c.doInitialize(ctx, params)
	if err == nil {
		log.Info().Str("url", c.url).Msg("Upstream connected via Streamable HTTP")
		return result, nil
	}

	span.AddEvent("transport_fallback", trace.WithAttributes(attribute.String("reason", err.Error())))
	log.Warn().Err(err).Str("url", c.url).Msg("Streamable HTTP failed, trying SSE fallback")

	// Fall back to SSE: try the configured URL first, then {url}/sse
	c.mu.Lock()
	c.transportType = TransportSSE
	origURL := c.url
	c.mu.Unlock()

	if sseErr := c.connectSSE(ctx); sseErr != nil {
		// Try {url}/sse
		sseURL := strings.TrimRight(origURL, "/") + "/sse"
		c.mu.Lock()
		c.url = sseURL
		c.mu.Unlock()

		if sseErr2 := c.connectSSE(ctx); sseErr2 != nil {
			// Restore original state
			c.mu.Lock()
			c.url = origURL
			c.transportType = TransportStreamableHTTP
			c.mu.Unlock()
			return nil, fmt.Errorf("all transports failed - HTTP: %v; SSE(%s): %v; SSE(%s): %v",
				err, origURL, sseErr, sseURL, sseErr2)
		}
	}

	result, err = c.doInitialize(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("SSE initialize failed: %w", err)
	}

	c.mu.RLock()
	log.Info().Str("url", c.url).Str("endpoint", c.messageEndpoint).Msg("Upstream connected via SSE")
	c.mu.RUnlock()

	return result, nil
}

// doInitialize performs the actual initialize handshake
func (c *Client) doInitialize(ctx context.Context, params *InitializeParams) (*InitializeResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodInitialize,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.capabilities = &result.Capabilities
	c.serverInfo = &result.ServerInfo
	c.mu.Unlock()

	// Send initialized notification
	notification := &JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  MethodInitialized,
	}
	if err := c.sendNotification(ctx, notification); err != nil {
		log.Warn().Err(err).Msg("Failed to send initialized notification")
	}

	return &result, nil
}

// connectSSE connects to the SSE endpoint, retrieves the message endpoint,
// and starts a background goroutine to read events.
func (c *Client) connectSSE(ctx context.Context) error {
	// Use a long-lived context for the SSE connection (cancelled on Close)
	sseCtx, cancel := context.WithCancel(context.Background())

	httpReq, err := http.NewRequestWithContext(sseCtx, http.MethodGet, c.url, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	c.applyAuthHeaders(httpReq)
	c.applyCustomHeaders(httpReq)

	// Use a client without timeout for the long-lived SSE connection
	sseHTTPClient := &http.Client{}
	resp, err := sseHTTPClient.Do(httpReq)
	if err != nil {
		cancel()
		return fmt.Errorf("SSE connect failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("SSE connect status: %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("unexpected Content-Type: %s (expected text/event-stream)", ct)
	}

	reader := NewSSEReader(resp.Body)

	// Read events until we get the endpoint (with timeout)
	type endpointResult struct {
		endpoint string
		err      error
	}
	ch := make(chan endpointResult, 1)
	go func() {
		for {
			event, err := reader.ReadEvent()
			if err != nil {
				ch <- endpointResult{err: fmt.Errorf("SSE read error: %w", err)}
				return
			}

			log.Debug().
				Str("event", event.Event).
				Str("data", event.Data).
				Msg("SSE event from upstream")

			if event.Event == "endpoint" {
				ch <- endpointResult{endpoint: event.Data}
				return
			}
		}
	}()

	select {
	case result := <-ch:
		if result.err != nil {
			resp.Body.Close()
			cancel()
			return result.err
		}

		endpoint := c.resolveEndpoint(result.endpoint)

		c.mu.Lock()
		c.messageEndpoint = endpoint
		c.sseCancel = cancel
		c.sseDone = make(chan struct{})
		c.mu.Unlock()

		log.Info().Str("endpoint", endpoint).Msg("Got SSE message endpoint")

		// Start background reader for SSE responses/notifications
		go c.readSSEEvents(resp.Body, reader)
		return nil

	case <-time.After(15 * time.Second):
		resp.Body.Close()
		cancel()
		return fmt.Errorf("timeout waiting for endpoint event")

	case <-ctx.Done():
		resp.Body.Close()
		cancel()
		return ctx.Err()
	}
}

// readSSEEvents reads SSE events in the background and routes JSON-RPC responses
// to the appropriate waiting request channel.
func (c *Client) readSSEEvents(body io.ReadCloser, reader *SSEReader) {
	defer body.Close()
	defer func() {
		c.mu.Lock()
		close(c.sseDone)
		c.mu.Unlock()
	}()

	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err != io.EOF {
				log.Debug().Err(err).Msg("SSE event reader ended")
			}
			return
		}

		if event.Event == "message" && event.Data != "" {
			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(event.Data), &resp); err != nil {
				log.Debug().Err(err).Str("data", event.Data).Msg("Failed to parse SSE message")
				continue
			}

			if resp.ID != nil {
				reqID := string(resp.ID)
				c.mu.Lock()
				if ch, ok := c.sseResponses[reqID]; ok {
					ch <- &resp
					delete(c.sseResponses, reqID)
				}
				c.mu.Unlock()
			}
		}
	}
}

// resolveEndpoint resolves a potentially relative endpoint URL to an absolute URL
func (c *Client) resolveEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}

	base, err := url.Parse(c.url)
	if err != nil {
		return strings.TrimRight(c.url, "/") + "/" + strings.TrimLeft(endpoint, "/")
	}

	ref, err := url.Parse(endpoint)
	if err != nil {
		return strings.TrimRight(c.url, "/") + "/" + strings.TrimLeft(endpoint, "/")
	}

	return base.ResolveReference(ref).String()
}

// sendRequest sends a JSON-RPC request and returns the response.
// For Streamable HTTP: POST and read JSON response.
// For SSE: POST and read response from either POST body (200) or SSE stream (202).
func (c *Client) sendRequest(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	ctx, span := mcpTracer.Start(ctx, "mcp.upstream.sendRequest",
		trace.WithAttributes(attribute.String("mcp.method", req.Method)),
	)
	defer span.End()
	reqStart := time.Now()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.mu.RLock()
	targetURL := c.url
	if c.transportType == TransportSSE && c.messageEndpoint != "" {
		targetURL = c.messageEndpoint
	}
	isSSE := c.transportType == TransportSSE
	c.mu.RUnlock()

	span.SetAttributes(attribute.String("url", targetURL))

	// For SSE transport, register a response channel before sending
	var responseCh chan *JSONRPCResponse
	if isSSE {
		reqID := string(req.ID)
		responseCh = make(chan *JSONRPCResponse, 1)
		c.mu.Lock()
		c.sseResponses[reqID] = responseCh
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			delete(c.sseResponses, string(req.ID))
			c.mu.Unlock()
		}()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", MCPProtocolVersion)

	c.mu.RLock()
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.RUnlock()

	c.applyAuthHeaders(httpReq)
	c.applyCustomHeaders(httpReq)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		c.recordUpstreamMetrics(ctx, req.Method, "error", reqStart)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", httpResp.StatusCode))

	// Capture upstream session ID
	if sid := httpResp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}

	switch httpResp.StatusCode {
	case http.StatusOK:
		respBody, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		ct := httpResp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/event-stream") {
			c.recordUpstreamMetrics(ctx, req.Method, "ok", reqStart)
			return c.parseSSEResponseBody(respBody)
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response (status 200, body: %.200s): %w", string(respBody), err)
		}
		c.recordUpstreamMetrics(ctx, req.Method, "ok", reqStart)
		return &resp, nil

	case http.StatusAccepted:
		// SSE transport: response comes on the SSE stream
		if responseCh != nil {
			select {
			case resp := <-responseCh:
				c.recordUpstreamMetrics(ctx, req.Method, "ok", reqStart)
				return resp, nil
			case <-c.sseDone:
				return nil, fmt.Errorf("SSE connection closed while waiting for response")
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(60 * time.Second):
				return nil, fmt.Errorf("timeout waiting for SSE response")
			}
		}
		// For non-SSE, 202 means accepted (e.g., notifications)
		return nil, fmt.Errorf("received 202 Accepted but no SSE channel")

	default:
		respBody, _ := io.ReadAll(httpResp.Body)
		span.SetStatus(codes.Error, fmt.Sprintf("upstream status %d", httpResp.StatusCode))
		c.recordUpstreamMetrics(ctx, req.Method, "error", reqStart)
		return nil, fmt.Errorf("upstream status %d: %s", httpResp.StatusCode, string(respBody))
	}
}

func (c *Client) recordUpstreamMetrics(ctx context.Context, method, status string, start time.Time) {
	duration := float64(time.Since(start).Milliseconds())
	telemetry.MCPUpstreamRequestsTotal.Add(ctx, 1,
		otelmetric.WithAttributes(
			attribute.String("method", method),
			attribute.String("status", status),
		),
	)
	telemetry.MCPUpstreamRequestDuration.Record(ctx, duration,
		otelmetric.WithAttributes(attribute.String("method", method)),
	)
}

// parseSSEResponseBody extracts a JSON-RPC response from an SSE-formatted response body
func (c *Client) parseSSEResponseBody(body []byte) (*JSONRPCResponse, error) {
	reader := NewSSEReader(bytes.NewReader(body))
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSE response: %w", err)
		}
		if event.Data != "" {
			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(event.Data), &resp); err != nil {
				continue // try next event
			}
			return &resp, nil
		}
	}
}

func (c *Client) sendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	c.mu.RLock()
	targetURL := c.url
	if c.transportType == TransportSSE && c.messageEndpoint != "" {
		targetURL = c.messageEndpoint
	}
	c.mu.RUnlock()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("MCP-Protocol-Version", MCPProtocolVersion)

	c.mu.RLock()
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.RUnlock()

	c.applyAuthHeaders(httpReq)
	c.applyCustomHeaders(httpReq)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("notification failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK &&
		httpResp.StatusCode != http.StatusAccepted &&
		httpResp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("notification status %d: %s", httpResp.StatusCode, string(respBody))
	}

	return nil
}

// --- Public API methods ---

// ListTools retrieves the list of tools from the upstream server
func (c *Client) ListTools(ctx context.Context, cursor *string) (*ToolsListResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodToolsList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}
	return &result, nil
}

// CallTool calls a tool on the upstream server
func (c *Client) CallTool(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodToolsCall,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call result: %w", err)
	}
	return &result, nil
}

// ListResources retrieves the list of resources from the upstream server
func (c *Client) ListResources(ctx context.Context, cursor *string) (*ResourcesListResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodResourcesList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ResourcesListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources/list result: %w", err)
	}
	return &result, nil
}

// ReadResource reads a resource from the upstream server
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceReadResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodResourcesRead,
	}
	params := ResourceReadParams{URI: uri}
	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("resources/read error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ResourceReadResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources/read result: %w", err)
	}
	return &result, nil
}

// ListPrompts retrieves the list of prompts from the upstream server
func (c *Client) ListPrompts(ctx context.Context, cursor *string) (*PromptsListResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodPromptsList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result PromptsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts/list result: %w", err)
	}
	return &result, nil
}

// GetPrompt retrieves a prompt from the upstream server
func (c *Client) GetPrompt(ctx context.Context, params *PromptGetParams) (*PromptGetResult, error) {
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.allocID(),
		Method:  MethodPromptsGet,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/get error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result PromptGetResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts/get result: %w", err)
	}
	return &result, nil
}

// SendRawRequest sends a raw JSON-RPC request to the upstream server
func (c *Client) SendRawRequest(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	return c.sendRequest(ctx, req)
}

// --- Accessors ---

func (c *Client) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = sessionID
}

func (c *Client) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

func (c *Client) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

func (c *Client) GetCapabilities() *ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities
}

func (c *Client) GetServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// Close closes the client and releases resources
func (c *Client) Close() error {
	c.mu.Lock()
	if c.sseCancel != nil {
		c.sseCancel()
	}
	c.mu.Unlock()
	c.httpClient.CloseIdleConnections()
	return nil
}

// --- Internal helpers ---

func (c *Client) applyAuthHeaders(req *http.Request) {
	if c.authToken != "" {
		headerName := c.authHeader
		if headerName == "" {
			headerName = "Authorization"
		}
		if headerName == "Authorization" {
			req.Header.Set(headerName, "Bearer "+c.authToken)
		} else {
			req.Header.Set(headerName, c.authToken)
		}
	}
}

func (c *Client) applyCustomHeaders(req *http.Request) {
	for key, value := range c.customHeaders {
		req.Header.Set(key, value)
	}
}
