package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/k8s"
	"github.com/reflow/gateway/internal/mcp"
	"github.com/reflow/gateway/internal/observability"
	"github.com/reflow/gateway/internal/stdio"
	"github.com/reflow/gateway/internal/telemetry"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// toolDelimiter separates target name from tool name (matches AgentGateway)
const toolDelimiter = "_"

// Proxy handles proxying requests to upstream MCP servers
type Proxy struct {
	repo         *database.Repository
	encryptor    *auth.TokenEncryptor
	authorizer   *Authorizer
	stdioManager *stdio.Manager
	k8sManager   *k8s.Manager
	obsHub       *observability.Hub
}

// NewProxy creates a new proxy
func NewProxy(repo *database.Repository, encryptor *auth.TokenEncryptor, authorizer *Authorizer, stdioManager *stdio.Manager, k8sManager *k8s.Manager, obsHub *observability.Hub) *Proxy {
	return &Proxy{
		repo:         repo,
		encryptor:    encryptor,
		authorizer:   authorizer,
		stdioManager: stdioManager,
		k8sManager:   k8sManager,
		obsHub:       obsHub,
	}
}

// InitializeSession initializes all targets for a session
func (p *Proxy) InitializeSession(ctx context.Context, session *Session, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	ctx, span := tracer.Start(ctx, "Proxy.InitializeSession")
	defer span.End()

	targets, err := p.repo.GetEnabledTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	if len(targets) == 0 {
		return &mcp.InitializeResult{
			ProtocolVersion: mcp.MCPProtocolVersion,
			Capabilities:    mcp.ServerCapabilities{},
			ServerInfo: mcp.ServerInfo{
				Name:    "reflow-gateway",
				Version: "1.0.0",
			},
			Instructions: "No MCP servers configured. Add targets via the API.",
		}, nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var authorizedTargets int

	aggregatedCaps := &mcp.ServerCapabilities{}

	span.SetAttributes(attribute.Int("target_count", len(targets)))

	for _, target := range targets {
		wg.Add(1)
		go func(target *database.Target) {
			defer wg.Done()

			_, targetSpan := tracer.Start(ctx, "InitializeTarget",
				trace.WithAttributes(
					attribute.String("target.name", target.Name),
					attribute.String("target.transport", target.TransportType),
				),
			)
			defer targetSpan.End()

			// Check authorization for this target
			if p.authorizer != nil {
				canAccess, policyName, err := p.authorizer.CanAccessTarget(ctx, session.UserID, session.Role, session.Groups, target.ID)
				if err != nil {
					log.Error().Err(err).Str("target", target.Name).Msg("Authorization check failed")
					mu.Lock()
					errors = append(errors, fmt.Errorf("target %s: authorization check failed: %w", target.Name, err))
					mu.Unlock()
					return
				}
				if !canAccess {
					log.Debug().
						Str("target", target.Name).
						Str("user_id", session.UserID.String()).
						Msg("User not authorized for target")
					return
				}
				log.Debug().
					Str("target", target.Name).
					Str("policy", policyName).
					Msg("User authorized for target")
			}

			var client mcp.MCPClient
			var clientErr error

			switch target.TransportType {
			case "stdio":
				client, clientErr = p.createStdioClient(ctx, session, target)
			case "kubernetes":
				client, clientErr = p.createK8sClient(ctx, session, target)
			default:
				client, clientErr = p.createHTTPClient(ctx, session, target)
			}
			if clientErr != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("target %s: %w", target.Name, clientErr))
				mu.Unlock()
				log.Error().Err(clientErr).Str("target", target.Name).Msg("Failed to create client")
				return
			}

			result, err := client.Initialize(ctx, params)
			if err != nil {
				client.Close()
				mu.Lock()
				errors = append(errors, fmt.Errorf("target %s: %w", target.Name, err))
				mu.Unlock()
				log.Error().Err(err).Str("target", target.Name).Msg("Failed to initialize upstream")
				return
			}

			session.SetClient(target.Name, client)
			session.SetTargetID(target.Name, target.ID)

			mu.Lock()
			authorizedTargets++
			// Aggregate capabilities
			if result.Capabilities.Tools != nil {
				aggregatedCaps.Tools = &mcp.ToolsCapability{
					ListChanged: aggregatedCaps.Tools != nil && aggregatedCaps.Tools.ListChanged || result.Capabilities.Tools.ListChanged,
				}
			}
			if result.Capabilities.Resources != nil {
				aggregatedCaps.Resources = &mcp.ResourcesCapability{
					Subscribe:   aggregatedCaps.Resources != nil && aggregatedCaps.Resources.Subscribe || result.Capabilities.Resources.Subscribe,
					ListChanged: aggregatedCaps.Resources != nil && aggregatedCaps.Resources.ListChanged || result.Capabilities.Resources.ListChanged,
				}
			}
			if result.Capabilities.Prompts != nil {
				aggregatedCaps.Prompts = &mcp.PromptsCapability{
					ListChanged: aggregatedCaps.Prompts != nil && aggregatedCaps.Prompts.ListChanged || result.Capabilities.Prompts.ListChanged,
				}
			}
			if result.Capabilities.Logging != nil {
				aggregatedCaps.Logging = &mcp.LoggingCapability{}
			}
			mu.Unlock()

			log.Info().
				Str("target", target.Name).
				Str("server_name", result.ServerInfo.Name).
				Str("server_version", result.ServerInfo.Version).
				Msg("Initialized upstream target")
		}(target)
	}

	wg.Wait()

	if authorizedTargets == 0 {
		if len(errors) > 0 {
			return nil, fmt.Errorf("all targets failed to initialize: %v", errors)
		}
		return &mcp.InitializeResult{
			ProtocolVersion: mcp.MCPProtocolVersion,
			Capabilities:    mcp.ServerCapabilities{},
			ServerInfo: mcp.ServerInfo{
				Name:    "reflow-gateway",
				Version: "1.0.0",
			},
			Instructions: "No authorized MCP servers available for your account.",
		}, nil
	}

	session.SetInitialized(aggregatedCaps)

	return &mcp.InitializeResult{
		ProtocolVersion: mcp.MCPProtocolVersion,
		Capabilities:    *aggregatedCaps,
		ServerInfo: mcp.ServerInfo{
			Name:    "reflow-gateway",
			Version: "1.0.0",
		},
	}, nil
}

