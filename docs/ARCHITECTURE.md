# Reflow Gateway - Architecture & Configuration Guide

## Overview

Reflow Gateway is an MCP (Model Context Protocol) multiplexing gateway. It sits between AI clients (Claude, Cursor, etc.) and multiple upstream MCP servers (GitHub, Jira, Slack, filesystem, etc.), providing:

- **Multiplexing**: Aggregate tools/resources/prompts from multiple MCP servers into a single endpoint
- **Authentication**: JWT-based auth for all MCP requests
- **Authorization**: Fine-grained, default-deny policy engine (target, tool, resource, prompt level)
- **Credential Injection**: Gateway resolves and injects upstream credentials per user/role/group
- **Transport Abstraction**: Supports Streamable HTTP, SSE, STDIO, and Kubernetes transports to upstream servers
- **Process/Pod Lifecycle**: Manages STDIO processes and Kubernetes pods with isolation and automatic GC
- **Audit Logging**: All requests and authorization decisions are logged

```
                          Reflow Gateway
                     +-----------------------+
  AI Clients         |                       |       Upstream MCP Servers
  (Claude,     ---->  |  POST /mcp (JSON-RPC) |  --->  GitHub (Streamable HTTP)
   Cursor,     ---->  |  GET  /mcp (SSE)      |  --->  Jira   (SSE)
   etc.)       ---->  |  DELETE /mcp           |  --->  Custom (STDIO process)
                     |                       |
                     |  JWT Auth + Policies   |
                     |  Credential Injection  |
                     |  Tool Multiplexing     |
                     +-----------------------+
                              |
                          PostgreSQL
                     (sessions, policies,
                      tokens, audit logs)
```

## Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22+ with Chi router |
| Database | PostgreSQL 15+ |
| Frontend | Next.js 14 + TanStack Query + shadcn/ui |
| Protocol | MCP Streamable HTTP (JSON-RPC 2.0 over HTTP) |

## Project Structure

```
backend/
  cmd/server/main.go          # Entry point
  internal/
    api/
      handlers.go              # REST API handlers (CRUD for targets, users, policies, etc.)
      policy_handlers.go       # Authorization policy CRUD
      env_handlers.go          # Environment config CRUD
      routes.go                # API route definitions (/api/...)
    auth/
      jwt.go                   # JWT creation/validation
      middleware.go            # Auth middleware (extracts user context)
      encryption.go            # AES-GCM encryption for stored tokens
    config/
      config.go                # YAML config with env var expansion
    database/
      db.go                    # PostgreSQL connection + migration runner
      models.go                # Data models
      repository.go            # All SQL queries
      migrations/              # Ordered SQL migration files
    gateway/
      handler.go               # MCP Streamable HTTP handler (POST/GET/DELETE /mcp)
      proxy.go                 # Upstream proxy, tool aggregation, credential injection
      session.go               # In-memory session management
      authorizer.go            # Policy evaluation engine
    mcp/
      client.go                # HTTP MCP client (Streamable HTTP + SSE auto-detect)
      interface.go             # MCPClient interface
      types.go                 # MCP protocol types (JSON-RPC, tools, resources, prompts)
      sse.go                   # SSE writer for notification streams
    stdio/
      process.go               # STDIO MCP process wrapper (stdin/stdout JSON-RPC)
      manager.go               # Process pool with GC (idle TTL, max lifetime)
    k8s/
      manager.go               # Kubernetes MCPInstance CR management + cached HTTP clients
operator/
  cmd/main.go                  # Operator entry point (controller-runtime)
  api/v1alpha1/
    mcpinstance_types.go       # MCPInstance CRD type definitions
    groupversion_info.go       # group: mcp.reflow.io, version: v1alpha1
  internal/controller/
    mcpinstance_controller.go  # Pod + Service reconciler with GC
  config/
    crd/bases/                 # CRD YAML manifest
    rbac/                      # RBAC manifests
    manager/                   # Controller manager deployment
frontend/
  src/
    app/                       # Next.js pages (dashboard, targets, policies, users, logs)
    lib/api.ts                 # API client + TypeScript types
    components/                # Shared UI components
```

## Setup

### Prerequisites

- Go 1.22+
- Node.js 18+
- PostgreSQL 15+

### Configuration

Create `config.yaml` at the project root:

