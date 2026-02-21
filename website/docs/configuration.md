---
sidebar_position: 3
title: Configuration
---

# Configuration Reference

The gateway is configured via a `config.yaml` file. All values support environment variable expansion with `${VAR_NAME}` syntax.

## Complete Reference

```yaml
server:
  port: 3000              # API server port
  host: 0.0.0.0           # Listen address

database:
  host: postgres          # PostgreSQL host
  port: 5432              # PostgreSQL port
  user: reflow            # Database user
  password: ${DB_PASSWORD} # Database password (from env)
  database: reflow_gateway # Database name
  sslmode: disable        # PostgreSQL SSL mode

jwt:
  secret: ${JWT_SECRET}   # JWT signing secret (from env)

cors:
  allowed_origins:        # Allowed CORS origins
    - "http://localhost:3001"
    - "http://localhost:3000"
  allowed_headers:
    - "*"
  expose_headers:         # Headers exposed to browser clients
    - "Mcp-Session-Id"
    - "MCP-Protocol-Version"

session:
  timeout: 30m            # Session expires after inactivity
  cleanup_interval: 5m    # Expired session cleanup frequency

encryption:
  key: ${ENCRYPTION_KEY}  # AES-256 key, exactly 32 characters (from env)

logging:
  level: info             # debug, info, warn, error
  format: json            # json or console

stdio:
  idle_ttl: 30m           # Kill idle STDIO processes after this duration
  max_lifetime: 24h       # Max process lifetime regardless of activity
  gc_interval: 1m         # How often to check for idle processes
  max_processes: 100      # Max concurrent STDIO processes

kubernetes:
  enabled: false          # Enable Kubernetes transport
  namespace: reflow       # Namespace for MCPInstance CRs
  kubeconfig: ""          # Path to kubeconfig (empty = in-cluster)
  idle_ttl: 30m           # Delete idle K8s instances after this duration
  max_lifetime: 24h       # Max instance lifetime
  gc_interval: 1m         # GC check interval
  max_instances: 100      # Max concurrent K8s instances

telemetry:
  enabled: false          # Enable OpenTelemetry
  endpoint: ""            # OTLP gRPC endpoint (e.g., "otel-collector:4317")
  service_name: "reflow-gateway"
  insecure: true          # Use insecure gRPC for OTLP
```

## Environment Variables

The following environment variables are referenced in the default `config.yaml`:

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_PASSWORD` | PostgreSQL password | `openssl rand -hex 16` |
| `JWT_SECRET` | JWT signing secret | `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | AES-256 encryption key (exactly 32 chars) | `openssl rand -base64 24 \| cut -c1-32` |

## Config File Location

By default the server looks for `config.yaml` in the current working directory. Override with the `-config` flag:

```bash
./reflow-gateway -config /etc/reflow/config.yaml
```

## Docker Compose

In Docker Compose, environment variables are loaded from `.env` and expanded at two levels:

1. **Docker Compose** expands `${VAR}` in `docker-compose.yml` (e.g., `DB_PASSWORD` in the postgres service)
2. **Go config loader** expands `${VAR}` in `config.yaml` at runtime (using the container's environment)

The `config.yaml` is mounted read-only into the container:

```yaml
volumes:
  - ./config.yaml:/app/config.yaml:ro
```

## Helm Chart

When deploying with the Helm chart, the config is rendered as a ConfigMap with `${VAR}` literals preserved. The Deployment injects actual values via `envFrom: secretRef`, and the Go config loader resolves them at runtime. See [Kubernetes Operator](./kubernetes-operator) for details.