// ListTools aggregates tools from all connected upstream targets.
// Tools are prefixed with target name when there are multiple targets.
func (p *Proxy) ListTools(ctx context.Context, session *Session) (*mcp.ToolsListResult, error) {
	clients := session.GetAllClients()
	if len(clients) == 0 {
		return &mcp.ToolsListResult{Tools: []mcp.Tool{}}, nil
	}

	multiplexing := len(clients) > 1

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allTools []mcp.Tool

	session.ClearToolMappings()

	for targetName, client := range clients {
		wg.Add(1)
		go func(name string, c mcp.MCPClient) {
			defer wg.Done()

			result, err := c.ListTools(ctx, nil)
			if err != nil {
				log.Error().Err(err).Str("target", name).Msg("Failed to list tools")
				return
			}

			targetID, _ := session.GetTargetID(name)

			mu.Lock()
			for _, tool := range result.Tools {
				// Check tool-level authorization
				if p.authorizer != nil {
					canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &targetID, "tool", tool.Name)
					if err != nil || !canAccess {
						continue
					}
				}

				// Prefix tool name with target name when multiplexing
				displayName := tool.Name
				if multiplexing {
					displayName = name + toolDelimiter + tool.Name
				}

				prefixedTool := mcp.Tool{
					Name:        displayName,
					Description: tool.Description,
					InputSchema: tool.InputSchema,
				}
				allTools = append(allTools, prefixedTool)

				session.SetToolMapping(displayName, ToolMapping{
					TargetID:   targetID,
					TargetName: name,
					ToolName:   tool.Name,
				})
			}
			mu.Unlock()
		}(targetName, client)
	}

	wg.Wait()

	return &mcp.ToolsListResult{Tools: allTools}, nil
}

