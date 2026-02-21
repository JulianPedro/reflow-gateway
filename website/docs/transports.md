---
sidebar_position: 7
title: Transports
---

# Transport Types

The gateway supports four transport types for connecting to upstream MCP servers.

## 1. Streamable HTTP (default)

For MCP servers accessible via HTTP. The gateway sends JSON-RPC requests as HTTP POST.

```json
{
  "name": "github-mcp",
  "url": "https://mcp-github.example.com/mcp",
  "transport_type": "streamable-http",
  "auth_type": "bearer"
}
```

- Gateway auto-detects between Streamable HTTP and SSE
- Tries POST first; falls back to SSE if needed
- Credentials injected as HTTP headers

## 2. SSE (Server-Sent Events)

For legacy MCP servers that use the SSE transport.

```json
{
  "name": "legacy-server",
  "url": "https://legacy-mcp.example.com/sse",
  "transport_type": "sse",
  "auth_type": "bearer"
}
```

- Gateway maintains a persistent SSE connection for responses
- Sends JSON-RPC requests via POST to the server's message endpoint
- Credentials injected as HTTP headers

## 3. STDIO

For MCP servers that run as local processes, communicating via stdin/stdout.

```json
{
  "name": "github-stdio",
  "transport_type": "stdio",
  "command": "npx",
  "args": ["@modelcontextprotocol/server-github"],
  "statefulness": "stateful",
  "isolation_boundary": "per_user"
}
```

- Gateway spawns the process with `exec.Command`
- Communicates via line-delimited JSON-RPC on stdin/stdout
- Environment variables injected at process startup
- Process lifecycle managed by the STDIO Manager (pool + GC)

**When to use:** Most MCP servers (GitHub, Jira, Slack) receive API tokens via environment variables at startup. STDIO transport enables isolated processes with correct credentials per user/group/role.

## 4. Kubernetes (CRD-managed)

For enterprise deployments where MCP servers run as isolated pods.

```json
{
  "name": "github-k8s",
  "transport_type": "kubernetes",
  "image": "ghcr.io/org/mcp-github:latest",
  "port": 8080,
  "statefulness": "stateful",
  "isolation_boundary": "per_user"
}
```

- Gateway creates an `MCPInstance` CR + Secret per subject key
- Operator reconciles into Pod + ClusterIP Service
- Gateway connects via HTTP (Streamable HTTP)
- Environment variables injected via Kubernetes Secrets

See [Kubernetes Operator](./kubernetes-operator) for details.

## Statefulness

Each target has a `statefulness` setting:

| Value | Description | Use Case |
|-------|-------------|----------|
| `stateless` | No state between requests | HTTP proxy targets, read-only servers |
| `stateful` | Maintains state (credentials, sessions) | Most real servers (GitHub, Jira) |

## Isolation Boundary

Controls how STDIO/K8s processes are shared between users:

| Value | Process Sharing | Subject Key | Use Case |
|-------|----------------|-------------|----------|
| `shared` | One for all users | `shared:<targetID>` | Stateless, read-only tools |
| `per_role` | One per role | `role:<hash>` | Role-based API keys |
| `per_group` | One per group | `group:<hash>` | Team-based API keys |
| `per_user` | One per user | `user:<hash>` | User-specific API keys |

### Process Lifecycle

- **Idle TTL** (default: 30min): killed if no requests
- **Max Lifetime** (default: 24h): killed regardless of activity
- **GC Interval** (default: 1min): check frequency
- **Max Processes** (default: 100): hard limit on concurrent processes

## Tool Prefixing

When a session connects to multiple targets, tools are prefixed with the target name using `_` as delimiter:

- Single target: `list_repos`, `create_issue`
- Multiple targets: `github_list_repos`, `jira_create_issue`

## MCP Client Connection

### Streamable HTTP (recommended)

```bash
# 1. Initialize session
POST /mcp
Authorization: Bearer <token>
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
  "protocolVersion":"2025-03-26",
  "capabilities":{},
  "clientInfo":{"name":"my-client","version":"1.0"}
}}

# Response includes Mcp-Session-Id header

# 2. List tools
POST /mcp
Mcp-Session-Id: <session-id>
{"jsonrpc":"2.0","id":2,"method":"tools/list"}

# 3. Call a tool
POST /mcp
Mcp-Session-Id: <session-id>
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{
  "name":"github_list_repos","arguments":{"owner":"myorg"}
}}

# 4. Terminate session
DELETE /mcp
Mcp-Session-Id: <session-id>
```

### SSE Transport (legacy)

```bash
# 1. Connect SSE stream
GET /mcp
Authorization: Bearer <token>
# Receives: event: endpoint, data: /mcp?session_id=xxx

# 2. Send requests via POST to the endpoint URL
POST /mcp?session_id=xxx
{"jsonrpc":"2.0","id":1,"method":"initialize",...}
```
