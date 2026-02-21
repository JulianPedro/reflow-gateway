<div align="center">

<img src="docs/banner.svg" alt="Reflow Gateway" width="100%" />

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](docker-compose.yml)

[**Docs**](https://reflowgateway.com/docs) · [**Quick Start**](#quick-start) · [**API Reference**](https://reflowgateway.com/docs/api-reference)

</div>

---

## What is Reflow Gateway?

Reflow Gateway sits between your AI clients (Claude, Cursor, Windsurf…) and your upstream MCP servers (GitHub, Jira, Slack, filesystem…). Instead of giving each client direct access to each server, the gateway centralizes:

- **Who** can access which tools (authorization policies)
- **Which credentials** to use for each upstream (per-user, per-group, per-role)
- **How** to route requests (Streamable HTTP, SSE, STDIO, Kubernetes pods)
- **What happened** (audit logs, OpenTelemetry traces, Grafana dashboards)

```
  AI Clients                    Reflow Gateway          MCP Servers
  ──────────                    ──────────────          ───────────

  Claude          ┐                           ┌─▶ GitHub  (HTTP)
  Cursor          ├──JWT Bearer──▶┌────────┐ ├─▶ Jira    (STDIO · per-user)
  Windsurf        ┘                │JWT Auth│ ├─▶ Slack   (STDIO · per-group)
  Any MCP client                   │Policies│ └─▶ Custom  (Kubernetes pod)
                                   │Cred Inj│
                                   └───┬────┘
                                       │
                                  PostgreSQL
                     (sessions · policies tokens · audit logs)
```

## Features

| | |
|---|---|
| 🔀 **MCP Multiplexing** | Aggregate tools from multiple servers into one endpoint |
| 🔐 **JWT Authentication** | Every request verified; API tokens for programmatic access |
| 🛡️ **Default-deny Authorization** | Fine-grained policies at target, tool, resource, and prompt level |
| 🔑 **Credential Injection** | Gateway resolves and injects upstream creds — clients never see them |
| 🚀 **Four Transports** | Streamable HTTP · SSE · STDIO processes · Kubernetes pods |
| 🔒 **Encryption at rest** | AES-256-GCM for all stored credentials |
| 📋 **Audit Logging** | Every MCP request logged with user, method, target, and duration |
| 📡 **OpenTelemetry** | Traces + metrics exported to any OTLP collector (Grafana included) |
| ♻️ **Session Recycle** | Auto-detects JWT claim changes and refreshes sessions mid-flight |
| 🐳 **Docker & Helm ready** | One command to run; Helm chart for Kubernetes deployments |

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/JulianPedro/reflow-gateway.git
cd gateway
cp .env.example .env
cp config.yaml.example config.yaml
```

Edit `.env` with secure secrets:

```bash
DB_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -base64 24 | cut -c1-32)
```

```bash
docker compose up -d
```

Gateway is running at **http://localhost:3000**. API docs at **http://localhost:3000/docs**.

### One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/reflow/gateway/main/install.sh | bash
```

### Helm (Kubernetes)

```bash
helm install reflow-gateway ./chart \
  --set secrets.jwtSecret="$(openssl rand -hex 32)" \
  --set secrets.encryptionKey="$(openssl rand -base64 24 | cut -c1-32)" \
  --set secrets.dbPassword="$(openssl rand -hex 16)" \
  --set config.database.host=my-postgres.default.svc
```

## Usage

### 1. Register and login

```bash
# First user gets admin role automatically
curl -X POST http://localhost:3000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"secure123"}'

TOKEN=$(curl -s -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"secure123"}' | jq -r .token)
```

### 2. Add an MCP server (STDIO, per-user isolation)

```bash
TARGET=$(curl -s -X POST http://localhost:3000/api/targets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "transport_type": "stdio",
    "command": "npx",
    "args": ["@modelcontextprotocol/server-github"],
    "statefulness": "stateful",
    "isolation_boundary": "per_user"
  }' | jq -r .id)

# Set the GitHub token for this user
curl -X POST http://localhost:3000/api/targets/$TARGET/env/user/$USER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"GITHUB_PERSONAL_ACCESS_TOKEN","value":"ghp_xxxx"}'
```

### 3. Create an authorization policy

