---
sidebar_position: 5
title: Authorization
---

# Authorization

The gateway operates on a **default-deny** model. Without explicit policies, no user can access any target, tool, resource, or prompt.

## Policy Structure

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
    {"subject_type": "role", "subject_value": "admin"}
  ]
}
```

### Fields

| Field | Values | Description |
|-------|--------|-------------|
| `target_id` | UUID or `null` | Specific target, or all targets if null |
| `resource_type` | `all`, `tool`, `resource`, `prompt` | Type of MCP resource |
| `resource_pattern` | regex or `null` | Pattern match on resource name |
| `effect` | `allow`, `deny` | Grant or deny access |
| `priority` | integer | Higher = evaluated first |
| `enabled` | boolean | Toggle without deleting |

### Subject Types

| Type | Description |
|------|-------------|
| `everyone` | All authenticated users |
| `role` | Users with a specific role |
| `group` | Users in a specific group |
| `user` | A specific user by ID |

## Evaluation Order

1. Policies are sorted by **priority** (descending)
2. **First matching policy wins**
3. If no policy matches, access is **denied**

This means deny rules with higher priority override allow rules with lower priority.

## Common Patterns

### Allow all users to access all targets

```bash
POST /api/policies
{
  "name": "Global allow",
  "resource_type": "all",
  "effect": "allow",
  "priority": 1,
  "subjects": [{"subject_type": "everyone"}]
}
```

### Allow a role to use specific tools

```bash
POST /api/policies
{
  "name": "Developers can use GitHub tools",
  "target_id": "<github-target-id>",
  "resource_type": "tool",
  "resource_pattern": ".*",
  "effect": "allow",
  "priority": 10,
  "subjects": [{"subject_type": "role", "subject_value": "developer"}]
}
```

### Block dangerous tools (with admin override)

```bash
# 1. Block delete tools for everyone (high priority)
POST /api/policies
{
  "name": "Block destructive tools",
  "resource_type": "tool",
  "resource_pattern": "delete_.*|remove_.*",
  "effect": "deny",
  "priority": 100,
  "subjects": [{"subject_type": "everyone"}]
}

# 2. Allow admins to use delete tools (highest priority)
POST /api/policies
{
  "name": "Admins can delete",
  "resource_type": "tool",
  "resource_pattern": "delete_.*|remove_.*",
  "effect": "allow",
  "priority": 200,
  "subjects": [{"subject_type": "role", "subject_value": "admin"}]
}
```

## Policy Management API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/policies` | List all policies |
| POST | `/api/policies` | Create policy |
| GET | `/api/policies/{id}` | Get policy details |
| PUT | `/api/policies/{id}` | Update policy |
| DELETE | `/api/policies/{id}` | Delete policy |
| POST | `/api/policies/{id}/subjects` | Add subject |
| DELETE | `/api/policies/{id}/subjects/{subjectId}` | Remove subject |

## Audit

All authorization decisions are logged in the request audit log (`GET /api/logs`), including the matched policy name and whether access was allowed or denied.
