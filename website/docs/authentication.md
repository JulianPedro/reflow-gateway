---
sidebar_position: 4
title: Authentication
---

# Authentication

All MCP requests and protected API endpoints require a JWT token in the `Authorization: Bearer <token>` header.

## User Registration

```bash
POST /api/auth/register
Content-Type: application/json

{"email": "admin@example.com", "password": "secure123"}
```

The first user to register automatically receives the `admin` role. Subsequent users get the `user` role.

## Login

```bash
POST /api/auth/login
Content-Type: application/json

{"email": "admin@example.com", "password": "secure123"}
```

Returns a JWT token and user info:

```json
{
  "user": {
    "id": "...",
    "email": "admin@example.com",
    "role": "admin",
    "groups": []
  },
  "token": "eyJ..."
}
```

## JWT Claims

The gateway extracts the following claims from every JWT:

| Claim | Description |
|-------|-------------|
| `sub` | User ID (UUID) |
| `email` | User email |
| `role` | User role (`admin` or `user`) |
| `groups` | Array of group names |

These claims are used for authorization policy evaluation and credential resolution.

## API Tokens

For programmatic access (MCP clients like Claude, Cursor), create long-lived API tokens:

```bash
POST /api/auth/tokens
Authorization: Bearer <login-token>
Content-Type: application/json

{"name": "my-claude-token"}
```

Response:

```json
{
  "api_token": {
    "id": "...",
    "name": "my-claude-token",
    "created_at": "..."
  },
  "token": "eyJ..."
}
```

API tokens:
- Are tied to the creating user and inherit their role/groups
- Never expire (until revoked)
- Can be listed with `GET /api/auth/tokens`
- Can be revoked with `DELETE /api/auth/tokens/{id}`

## User Management

Admins can manage users via the REST API:

```bash
# List all users
GET /api/users

# Update a user's role and groups
PUT /api/users/{id}
{"role": "developer", "groups": ["engineering", "platform"]}
```

Role and group changes take effect on the next MCP request (via automatic session recycle). See [Session Management](./session-management) for details.