```yaml
server:
  port: 3000
  host: 0.0.0.0

database:
  host: localhost
  port: 5432
  user: reflow
  password: ${DB_PASSWORD}
  database: reflow_gateway
  sslmode: disable

jwt:
  secret: ${JWT_SECRET}

cors:
  allowed_origins:
    - "http://localhost:3001"
    - "http://localhost:3000"
  allowed_headers:
    - "*"
  expose_headers:
    - "Mcp-Session-Id"
    - "MCP-Protocol-Version"

session:
  timeout: 30m             # Session expires after 30min of inactivity
  cleanup_interval: 5m     # Check for expired sessions every 5min

encryption:
  key: ${ENCRYPTION_KEY}   # Must be exactly 32 characters (AES-256)

logging:
  level: info              # debug, info, warn, error
  format: json             # json or console

stdio:
  idle_ttl: 30m            # Kill idle STDIO processes after 30min
  max_lifetime: 24h        # Kill STDIO processes after 24h regardless
  gc_interval: 1m          # Check for idle processes every 1min
  max_processes: 100       # Maximum concurrent STDIO processes
```

Environment variables referenced with `${VAR_NAME}` are expanded at load time.

### Environment Variables

```bash
DB_PASSWORD=your_db_password
JWT_SECRET=your-super-secret-jwt-key-change-in-production
ENCRYPTION_KEY=12345678901234567890123456789012  # exactly 32 chars
NEXT_PUBLIC_API_URL=http://localhost:3000
```

### Running

```bash
# Backend
cd backend && go build -o reflow-gateway ./cmd/server && ./reflow-gateway -config ../config.yaml

# Frontend
cd frontend && npm install && npm run dev
```

The first user to register automatically gets the `admin` role.

---

## Authentication

All MCP requests require a JWT token in the `Authorization: Bearer <token>` header.

### User Registration & Login

```bash
# Register (first user gets admin role)
POST /api/auth/register
{ "email": "admin@example.com", "password": "secure123" }

# Login
POST /api/auth/login
{ "email": "admin@example.com", "password": "secure123" }
# Returns: { "user": {...}, "token": "eyJ..." }
```

### API Tokens

For programmatic access (MCP clients), create long-lived API tokens:

```bash
POST /api/auth/tokens
{ "name": "my-claude-token" }
# Returns: { "api_token": {...}, "token": "eyJ..." }
```

API tokens are tied to a user and inherit their role/groups.

---

## Authorization Model

The gateway operates on a **default-deny** model. Without policies, no user can access any target.

### Policy Structure

```json
{
  "name": "Allow admins everything",
  "description": "Full access for admin role",
  "target_id": null,
  "resource_type": "all",
  "resource_pattern": null,
  "effect": "allow",
  "priority": 100,
  "enabled": true,
  "subjects": [
    { "subject_type": "role", "subject_value": "admin" }
  ]
}
```

### Fields

| Field | Values | Description |
|-------|--------|-------------|
| `target_id` | UUID or null | Applies to specific target, or all targets if null |
| `resource_type` | `all`, `tool`, `resource`, `prompt` | What type of MCP resource |
| `resource_pattern` | regex or null | Pattern match on resource name (e.g., `list_.*`) |
| `effect` | `allow`, `deny` | Grant or deny access |
| `priority` | integer | Higher priority = evaluated first |
| `subjects` | array | Who this policy applies to |

### Subject Types

| Type | Description |
|------|-------------|
| `everyone` | Matches all authenticated users |
| `role` | Matches users with specific role (e.g., `admin`, `user`, `developer`) |
| `group` | Matches users in specific group |
| `user` | Matches specific user by ID |

### Evaluation Order

1. Policies are sorted by priority (descending)
2. First matching policy wins
3. If no policy matches, access is **denied**

### Common Policy Scenarios

**Allow all users to access all targets:**
```json
{
  "name": "Global allow",
  "resource_type": "all",
  "effect": "allow",
  "priority": 1,
  "subjects": [{ "subject_type": "everyone" }]
}
```

**Allow specific role to use specific tools:**
```json
{
  "name": "Developers can use GitHub tools",
  "target_id": "<github-target-id>",
  "resource_type": "tool",
  "resource_pattern": ".*",
  "effect": "allow",
  "priority": 10,
  "subjects": [{ "subject_type": "role", "subject_value": "developer" }]
}
```

**Deny dangerous tools for non-admins:**
```json
{
  "name": "Block delete tools",
  "resource_type": "tool",
  "resource_pattern": "delete_.*",
  "effect": "deny",
  "priority": 100,
  "subjects": [{ "subject_type": "everyone" }]
}
```

---

## Transport Types