// CallTool routes a tool call to the appropriate upstream target
func (p *Proxy) CallTool(ctx context.Context, session *Session, params *mcp.ToolCallParams) (*mcp.ToolCallResult, error) {
	ctx, span := tracer.Start(ctx, "Proxy.CallTool",
		trace.WithAttributes(attribute.String("tool.name", params.Name)),
	)
	defer span.End()
	callStart := time.Now()

	// Look up tool mapping
	mapping, exists := session.GetToolMapping(params.Name)

	if !exists {
		// Try to parse the tool name as target_toolname
		clients := session.GetAllClients()
		if len(clients) > 1 {
			// Multiplexing: parse prefix
			targetName, toolName := parseResourceName(params.Name)
			if targetName != "" {
				targetID, _ := session.GetTargetID(targetName)
				mapping = ToolMapping{
					TargetID:   targetID,
					TargetName: targetName,
					ToolName:   toolName,
				}
				exists = true
			}
		} else {
			// Single target: use the tool name as-is
			for name, _ := range clients {
				targetID, _ := session.GetTargetID(name)
				mapping = ToolMapping{
					TargetID:   targetID,
					TargetName: name,
					ToolName:   params.Name,
				}
				exists = true
				break
			}
		}
	}

	if !exists {
		return mcp.NewToolCallError(fmt.Sprintf("Tool not found: %s", params.Name)), nil
	}

	// Re-check authorization
	if p.authorizer != nil {
		canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &mapping.TargetID, "tool", mapping.ToolName)
		if err != nil || !canAccess {
			return mcp.NewToolCallError(fmt.Sprintf("Not authorized to call tool: %s", params.Name)), nil
		}
	}

	client := session.GetClient(mapping.TargetName)
	if client == nil {
		return mcp.NewToolCallError(fmt.Sprintf("Target not connected: %s", mapping.TargetName)), nil
	}

	span.SetAttributes(attribute.String("tool.target", mapping.TargetName))

	// Use original tool name (without prefix), always pass arguments (even if empty)
	args := params.Arguments
	if args == nil {
		args = make(map[string]interface{})
	}
	originalParams := &mcp.ToolCallParams{
		Name:      mapping.ToolName,
		Arguments: args,
	}

	result, err := client.CallTool(ctx, originalParams)

	// Record tool call metrics
	callDuration := float64(time.Since(callStart).Milliseconds())
	telemetry.MCPToolCallsTotal.Add(ctx, 1,
		otelmetric.WithAttributes(
			attribute.String("tool_name", mapping.ToolName),
			attribute.String("target", mapping.TargetName),
		),
	)
	telemetry.MCPToolCallDuration.Record(ctx, callDuration,
		otelmetric.WithAttributes(
			attribute.String("tool_name", mapping.ToolName),
			attribute.String("target", mapping.TargetName),
		),
	)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		p.emitActivity(ctx, callStart, session, "tools/call", mapping.TargetName, mapping.ToolName, "error")
	} else {
		p.emitActivity(ctx, callStart, session, "tools/call", mapping.TargetName, mapping.ToolName, "ok")
	}

	return result, err
}

// ListResources aggregates resources from all connected upstream targets
func (p *Proxy) ListResources(ctx context.Context, session *Session) (*mcp.ResourcesListResult, error) {
	clients := session.GetAllClients()
	if len(clients) == 0 {
		return &mcp.ResourcesListResult{Resources: []mcp.Resource{}}, nil
	}

	multiplexing := len(clients) > 1

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResources []mcp.Resource

	session.ClearResourceMappings()

	for targetName, client := range clients {
		wg.Add(1)
		go func(name string, c mcp.MCPClient) {
			defer wg.Done()

			result, err := c.ListResources(ctx, nil)
			if err != nil {
				log.Error().Err(err).Str("target", name).Msg("Failed to list resources")
				return
			}

			targetID, _ := session.GetTargetID(name)

			mu.Lock()
			for _, resource := range result.Resources {
				if p.authorizer != nil {
					canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &targetID, "resource", resource.URI)
					if err != nil || !canAccess {
						continue
					}
				}

				displayURI := resource.URI
				if multiplexing {
					displayURI = name + toolDelimiter + resource.URI
				}

				prefixedResource := mcp.Resource{
					URI:         displayURI,
					Name:        resource.Name,
					Description: resource.Description,
					MimeType:    resource.MimeType,
				}
				allResources = append(allResources, prefixedResource)

				session.SetResourceMapping(displayURI, ResourceMapping{
					TargetID:   targetID,
					TargetName: name,
					URI:        resource.URI,
				})
			}
			mu.Unlock()
		}(targetName, client)
	}

	wg.Wait()

	return &mcp.ResourcesListResult{Resources: allResources}, nil
}

