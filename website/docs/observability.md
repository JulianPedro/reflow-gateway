---
sidebar_position: 10
title: Observability
---

# Observability

The gateway provides comprehensive observability through OpenTelemetry integration, Grafana dashboards, and a real-time WebSocket API.

## OpenTelemetry

The gateway exports traces and metrics via OTLP (OpenTelemetry Protocol):

```yaml
telemetry:
  enabled: true
  endpoint: "otel-collector:4317"  # OTLP gRPC endpoint
  service_name: "reflow-gateway"
  insecure: true
```

### Traces

Every MCP request generates a distributed trace with spans for:
- HTTP handler
- Authorization evaluation
- Credential resolution
- Upstream proxy request
- STDIO/K8s process operations

### Metrics

Key metrics exported:
- Request count and duration (by method, target, status)
- Active sessions
- Upstream response times
- STDIO/K8s instance counts

## Docker Compose Stack

The default `docker-compose.yml` includes a Grafana LGTM (Loki + Grafana + Tempo + Mimir) stack:

```yaml
otel-lgtm:
  image: grafana/otel-lgtm:latest
  ports:
    - "3002:3000"   # Grafana UI
    - "4317:4317"   # OTLP gRPC
    - "4318:4318"   # OTLP HTTP
```

Access Grafana at **http://localhost:3002** (default credentials: admin/admin).

### Pre-provisioned Dashboards

The `observability/grafana/` directory contains provisioned dashboards and data sources that are automatically loaded into Grafana.

## Real-Time Dashboard

The gateway exposes a WebSocket endpoint for real-time observability data:

```
GET /api/observability/ws
Authorization: Bearer <token>
```

This streams live updates including:
- Active request count
- Request rates and latencies
- Session activity
- Target health status

### Snapshot API

For point-in-time data without a WebSocket connection:

```bash
GET /api/observability/snapshot
Authorization: Bearer <token>
```

Returns a JSON object with aggregated observability metrics.

## Request Audit Logs

All MCP requests are logged to PostgreSQL:

```bash
GET /api/logs?limit=50&offset=0
Authorization: Bearer <token>
```

Each log entry includes:
- Session ID
- User ID
- MCP method called
- Target name
- Response status
- Duration (ms)
- Timestamp