The gateway supports four transport types for connecting to upstream MCP servers.

### 1. Streamable HTTP (default)

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

### 2. SSE (Server-Sent Events)

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

### 3. STDIO

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
- Environment variables are injected at process startup
- Process lifecycle managed by `StdioManager` (pool + GC)

**When to use STDIO:** Most real MCP servers (GitHub, Jira, Slack, etc.) receive their API tokens via environment variables at startup, not via HTTP headers per-request. STDIO transport enables the gateway to spawn isolated processes with the correct credentials for each user/group/role.

### 4. Kubernetes (CRD-managed)

For enterprise deployments where MCP servers run as isolated pods in a Kubernetes cluster. The gateway creates `MCPInstance` custom resources, and a separate operator reconciles them into Pods + Services.

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

- Gateway creates an `MCPInstance` CR + Secret per subject key (user/role/group)
- Operator (separate binary) reconciles the CR into a Pod + ClusterIP Service
- Gateway polls until the pod is Ready, then connects via HTTP (Streamable HTTP)
- Environment variables are injected via Kubernetes Secrets (`envFrom`)
- Pod lifecycle managed by the operator (idle TTL, max lifetime, GC)
- Owner references ensure cascading cleanup (delete CR → delete Pod + Service + Secret)

**When to use Kubernetes:** When you need the isolation benefits of STDIO (per-user/group/role processes) but in a distributed Kubernetes environment instead of on the gateway host. Ideal for multi-tenant deployments with resource limits, automatic scaling, and pod-level security boundaries.

**Architecture:**

```
Gateway                         Kubernetes Cluster
┌────────────┐                 ┌──────────────────────────────┐
│  proxy.go  │  create CR      │  Operator (controller)       │
│  k8sManager├────────────────►│  MCPInstance Reconciler       │
│            │                 │       │ creates               │
│   HTTP req │  HTTP (mcp)     │  Secret  Pod    Service       │
│            ├────────────────►│        (MCP)   (ClusterIP)    │
└────────────┘                 └──────────────────────────────┘
```

**Configuration:**

```yaml
kubernetes:
  enabled: true
  namespace: reflow       # Namespace for MCPInstance CRs
  kubeconfig: ""          # Empty = in-cluster config
  idle_ttl: 30m           # Delete idle instances after 30min
  max_lifetime: 24h       # Max instance lifetime
  gc_interval: 1m         # GC check interval
  max_instances: 100      # Max concurrent instances
```

**Operator deployment:**

```bash
# Install CRD
kubectl apply -f operator/config/crd/bases/

# Deploy operator
kubectl apply -f operator/config/rbac/
kubectl apply -f operator/config/manager/
```

---

## Statefulness Model

Each target has a `statefulness` level that describes whether the MCP server holds state:

| Value | Description | Use Case |
|-------|-------------|----------|
| `stateless` | No state between requests | HTTP proxy targets, read-only servers |
| `stateful` | Maintains state (credentials, sessions, memory) | Most real servers (GitHub, Jira), conversation-aware servers |

This field is informational for HTTP/SSE targets but critical for STDIO targets, where it informs the isolation strategy.

---

## Isolation Boundary

Controls how STDIO processes are shared (or not) between users:

| Value | Process Sharing | Subject Key | Use Case |
|-------|----------------|-------------|----------|
| `shared` | One process for all users | `shared:<targetID>` | Stateless servers, read-only tools |
| `per_role` | One process per role | `role:<hash>` | Role-based API keys |
| `per_group` | One process per group | `group:<hash>` | Team-based API keys |
| `per_user` | One process per user | `user:<hash>` | User-specific API keys (most secure) |

### How it works

1. When a user sends `initialize`, the gateway computes a **subject key** based on the target's isolation boundary and the user's identity
2. The `StdioManager` checks if a process for that subject key already exists
3. If yes, reuse it. If no, spawn a new process with the appropriate environment

### Process Lifecycle

- **Idle TTL** (default: 30min): Process is killed if no requests for this duration
- **Max Lifetime** (default: 24h): Process is killed after this duration regardless of activity
- **GC Interval** (default: 1min): How often the manager checks for idle/expired processes
- **Max Processes** (default: 100): Hard limit on concurrent STDIO processes

---

## Session Recycle

When a user's role or groups change (e.g., via IdP like Okta/Azure AD, or admin action), existing MCP sessions become stale because STDIO processes were spawned with old credentials/isolation keys. The gateway handles this in two ways:

### Auto-Detection