// ReadResource routes a resource read to the appropriate upstream target
func (p *Proxy) ReadResource(ctx context.Context, session *Session, uri string) (*mcp.ResourceReadResult, error) {
	mapping, exists := session.GetResourceMapping(uri)
	if !exists {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	if p.authorizer != nil {
		canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &mapping.TargetID, "resource", mapping.URI)
		if err != nil || !canAccess {
			return nil, fmt.Errorf("not authorized to read resource: %s", uri)
		}
	}

	client := session.GetClient(mapping.TargetName)
	if client == nil {
		return nil, fmt.Errorf("target not connected: %s", mapping.TargetName)
	}

	return client.ReadResource(ctx, mapping.URI)
}

// ListPrompts aggregates prompts from all connected upstream targets
func (p *Proxy) ListPrompts(ctx context.Context, session *Session) (*mcp.PromptsListResult, error) {
	clients := session.GetAllClients()
	if len(clients) == 0 {
		return &mcp.PromptsListResult{Prompts: []mcp.Prompt{}}, nil
	}

	multiplexing := len(clients) > 1

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPrompts []mcp.Prompt

	session.ClearPromptMappings()

	for targetName, client := range clients {
		wg.Add(1)
		go func(name string, c mcp.MCPClient) {
			defer wg.Done()

			result, err := c.ListPrompts(ctx, nil)
			if err != nil {
				log.Error().Err(err).Str("target", name).Msg("Failed to list prompts")
				return
			}

			targetID, _ := session.GetTargetID(name)

			mu.Lock()
			for _, prompt := range result.Prompts {
				if p.authorizer != nil {
					canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &targetID, "prompt", prompt.Name)
					if err != nil || !canAccess {
						continue
					}
				}

				displayName := prompt.Name
				if multiplexing {
					displayName = name + toolDelimiter + prompt.Name
				}

				prefixedPrompt := mcp.Prompt{
					Name:        displayName,
					Description: prompt.Description,
					Arguments:   prompt.Arguments,
				}
				allPrompts = append(allPrompts, prefixedPrompt)

				session.SetPromptMapping(displayName, PromptMapping{
					TargetID:   targetID,
					TargetName: name,
					PromptName: prompt.Name,
				})
			}
			mu.Unlock()
		}(targetName, client)
	}

	wg.Wait()

	return &mcp.PromptsListResult{Prompts: allPrompts}, nil
}

// GetPrompt routes a prompt get to the appropriate upstream target
func (p *Proxy) GetPrompt(ctx context.Context, session *Session, params *mcp.PromptGetParams) (*mcp.PromptGetResult, error) {
	mapping, exists := session.GetPromptMapping(params.Name)
	if !exists {
		return nil, fmt.Errorf("prompt not found: %s", params.Name)
	}

	if p.authorizer != nil {
		canAccess, _, err := p.authorizer.CanAccess(ctx, session.UserID, session.Role, session.Groups, &mapping.TargetID, "prompt", mapping.PromptName)
		if err != nil || !canAccess {
			return nil, fmt.Errorf("not authorized to get prompt: %s", params.Name)
		}
	}

	client := session.GetClient(mapping.TargetName)
	if client == nil {
		return nil, fmt.Errorf("target not connected: %s", mapping.TargetName)
	}

	originalParams := &mcp.PromptGetParams{
		Name:      mapping.PromptName,
		Arguments: params.Arguments,
	}

	return client.GetPrompt(ctx, originalParams)
}

