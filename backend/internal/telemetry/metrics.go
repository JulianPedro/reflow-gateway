package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Package-level metric instruments. When OTel is disabled these are no-op.
var (
	MCPRequestsTotal          metric.Int64Counter
	MCPRequestDuration        metric.Float64Histogram
	MCPToolCallsTotal         metric.Int64Counter
	MCPToolCallDuration       metric.Float64Histogram
	MCPAuthzDecisionsTotal    metric.Int64Counter
	MCPSessionsActive         metric.Int64UpDownCounter
	MCPUpstreamRequestsTotal  metric.Int64Counter
	MCPUpstreamRequestDuration metric.Float64Histogram
)

// InitMetrics registers all custom MCP metrics.
// Safe to call even when OTel is disabled (instruments become no-op).
func InitMetrics() {
	meter := otel.Meter("reflow-gateway")

	MCPRequestsTotal, _ = meter.Int64Counter("mcp.requests.total",
		metric.WithDescription("Total MCP requests received"),
	)
	MCPRequestDuration, _ = meter.Float64Histogram("mcp.request.duration",
		metric.WithDescription("MCP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	MCPToolCallsTotal, _ = meter.Int64Counter("mcp.tool.calls.total",
		metric.WithDescription("Total MCP tool calls"),
	)
	MCPToolCallDuration, _ = meter.Float64Histogram("mcp.tool.call.duration",
		metric.WithDescription("MCP tool call duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	MCPAuthzDecisionsTotal, _ = meter.Int64Counter("mcp.authz.decisions.total",
		metric.WithDescription("Total authorization decisions"),
	)
	MCPSessionsActive, _ = meter.Int64UpDownCounter("mcp.sessions.active",
		metric.WithDescription("Currently active MCP sessions"),
	)
	MCPUpstreamRequestsTotal, _ = meter.Int64Counter("mcp.upstream.requests.total",
		metric.WithDescription("Total upstream MCP requests"),
	)
	MCPUpstreamRequestDuration, _ = meter.Float64Histogram("mcp.upstream.request.duration",
		metric.WithDescription("Upstream MCP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
}