```bash
curl -X POST http://localhost:3000/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Developers can use GitHub",
    "target_id": "'$TARGET'",
    "resource_type": "all",
    "effect": "allow",
    "priority": 10,
    "subjects": [{"subject_type": "role", "subject_value": "developer"}]
  }'
```

### 4. Connect with an MCP client

Configure Claude Desktop, Cursor, or any MCP client:

```json
{
  "mcpServers": {
    "reflow": {
      "url": "http://localhost:3000/mcp",
      "headers": {
        "Authorization": "Bearer <your-api-token>"
      }
    }
  }
}
```

Tools from all authorized targets are automatically available, prefixed by target name (`github_list_repos`, `jira_create_issue`, etc.).

## Architecture

```
backend/
  cmd/server/main.go              Entry point
  internal/
    api/                          REST API (handlers, routes)
    auth/                         JWT validation, AES-256-GCM encryption
    config/                       YAML config with ${ENV_VAR} expansion
    database/                     PostgreSQL + single consolidated migration
    gateway/                      MCP handler, proxy, session manager, authorizer
    mcp/                          HTTP/SSE MCP client (auto-detect transport)
    stdio/                        STDIO process pool with GC
    k8s/                          Kubernetes MCPInstance CR manager
    observability/                Real-time WebSocket dashboard
    telemetry/                    OpenTelemetry tracing and metrics
    docs/                         Embedded OpenAPI spec + Scalar UI

operator/                         Kubernetes operator (separate Go module)
  Reconciles MCPInstance CRDs → Pods + Services

website/                          Docusaurus documentation site
chart/                            Helm chart
```

## Configuration

```yaml
# config.yaml — all values support ${ENV_VAR} expansion
server:       { port: 3000, host: "0.0.0.0" }
database:     { host: postgres, port: 5432, user: reflow, password: ${DB_PASSWORD} }
jwt:          { secret: ${JWT_SECRET} }
encryption:   { key: ${ENCRYPTION_KEY} }   # exactly 32 chars
session:      { timeout: 30m }
logging:      { level: info, format: json }
kubernetes:   { enabled: false, namespace: reflow }
telemetry:    { enabled: false, endpoint: "otel-collector:4317" }
```

See [full configuration reference](https://reflowgateway.com/docs/configuration).

## Documentation

| | |
|---|---|
| [Getting Started](https://reflowgateway.com/docs) | Docker Compose setup in 5 minutes |
| [Architecture](https://reflowgateway.com/docs/architecture) | System design and request flow |
| [Authentication](https://reflowgateway.com/docs/authentication) | JWT, API tokens, user management |
| [Authorization](https://reflowgateway.com/docs/authorization) | Default-deny policies, evaluation order |
| [Credential Management](https://reflowgateway.com/docs/credential-management) | Per-user/group/role credential injection |
| [Transports](https://reflowgateway.com/docs/transports) | HTTP, SSE, STDIO, Kubernetes |
| [Session Management](https://reflowgateway.com/docs/session-management) | Recycle on identity changes |
| [Kubernetes Operator](https://reflowgateway.com/docs/kubernetes-operator) | CRDs, Helm, pod lifecycle |
| [Observability](https://reflowgateway.com/docs/observability) | OpenTelemetry, Grafana, audit logs |
| [API Reference](https://reflowgateway.com/docs/api-reference) | Interactive Scalar UI at `/docs` |

## Transports at a glance

| Transport | When to use | Isolation |
|-----------|-------------|-----------|
| `streamable-http` | Remote HTTP MCP servers | N/A (stateless proxy) |
| `sse` | Legacy SSE MCP servers | N/A |
| `stdio` | Local processes (npx, python…) | `shared` · `per_role` · `per_group` · `per_user` |
| `kubernetes` | Isolated pods in K8s clusters | same as stdio |

## Contributing

Contributions are welcome. Please open an issue before submitting a large PR.

```bash
# Backend
cd backend && go run cmd/server/main.go -config ../config.yaml

# Frontend
cd frontend && npm install && npm run dev

# Docs site
cd website && npm install && npm start
```

## License

MIT — see [LICENSE](LICENSE).

---

<div align="center">
Built with Go · PostgreSQL · MCP Streamable HTTP
</div>
