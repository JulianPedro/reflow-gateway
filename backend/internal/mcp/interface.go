package mcp

import "context"

// MCPClient defines the interface for communicating with an MCP server.
// Both the HTTP *Client and the STDIO Process implement this interface.
type MCPClient interface {
	Initialize(ctx context.Context, params *InitializeParams) (*InitializeResult, error)
	ListTools(ctx context.Context, cursor *string) (*ToolsListResult, error)
	CallTool(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error)
	ListResources(ctx context.Context, cursor *string) (*ResourcesListResult, error)
	ReadResource(ctx context.Context, uri string) (*ResourceReadResult, error)
	ListPrompts(ctx context.Context, cursor *string) (*PromptsListResult, error)
	GetPrompt(ctx context.Context, params *PromptGetParams) (*PromptGetResult, error)
	SendRawRequest(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error)
	IsInitialized() bool
	GetCapabilities() *ServerCapabilities
	GetServerInfo() *ServerInfo
	Close() error
}