// ForwardRequest forwards a raw request to all targets or a specific target
func (p *Proxy) ForwardRequest(ctx context.Context, session *Session, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	clients := session.GetAllClients()
	if len(clients) == 0 {
		return mcp.NewErrorResponse(req.ID, mcp.InternalError, "No targets connected"), nil
	}

	for _, client := range clients {
		return client.SendRawRequest(ctx, req)
	}

	return mcp.NewErrorResponse(req.ID, mcp.InternalError, "No clients available"), nil
}

// HandleRequest handles a JSON-RPC request and routes it appropriately
func (p *Proxy) HandleRequest(ctx context.Context, session *Session, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	ctx, span := tracer.Start(ctx, "Proxy.HandleRequest",
		trace.WithAttributes(attribute.String("mcp.method", req.Method)),
	)
	defer span.End()
	start := time.Now()

	switch req.Method {
	case mcp.MethodToolsList:
		result, err := p.ListTools(ctx, session)
		if err != nil {
			p.emitActivity(ctx, start, session, req.Method, "", "", "error")
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		p.emitActivity(ctx, start, session, req.Method, "", "", "ok")
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodToolsCall:
		// CallTool emits its own activity event with target+tool info
		var params mcp.ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "Invalid params"), nil
		}
		result, err := p.CallTool(ctx, session, &params)
		if err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodResourcesList:
		result, err := p.ListResources(ctx, session)
		if err != nil {
			p.emitActivity(ctx, start, session, req.Method, "", "", "error")
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		p.emitActivity(ctx, start, session, req.Method, "", "", "ok")
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodResourcesRead:
		var params mcp.ResourceReadParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "Invalid params"), nil
		}
		result, err := p.ReadResource(ctx, session, params.URI)
		if err != nil {
			p.emitActivity(ctx, start, session, req.Method, "", "", "error")
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		p.emitActivity(ctx, start, session, req.Method, "", "", "ok")
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodPromptsList:
		result, err := p.ListPrompts(ctx, session)
		if err != nil {
			p.emitActivity(ctx, start, session, req.Method, "", "", "error")
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		p.emitActivity(ctx, start, session, req.Method, "", "", "ok")
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodPromptsGet:
		var params mcp.PromptGetParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "Invalid params"), nil
		}
		result, err := p.GetPrompt(ctx, session, &params)
		if err != nil {
			p.emitActivity(ctx, start, session, req.Method, "", "", "error")
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error()), nil
		}
		p.emitActivity(ctx, start, session, req.Method, "", "", "ok")
		return mcp.NewSuccessResponse(req.ID, result)

	case mcp.MethodPing:
		return mcp.NewSuccessResponse(req.ID, map[string]interface{}{})

	default:
		return p.ForwardRequest(ctx, session, req)
	}
}

// emitActivity publishes an activity event via the observability hub.
func (p *Proxy) emitActivity(ctx context.Context, start time.Time, session *Session, method, target, tool, status string) {
	if p.obsHub == nil {
		return
	}
	traceID := ""
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		traceID = span.SpanContext().TraceID().String()
	}
	userEmail, _ := auth.GetUserEmail(ctx)
	p.obsHub.EmitActivity(observability.ActivityEvent{
		Timestamp:  start,
		UserID:     session.UserID.String(),
		UserEmail:  userEmail,
		Method:     method,
		Target:     target,
		Tool:       tool,
		DurationMS: float64(time.Since(start).Milliseconds()),
		Status:     status,
		TraceID:    traceID,
	})
}

// GetAuthorizer returns the authorizer
func (p *Proxy) GetAuthorizer() *Authorizer {
	return p.authorizer
}

// parseResourceName splits a prefixed resource name into (targetName, resourceName).
// E.g., "github_list_repos" with delimiter "_" → tries to match known target names.
func parseResourceName(name string) (string, string) {
	idx := strings.Index(name, toolDelimiter)
	if idx == -1 {
		return "", name
	}
	return name[:idx], name[idx+len(toolDelimiter):]
}

