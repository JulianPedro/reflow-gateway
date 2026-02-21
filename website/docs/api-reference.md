---
sidebar_position: 11
title: API Reference
---

# API Reference

The complete API reference is available as an interactive Scalar UI, served directly by the gateway.

## Interactive Documentation

When the gateway is running, visit:

**[http://localhost:3000/docs](http://localhost:3000/docs)**

This provides:
- Interactive request builder with authentication
- All request/response schemas
- Try-it-out functionality for every endpoint
- OpenAPI 3.0 spec download

## OpenAPI Spec

The raw OpenAPI 3.0 YAML spec is available at:

```
GET /docs/openapi.yaml
```

## Endpoint Summary

### Public (no auth)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login |

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Get current user |
| GET | `/api/auth/tokens` | List API tokens |
| POST | `/api/auth/tokens` | Create API token |
| DELETE | `/api/auth/tokens/{id}` | Revoke API token |

### Users (admin)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users` | List all users |
| PUT | `/api/users/{id}` | Update user |
| POST | `/api/users/{id}/recycle` | Recycle user's sessions |

### Sessions

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/sessions/recycle` | Recycle own sessions |

### Targets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/targets` | List targets |
| POST | `/api/targets` | Create target |
| GET | `/api/targets/{id}` | Get target |
| PUT | `/api/targets/{id}` | Update target |
| DELETE | `/api/targets/{id}` | Delete target |
| POST | `/api/targets/{id}/restart-instances` | Restart K8s instances |

### Target Tokens

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/targets/{id}/tokens` | View all token config |
| GET | `/api/targets/{id}/token` | Check own token |
| PUT | `/api/targets/{id}/token` | Set own token |
| DELETE | `/api/targets/{id}/token` | Remove own token |
| PUT | `/api/targets/{id}/tokens/role` | Set role token |
| DELETE | `/api/targets/{id}/tokens/role/{role}` | Delete role token |
| PUT | `/api/targets/{id}/tokens/group` | Set group token |
| DELETE | `/api/targets/{id}/tokens/group/{group}` | Delete group token |
| PUT | `/api/targets/{id}/tokens/default` | Set default token |
| DELETE | `/api/targets/{id}/tokens/default` | Delete default token |

### Authorization Policies

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/policies` | List policies |
| POST | `/api/policies` | Create policy |
| GET | `/api/policies/{id}` | Get policy |
| PUT | `/api/policies/{id}` | Update policy |
| DELETE | `/api/policies/{id}` | Delete policy |
| POST | `/api/policies/{id}/subjects` | Add subject |
| DELETE | `/api/policies/{id}/subjects/{subjectId}` | Remove subject |

### Environment Config

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/targets/{id}/env` | List all env configs |
| GET | `/api/targets/{id}/env/resolve` | Resolve for current user |
| GET/PUT/POST | `/api/targets/{id}/env/default` | Default scope configs |
| DELETE | `/api/targets/{id}/env/default/{key}` | Delete default config |
| GET/PUT/POST | `/api/targets/{id}/env/role/{scopeValue}` | Role scope configs |
| DELETE | `/api/targets/{id}/env/role/{scopeValue}/{key}` | Delete role config |
| GET/PUT/POST | `/api/targets/{id}/env/group/{scopeValue}` | Group scope configs |
| DELETE | `/api/targets/{id}/env/group/{scopeValue}/{key}` | Delete group config |
| GET/PUT/POST | `/api/targets/{id}/env/user/{scopeValue}` | User scope configs |
| DELETE | `/api/targets/{id}/env/user/{scopeValue}/{key}` | Delete user config |

### Logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/logs` | List request audit logs |

### Observability

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/observability/ws` | WebSocket real-time dashboard |
| GET | `/api/observability/snapshot` | Observability snapshot |

### MCP Protocol

| Method | Path | Description |
|--------|------|-------------|
| POST | `/mcp` | Send JSON-RPC request |
| GET | `/mcp` | Open SSE notification stream |
| DELETE | `/mcp` | Close MCP session |
