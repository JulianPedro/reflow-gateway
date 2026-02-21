---
sidebar_position: 6
title: Credential Management
---

# Credential Management

The gateway resolves credentials for upstream MCP servers based on the requesting user's identity. Credentials are **never** forwarded from the user's JWT.

## Resolution Priority

When a request reaches an upstream target, the gateway resolves credentials in this order:

1. **User-specific** credential (highest priority)
2. **Group-specific** credential
3. **Role-specific** credential
4. **Default** credential (lowest priority)

If no credential can be resolved and the target requires authentication, the request is denied.

## Two Credential Systems

### 1. Legacy Tokens (per-target)

Simple bearer/header tokens, configured via API:

```bash
# Default token (fallback for all users)
PUT /api/targets/{id}/tokens/default
{"token": "ghp_xxxxxxxxxxxx"}

# Role-specific token
PUT /api/targets/{id}/tokens/role
{"role": "developer", "token": "ghp_dev_token"}

# Group-specific token
PUT /api/targets/{id}/tokens/group
{"group_name": "engineering", "token": "ghp_eng_token"}

# User-specific token (set by the user themselves)
PUT /api/targets/{id}/token
{"token": "ghp_my_personal_token"}
```

**For HTTP/SSE targets**, the resolved token is injected as:
- `Authorization: Bearer <token>` (if `auth_type=bearer`)
- `<auth_header_name>: <token>` (if `auth_type=header`)

**For STDIO targets**, the resolved token is injected as:
- `AUTH_TOKEN=<token>` environment variable

### 2. Environment Configs (recommended)

Key-value configurations injected based on scope. This is the recommended approach, especially for STDIO and Kubernetes targets.

```bash
# Set a default env config
POST /api/targets/{id}/env/default
{"key": "GITHUB_TOKEN", "value": "ghp_xxxx", "description": "GitHub API token"}

# Set per-role
POST /api/targets/{id}/env/role/developer
{"key": "GITHUB_TOKEN", "value": "ghp_dev_token"}

# Set per-group
POST /api/targets/{id}/env/group/engineering
{"key": "GITHUB_TOKEN", "value": "ghp_eng_token"}

# Set per-user
POST /api/targets/{id}/env/user/{userId}
{"key": "GITHUB_TOKEN", "value": "ghp_personal_token"}
```

Resolution priority is the same: **user > group > role > default** (per key).

**For STDIO targets**, all resolved env configs are injected as environment variables when spawning the process.

**For Kubernetes targets**, env configs are injected as Kubernetes Secrets (`envFrom`) when creating pods.

**For HTTP/SSE targets**, reserved keys have special behavior:

| Key | Behavior |
|-----|----------|
| `AUTH_TOKEN` | Used as bearer/auth token |
| `AUTH_HEADER` | Override the auth header name |
| `BASE_URL` | Override the target URL |
| `TIMEOUT` | Set request timeout (e.g., `30s`, `2m`) |
| Other keys | Passed as `X-Env-<KEY>` custom headers |

## Encryption

All credential values are stored with **AES-256-GCM** encryption in PostgreSQL. The encryption key is configured via the `ENCRYPTION_KEY` environment variable (exactly 32 characters).

## Token Config Overview

Admins can view the complete token configuration for a target:

```bash
GET /api/targets/{id}/tokens
```

Returns which users, roles, groups, and default tokens are configured (without exposing the actual values).