// createStdioClient creates a STDIO MCP client with resolved environment configs
func (p *Proxy) createStdioClient(ctx context.Context, session *Session, target *database.Target) (mcp.MCPClient, error) {
	_, span := tracer.Start(ctx, "createStdioClient",
		trace.WithAttributes(attribute.String("target.name", target.Name)),
	)
	defer span.End()

	if p.stdioManager == nil {
		return nil, fmt.Errorf("STDIO manager not configured")
	}
	if target.Command == "" {
		return nil, fmt.Errorf("STDIO target %s has no command configured", target.Name)
	}

	// Compute subject key for isolation
	subjectKey := stdio.ComputeSubjectKey(target, session.UserID.String(), session.Role, session.Groups)

	// Resolve environment configs
	envConfigs, err := p.repo.ResolveEnvConfigsForTarget(ctx, target.ID, session.UserID, session.Role, session.Groups)
	if err != nil {
		log.Warn().Err(err).Str("target", target.Name).Msg("Failed to resolve env configs for STDIO")
	}

	// Build environment: inherit current env + add resolved configs
	env := os.Environ()
	if envConfigs != nil && p.encryptor != nil {
		for key, config := range envConfigs {
			decrypted, err := p.encryptor.Decrypt(config.Value)
			if err == nil {
				env = append(env, key+"="+decrypted)
			}
		}
	}

	// Also inject legacy token as env var if available
	if target.AuthType == "bearer" || target.AuthType == "header" {
		tokenInfo, err := p.repo.ResolveTokenForTarget(ctx, session.UserID, session.Role, session.Groups, target.ID)
		if err == nil && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(tokenInfo.Token)
			if err == nil {
				// Only set if not already set by env configs
				hasAuthToken := false
				if envConfigs != nil {
					_, hasAuthToken = envConfigs["AUTH_TOKEN"]
				}
				if !hasAuthToken {
					env = append(env, "AUTH_TOKEN="+decrypted)
				}
			}
		}
	}

	proc, err := p.stdioManager.GetOrCreateForTarget(ctx, subjectKey, target, env)
	if err != nil {
		return nil, fmt.Errorf("STDIO process: %w", err)
	}

	log.Info().
		Str("target", target.Name).
		Str("subject_key", subjectKey).
		Str("isolation", target.IsolationBoundary).
		Msg("Using STDIO client")

	return proc, nil
}

// createK8sClient creates a Kubernetes-managed MCP client with resolved environment configs
func (p *Proxy) createK8sClient(ctx context.Context, session *Session, target *database.Target) (mcp.MCPClient, error) {
	_, span := tracer.Start(ctx, "createK8sClient",
		trace.WithAttributes(
			attribute.String("target.name", target.Name),
			attribute.String("target.image", target.Image),
		),
	)
	defer span.End()

	if p.k8sManager == nil {
		return nil, fmt.Errorf("Kubernetes manager not configured")
	}
	if target.Image == "" {
		return nil, fmt.Errorf("Kubernetes target %s has no image configured", target.Name)
	}

	// Compute subject key for isolation (reuse STDIO logic)
	subjectKey := stdio.ComputeSubjectKey(target, session.UserID.String(), session.Role, session.Groups)

	// Resolve environment configs
	envConfigs, err := p.repo.ResolveEnvConfigsForTarget(ctx, target.ID, session.UserID, session.Role, session.Groups)
	if err != nil {
		log.Warn().Err(err).Str("target", target.Name).Msg("Failed to resolve env configs for Kubernetes")
	}

	// Decrypt env configs
	decryptedEnv := make(map[string]string)
	if envConfigs != nil && p.encryptor != nil {
		for key, config := range envConfigs {
			decrypted, err := p.encryptor.Decrypt(config.Value)
			if err == nil {
				decryptedEnv[key] = decrypted
			}
		}
	}

	// Also inject legacy token if applicable
	if target.AuthType == "bearer" || target.AuthType == "header" {
		tokenInfo, err := p.repo.ResolveTokenForTarget(ctx, session.UserID, session.Role, session.Groups, target.ID)
		if err == nil && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(tokenInfo.Token)
			if err == nil {
				if _, hasAuthToken := decryptedEnv["AUTH_TOKEN"]; !hasAuthToken {
					decryptedEnv["AUTH_TOKEN"] = decrypted
				}
			}
		}
	}

	client, err := p.k8sManager.GetOrCreate(ctx, subjectKey, target, decryptedEnv)
	if err != nil {
		return nil, fmt.Errorf("Kubernetes instance: %w", err)
	}

	log.Info().
		Str("target", target.Name).
		Str("subject_key", subjectKey).
		Str("isolation", target.IsolationBoundary).
		Str("image", target.Image).
		Msg("Using Kubernetes client")

	return client, nil
}