On every MCP request, the gateway compares the JWT claims (role, groups) with the session's stored values. If they differ, the session is automatically recycled:

1. All HTTP upstream clients are closed
2. Session state (tool/resource/prompt mappings) is cleared
3. Identity context is updated to match new claims
4. Client receives an error asking it to re-initialize
5. On re-initialization, new STDIO processes are spawned with correct isolation keys

### Explicit Recycle API

For cases where sessions need to be recycled proactively (e.g., IdP webhook, admin action):

```bash
# User: recycle own sessions
POST /api/sessions/recycle
Authorization: Bearer <token>

# Admin: recycle another user's sessions
POST /api/users/{id}/recycle
Authorization: Bearer <admin-token>

# Response
{ "recycled": 3, "user_id": "..." }
```

### When to Use

- **Auto-detection** handles the common case: user logs in with updated JWT after IdP changes
- **Explicit recycle** is for:
  - IdP webhooks that fire when group memberships change
  - Admin actions that update user roles/groups via the REST API
  - Forcing credential rotation without waiting for session expiry

---

## Credential Management

The gateway resolves credentials for upstream MCP servers based on the requesting user's identity. Credentials are **never** forwarded from the user's JWT.

### Resolution Priority

1. **User-specific** credential (highest priority)
2. **Group-specific** credential
3. **Role-specific** credential
4. **Default** credential (lowest priority)

### Two credential systems

#### 1. Legacy Tokens (per-target)

Simple bearer/header tokens. Set via API:

```bash
# Default token (fallback for all users)
PUT /api/targets/{id}/tokens/default
{ "token": "ghp_xxxxxxxxxxxx" }

# Role-specific token
PUT /api/targets/{id}/tokens/role
{ "role": "developer", "token": "ghp_dev_token" }

# Group-specific token
PUT /api/targets/{id}/tokens/group
{ "group": "engineering", "token": "ghp_eng_token" }

# User-specific token (set by the user themselves)
PUT /api/targets/{id}/token
{ "token": "ghp_my_personal_token" }
```

For **HTTP/SSE targets**, the resolved token is injected as:
- `Authorization: Bearer <token>` (if `auth_type=bearer`)
- `<auth_header_name>: <token>` (if `auth_type=header`)

For **STDIO targets**, the resolved token is injected as:
- `AUTH_TOKEN=<token>` environment variable

#### 2. Environment Configs (advanced)

Key-value configurations that are injected based on scope. This is the recommended approach for STDIO targets.

```bash
# Set a default env config
POST /api/targets/{id}/env/default
{ "key": "GITHUB_TOKEN", "value": "ghp_xxxxxxxxxxxx", "description": "GitHub API token" }

# Set per-role
POST /api/targets/{id}/env/role/developer
{ "key": "GITHUB_TOKEN", "value": "ghp_dev_token" }

# Set per-group
POST /api/targets/{id}/env/group/engineering
{ "key": "GITHUB_TOKEN", "value": "ghp_eng_token" }

# Set per-user
POST /api/targets/{id}/env/user/{userId}
{ "key": "GITHUB_TOKEN", "value": "ghp_personal_token" }
```

**Resolution priority:** User > Group > Role > Default (per key).

**For STDIO targets**, all resolved env configs are injected as environment variables when spawning the process.

**For HTTP/SSE targets**, reserved keys have special behavior:

| Key | Behavior |
|-----|----------|
| `AUTH_TOKEN` | Used as bearer/auth token |
| `AUTH_HEADER` | Override the auth header name |
| `BASE_URL` | Override the target URL |
| `TIMEOUT` | Set request timeout (e.g., `30s`, `2m`) |
| Other keys | Passed as `X-Env-<KEY>` custom headers |

All values are stored AES-256-GCM encrypted in the database.

---

## Scenarios

### Scenario 1: HTTP MCP Server with Shared Token

Use case: A self-hosted MCP server with a single API key.

```bash
# 1. Create target
POST /api/targets
{
  "name": "internal-tools",
  "url": "https://mcp-tools.internal.company.com/mcp",
  "transport_type": "streamable-http",
  "auth_type": "bearer"
}

# 2. Set default token
PUT /api/targets/{id}/tokens/default
{ "token": "api-key-xxxxx" }

# 3. Create allow-all policy
POST /api/policies
{
  "name": "Allow all to internal-tools",
  "target_id": "{id}",
  "resource_type": "all",
  "effect": "allow",
  "priority": 1,
  "subjects": [{ "subject_type": "everyone" }]
}
```

