---
sidebar_position: 2
title: Architecture
---

# Architecture

## Overview

Reflow Gateway is an MCP multiplexing gateway that sits between AI clients (Claude, Cursor, etc.) and upstream MCP servers (GitHub, Jira, Slack, filesystem, etc.).

```
                          Reflow Gateway
                     +-----------------------+
  AI Clients         |                       |       Upstream MCP Servers
  (Claude,     ---->  |  POST /mcp (JSON-RPC) |  --->  GitHub (Streamable HTTP)
   Cursor,     ---->  |  GET  /mcp (SSE)      |  --->  Jira   (SSE)
   etc.)       ---->  |  DELETE /mcp           |  --->  Custom (STDIO process)
                     |                       |  --->  Internal (Kubernetes pod)
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
| Observability | OpenTelemetry + Grafana (Loki, Tempo, Prometheus) |

## Project Structure

```
backend/
  cmd/server/main.go          # Entry point
  internal/
    api/                       # REST API handlers and routes
    auth/                      # JWT, middleware, AES-GCM encryption
    config/                    # YAML config with env var expansion
    database/                  # PostgreSQL, migrations, repository
    gateway/                   # MCP handler, proxy, sessions, authorizer
    mcp/                       # MCP protocol client (HTTP + SSE)
    stdio/                     # STDIO process manager with GC
    k8s/                       # Kubernetes MCPInstance CR manager
    observability/             # Real-time dashboard hub (WebSocket)
    telemetry/                 # OpenTelemetry tracing and metrics
    docs/                      # Embedded OpenAPI spec + Scalar UI

operator/                      # Kubernetes operator (separate Go module)
  cmd/main.go                  # Operator entry point (controller-runtime)
  api/v1alpha1/                # MCPInstance CRD types
  internal/controller/         # Pod + Service reconciler

frontend/                      # Next.js dashboard
  src/app/                     # Pages (targets, policies, users, logs)
  src/lib/api.ts               # API client
  src/components/              # Shared UI components
```

## Request Flow

1. **Client** sends a JSON-RPC request to `POST /mcp` with a JWT token
2. **Auth Middleware** validates the JWT and extracts user identity (sub, role, groups)
3. **Session Manager** looks up or creates an MCP session
4. **Handler** parses the JSON-RPC method:
   - `initialize` -- creates session, connects to upstream targets
   - `tools/list` -- aggregates tools from all connected targets
   - `tools/call` -- routes to the correct upstream target
5. **Authorizer** checks policies (default-deny): is this user allowed to call this tool on this target?
6. **Proxy** resolves credentials (user > group > role > default) and injects them into the upstream request
7. **Upstream Client** sends the request using the appropriate transport (HTTP, SSE, STDIO, or K8s pod)
8. **Response** is returned to the client, and the request is logged for audit

## Key Design Decisions

- **Default-deny authorization**: no access without explicit policies
- **Credential isolation**: user JWT is never forwarded to upstream servers
- **Tool prefixing**: uses `_` delimiter when multiplexing multiple targets (e.g., `github_list_repos`)
- **Transport auto-detection**: tries Streamable HTTP POST first, falls back to SSE
- **Session recycle**: auto-detects JWT claim changes and refreshes sessions
- **AES-256-GCM encryption**: all sensitive values encrypted at rest in PostgreSQL

## Database

PostgreSQL stores:
- Users, roles, and groups
- Targets (upstream MCP server configurations)
- API tokens
- Authorization policies and subjects
- Target tokens (user, role, group, default -- all encrypted)
- Environment configurations (per-scope, encrypted)
- MCP sessions
- Request audit logs
- MCP instances (STDIO/K8s process tracking)

Migrations run automatically at startup (7 migration files, ordered).