// createHTTPClient creates an HTTP MCP client with resolved environment configs
func (p *Proxy) createHTTPClient(ctx context.Context, session *Session, target *database.Target) (*mcp.Client, error) {
	_, span := tracer.Start(ctx, "createHTTPClient",
		trace.WithAttributes(
			attribute.String("target.name", target.Name),
			attribute.String("target.transport", target.TransportType),
		),
	)
	defer span.End()

	cfg := mcp.ClientConfig{
		URL:           target.URL,
		CustomHeaders: make(map[string]string),
		TransportType: mcp.TransportType(target.TransportType),
	}

	// Resolve environment configs for this target
	envConfigs, err := p.repo.ResolveEnvConfigsForTarget(ctx, target.ID, session.UserID, session.Role, session.Groups)
	if err != nil {
		log.Warn().Err(err).Str("target", target.Name).Msg("Failed to resolve env configs, using defaults")
	}

	// Apply environment configs
	if envConfigs != nil {
		if authToken, ok := envConfigs["AUTH_TOKEN"]; ok && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(authToken.Value)
			if err == nil {
				cfg.AuthToken = decrypted
				log.Debug().
					Str("target", target.Name).
					Str("token_source", authToken.Source).
					Msg("Using AUTH_TOKEN from env config")
			}
		}

		if authHeader, ok := envConfigs["AUTH_HEADER"]; ok && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(authHeader.Value)
			if err == nil {
				cfg.AuthHeader = decrypted
			}
		} else if target.AuthHeaderName != "" {
			cfg.AuthHeader = target.AuthHeaderName
		}

		if baseURL, ok := envConfigs["BASE_URL"]; ok && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(baseURL.Value)
			if err == nil {
				cfg.URL = decrypted
				log.Debug().
					Str("target", target.Name).
					Str("url", decrypted).
					Msg("Using BASE_URL override from env config")
			}
		}

		if timeout, ok := envConfigs["TIMEOUT"]; ok && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(timeout.Value)
			if err == nil {
				if d, err := time.ParseDuration(decrypted); err == nil {
					cfg.Timeout = d
				}
			}
		}

		reservedKeys := map[string]bool{
			"AUTH_TOKEN":  true,
			"AUTH_HEADER": true,
			"BASE_URL":    true,
			"TIMEOUT":     true,
		}

		for key, config := range envConfigs {
			if !reservedKeys[key] && p.encryptor != nil {
				decrypted, err := p.encryptor.Decrypt(config.Value)
				if err == nil {
					cfg.CustomHeaders["X-Env-"+key] = decrypted
				}
			}
		}
	}

	// Fallback to legacy token resolution
	if cfg.AuthToken == "" && (target.AuthType == "bearer" || target.AuthType == "header") {
		tokenInfo, err := p.repo.ResolveTokenForTarget(ctx, session.UserID, session.Role, session.Groups, target.ID)
		if err == nil && p.encryptor != nil {
			decrypted, err := p.encryptor.Decrypt(tokenInfo.Token)
			if err == nil {
				cfg.AuthToken = decrypted
				if target.AuthHeaderName != "" && cfg.AuthHeader == "" {
					cfg.AuthHeader = target.AuthHeaderName
				}
				log.Debug().
					Str("target", target.Name).
					Str("token_source", tokenInfo.Source).
					Msg("Using legacy token for target")
			}
		}
	}

	return mcp.NewClient(cfg), nil
}