### Scenario 2: GitHub MCP Server per-User (STDIO)

Use case: Each user has their own GitHub Personal Access Token.

```bash
# 1. Create STDIO target
POST /api/targets
{
  "name": "github",
  "transport_type": "stdio",
  "command": "npx",
  "args": ["@modelcontextprotocol/server-github"],
  "statefulness": "stateful",
  "isolation_boundary": "per_user"
}

# 2. Set env config for each user
POST /api/targets/{id}/env/user/{user1Id}
{ "key": "GITHUB_PERSONAL_ACCESS_TOKEN", "value": "ghp_user1_token" }

POST /api/targets/{id}/env/user/{user2Id}
{ "key": "GITHUB_PERSONAL_ACCESS_TOKEN", "value": "ghp_user2_token" }

# 3. Create policy
POST /api/policies
{
  "name": "Developers can use GitHub",
  "target_id": "{id}",
  "resource_type": "all",
  "effect": "allow",
  "priority": 10,
  "subjects": [{ "subject_type": "role", "subject_value": "developer" }]
}
```

**Result:** When User1 connects, the gateway spawns `npx @modelcontextprotocol/server-github` with `GITHUB_PERSONAL_ACCESS_TOKEN=ghp_user1_token`. User2 gets a separate process with their own token.

### Scenario 3: Jira MCP Server per-Team (STDIO)

Use case: Engineering and Product teams have different Jira API tokens.

```bash
# 1. Create target
POST /api/targets
{
  "name": "jira",
  "transport_type": "stdio",
  "command": "npx",
  "args": ["@modelcontextprotocol/server-jira"],
  "statefulness": "stateful",
  "isolation_boundary": "per_group"
}

# 2. Set env configs per group
POST /api/targets/{id}/env/group/engineering
{ "key": "JIRA_API_TOKEN", "value": "eng_jira_token" }
{ "key": "JIRA_BASE_URL", "value": "https://eng.atlassian.net" }

POST /api/targets/{id}/env/group/product
{ "key": "JIRA_API_TOKEN", "value": "prod_jira_token" }
{ "key": "JIRA_BASE_URL", "value": "https://product.atlassian.net" }

# 3. Create policy
POST /api/policies
{
  "name": "Allow Jira access",
  "target_id": "{id}",
  "resource_type": "all",
  "effect": "allow",
  "priority": 10,
  "subjects": [
    { "subject_type": "group", "subject_value": "engineering" },
    { "subject_type": "group", "subject_value": "product" }
  ]
}
```

**Result:** All engineering users share one Jira process; all product users share another.

### Scenario 4: Mixed Transport Setup

Use case: Some servers are remote (HTTP), others are local (STDIO).

```bash
# Remote HTTP target
POST /api/targets
{
  "name": "remote-search",
  "url": "https://search-mcp.example.com/mcp",
  "transport_type": "streamable-http",
  "auth_type": "bearer",
  "statefulness": "stateless",
  "isolation_boundary": "shared"
}

# Local STDIO target
POST /api/targets
{
  "name": "filesystem",
  "transport_type": "stdio",
  "command": "npx",
  "args": ["@modelcontextprotocol/server-filesystem", "/data"],
  "statefulness": "stateless",
  "isolation_boundary": "shared"
}
```

When a client connects, it sees tools from both targets, prefixed with the target name (e.g., `remote-search_search`, `filesystem_read_file`). Single-target sessions don't add prefixes.

### Scenario 5: Restricting Specific Tools

Use case: Allow GitHub access but block destructive operations.

```bash
# 1. Allow GitHub for developers (low priority)
POST /api/policies
{
  "name": "Developers GitHub access",
  "target_id": "{github-id}",
  "resource_type": "all",
  "effect": "allow",
  "priority": 10,
  "subjects": [{ "subject_type": "role", "subject_value": "developer" }]
}

# 2. Block delete tools for everyone (high priority)
POST /api/policies
{
  "name": "Block destructive GitHub tools",
  "target_id": "{github-id}",
  "resource_type": "tool",
  "resource_pattern": "delete_.*|remove_.*",
  "effect": "deny",
  "priority": 100,
  "subjects": [{ "subject_type": "everyone" }]
}

# 3. But allow admins to use delete tools (highest priority)
POST /api/policies
{
  "name": "Admins can delete",
  "target_id": "{github-id}",
  "resource_type": "tool",
  "resource_pattern": "delete_.*|remove_.*",
  "effect": "allow",
  "priority": 200,
  "subjects": [{ "subject_type": "role", "subject_value": "admin" }]
}
```

