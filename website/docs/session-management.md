---
sidebar_position: 8
title: Session Management
---

# Session Management

MCP sessions track the connection state between a client and the gateway, including upstream connections, tool mappings, and user identity.

## Session Lifecycle

1. **Create**: client sends `initialize` via `POST /mcp`
2. **Active**: client sends requests using the `Mcp-Session-Id` header
3. **Expire**: session times out after inactivity (default: 30 minutes)
4. **Close**: client sends `DELETE /mcp` or session is recycled

## Session Recycle

When a user's role or groups change (e.g., via IdP like Okta/Azure AD, or admin action), existing MCP sessions become stale. The gateway handles this in two ways.

### Auto-Detection

On every MCP request, the gateway compares the JWT claims (role, groups) with the session's stored values. If they differ, the session is automatically recycled:

1. All HTTP upstream clients are closed
2. Session state (tool/resource/prompt mappings) is cleared
3. Identity context is updated to match new claims
4. Client receives an error asking it to re-initialize
5. On re-initialization, new STDIO/K8s processes are spawned with correct isolation keys

### Explicit Recycle API

For proactive recycling (e.g., IdP webhooks, admin actions):

```bash
# User: recycle own sessions
POST /api/sessions/recycle
Authorization: Bearer <token>

# Admin: recycle another user's sessions
POST /api/users/{id}/recycle
Authorization: Bearer <admin-token>
```

Response:

```json
{"recycled": 3, "user_id": "..."}
```

### When to Use

- **Auto-detection** handles the common case: user logs in with updated JWT after IdP changes
- **Explicit recycle** is for:
  - IdP webhooks that fire when group memberships change
  - Admin actions that update user roles/groups via the REST API
  - Forcing credential rotation without waiting for session expiry

## Configuration

```yaml
session:
  timeout: 30m            # Session expires after inactivity
  cleanup_interval: 5m    # How often to check for expired sessions
```
