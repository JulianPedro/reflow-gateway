---
sidebar_position: 9
title: Kubernetes Operator
---

# Kubernetes Operator

The Reflow Gateway includes a Kubernetes operator that manages MCP server instances as pods. When a target uses the `kubernetes` transport, the gateway creates `MCPInstance` custom resources, and the operator reconciles them into Pods + Services.

## Architecture

```
Gateway                         Kubernetes Cluster
+--------------+               +-------------------------------+
|  proxy.go    |  create CR    |  Operator (controller)        |
|  k8sManager  |-------------->|  MCPInstance Reconciler        |
|              |               |       | creates                |
|   HTTP req   |  HTTP (mcp)   |  Secret  Pod    Service        |
|              |-------------->|        (MCP)   (ClusterIP)     |
+--------------+               +-------------------------------+
```

## MCPInstance CRD

```yaml
apiVersion: mcp.reflow.io/v1alpha1
kind: MCPInstance
metadata:
  name: github-user-abc123
  namespace: reflow
spec:
  image: ghcr.io/org/mcp-github:latest
  port: 8080
  targetName: github
  subjectKey: "user:abc123"
  secretName: github-user-abc123
  healthPath: "/"
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

The operator creates:
- A **Pod** running the MCP server container
- A **ClusterIP Service** for HTTP access
- Links the pre-created **Secret** (environment variables) via `envFrom`
- Sets **owner references** for cascading cleanup

## Creating a Kubernetes Target

```bash
POST /api/targets
{
  "name": "github-k8s",
  "transport_type": "kubernetes",
  "image": "ghcr.io/org/mcp-github:latest",
  "port": 8080,
  "statefulness": "stateful",
  "isolation_boundary": "per_user"
}
```

## How It Works

1. User sends an MCP request to the gateway
2. Gateway computes a **subject key** from the target's isolation boundary + user identity
3. Gateway creates a Kubernetes Secret with resolved environment variables
4. Gateway creates an MCPInstance CR referencing the Secret
5. Operator reconciles: creates Pod + Service
6. Gateway polls until the Pod is Ready
7. Gateway connects to the Pod via HTTP (Streamable HTTP) through the ClusterIP Service
8. Subsequent requests reuse the cached HTTP client

## Pod Lifecycle

| Phase | Managed By |
|-------|-----------|
| Creation | Gateway (creates CR + Secret) |
| Reconciliation | Operator (creates Pod + Service) |
| Health checking | Kubernetes probes + operator |
| Garbage collection | Operator (idle TTL, max lifetime) |
| Deletion | Cascading via owner references |

## Configuration

### Gateway config

```yaml
kubernetes:
  enabled: true
  namespace: reflow       # Namespace for MCPInstance CRs
  kubeconfig: ""          # Empty = in-cluster config
  idle_ttl: 30m           # Delete idle instances
  max_lifetime: 24h       # Max instance lifetime
  gc_interval: 1m         # GC check interval
  max_instances: 100      # Max concurrent instances
```

### Installing the Operator

```bash
# Install CRD
kubectl apply -f operator/config/crd/bases/

# Install RBAC
kubectl apply -f operator/config/rbac/

# Deploy operator
kubectl apply -f operator/config/manager/
```

Or use the operator Helm chart:

```bash
helm install reflow-operator ./operator/chart \
  --namespace reflow-system \
  --create-namespace
```

## Restarting Instances

Admins can restart all running instances for a target:

```bash
POST /api/targets/{id}/restart-instances
Authorization: Bearer <admin-token>
```

This deletes all MCPInstance CRs for the target, causing the operator to terminate pods. New instances are created on the next MCP request.

## Security

- Credentials are stored in Kubernetes Secrets (encrypted at rest if etcd encryption is enabled)
- Network policies should restrict access to MCP pods (only gateway can reach them)
- The gateway ServiceAccount needs minimal RBAC: create/delete MCPInstances, Secrets, and Services in its namespace