### Scenario 6: GitHub MCP Server per-User (Kubernetes)

Use case: Same as Scenario 2, but running in Kubernetes instead of locally.

```bash
# 1. Create Kubernetes target
POST /api/targets
{
  "name": "github-k8s",
  "transport_type": "kubernetes",
  "image": "ghcr.io/github/mcp-server:latest",
  "port": 8080,
  "statefulness": "stateful",
  "isolation_boundary": "per_user"
}

# 2. Set env config for each user (injected into pod via Secret)
POST /api/targets/{id}/env/user/{user1Id}
{ "key": "GITHUB_PERSONAL_ACCESS_TOKEN", "value": "ghp_user1_token" }

POST /api/targets/{id}/env/user/{user2Id}
{ "key": "GITHUB_PERSONAL_ACCESS_TOKEN", "value": "ghp_user2_token" }

# 3. Create policy
POST /api/policies
{
  "name": "Developers can use GitHub",
  "target_id": "{id}",
  "resource_type": "all",
  "effect": "allow",
  "priority": 10,
  "subjects": [{ "subject_type": "role", "subject_value": "developer" }]
}
```

**Result:** When User1 connects, the gateway creates an MCPInstance CR + Secret. The operator creates a pod running `ghcr.io/github/mcp-server:latest` with `GITHUB_PERSONAL_ACCESS_TOKEN=ghp_user1_token`. Gateway connects via the ClusterIP service. User2 gets a separate pod with their own token.

---

## MCP Client Connection

### Streamable HTTP (recommended)

```bash
# 1. Initialize session
POST /mcp
Authorization: Bearer <token>
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}}}

# Response includes Mcp-Session-Id header

# 2. List tools
POST /mcp
Authorization: Bearer <token>
Mcp-Session-Id: <session-id>

{"jsonrpc":"2.0","id":2,"method":"tools/list"}

# 3. Call a tool
POST /mcp
Authorization: Bearer <token>
Mcp-Session-Id: <session-id>

{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"github_list_repos","arguments":{"owner":"myorg"}}}

# 4. Terminate session
DELETE /mcp
Authorization: Bearer <token>
Mcp-Session-Id: <session-id>
```

### SSE Transport (legacy)

```bash
# 1. Connect SSE
GET /mcp
Authorization: Bearer <token>

# Server sends: event: endpoint
#               data: http://host/mcp?session_id=xxx

# 2. Send requests to the endpoint URL via POST
POST /mcp?session_id=xxx
Authorization: Bearer <token>

{"jsonrpc":"2.0","id":1,"method":"initialize",...}
```

---

## REST API Reference

All REST endpoints are under `/api/` and require JWT authentication (except register/login).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login |
| GET | `/api/auth/me` | Current user info |
| GET/POST/DELETE | `/api/auth/tokens` | Manage API tokens |
| GET/PUT | `/api/users` | User management (admin) |
| POST | `/api/users/{id}/recycle` | Recycle user's MCP sessions (admin) |
| POST | `/api/sessions/recycle` | Recycle own MCP sessions |
| GET/POST/PUT/DELETE | `/api/targets` | Target CRUD |
| GET/PUT/DELETE | `/api/targets/{id}/tokens/*` | Token management |
| GET/POST/PUT/DELETE | `/api/targets/{id}/env/*` | Environment config management |
| GET/POST/PUT/DELETE | `/api/policies` | Authorization policy CRUD |
| POST/DELETE | `/api/policies/{id}/subjects` | Policy subject management |
| GET | `/api/logs` | Request audit logs |
| GET | `/health` | Health check (no auth) |

---

## Database Migrations

Migrations run automatically at startup. Current migrations:

| # | File | Description |
|---|------|-------------|
| 001 | `001_initial_schema.sql` | Users, targets, sessions, request logs |
| 002 | `002_token_segregation.sql` | User/role/group token tables |
| 003 | `003_authorization_policies.sql` | Authorization policies + env configs |
| 004 | `004_transport_type.sql` | Transport type column on targets |
| 005 | `005_stdio_statefulness.sql` | STDIO fields, statefulness, isolation, mcp_instances |
| 006 | `006_simplify_statefulness.sql` | Simplify statefulness to 2 values (stateless, stateful) |
| 007 | `007_kubernetes_transport.sql` | Add image and port fields for Kubernetes transport |
