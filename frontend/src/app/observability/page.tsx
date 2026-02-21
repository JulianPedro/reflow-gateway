"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ReactFlow,
  Background,
  Controls,
  Edge,
  MarkerType,
  Node,
  Position,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Activity,
  Clock,
  AlertTriangle,
  Users,
  RefreshCw,
  Wifi,
  WifiOff,
  Server,
  User,
  Zap,
} from "lucide-react";
import {
  observabilityApi,
  targetsApi,
  type MetricsSnapshot,
  type TargetStats,
  type ActivityEvent,
  type SessionEvent,
  type WSEvent,
  type Target,
} from "@/lib/api";

// --- WebSocket hook ---
function useObservabilityWS() {
  const [connected, setConnected] = useState(false);
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [activities, setActivities] = useState<ActivityEvent[]>([]);
  const [sessions, setSessions] = useState<SessionEvent[]>([]);
  const [activeTargets, setActiveTargets] = useState<Set<string>>(new Set());
  // Track active users: user_id -> display label (email or truncated id)
  const [activeUsers, setActiveUsers] = useState<Map<string, string>>(new Map());
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();
  const decayTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const markActive = useCallback((key: string, setter: React.Dispatch<React.SetStateAction<Set<string>>>) => {
    setter((prev) => {
      const next = new Set(prev);
      next.add(key);
      return next;
    });
    const existing = decayTimers.current.get(key);
    if (existing) clearTimeout(existing);
    decayTimers.current.set(
      key,
      setTimeout(() => {
        setter((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
        decayTimers.current.delete(key);
      }, 6000)
    );
  }, []);

  const markUserActive = useCallback((userId: string, userEmail?: string) => {
    const label = userEmail || userId.slice(0, 8) + "...";
    setActiveUsers((prev) => {
      const next = new Map(prev);
      next.set(userId, label);
      return next;
    });
    const timerKey = `user:${userId}`;
    const existing = decayTimers.current.get(timerKey);
    if (existing) clearTimeout(existing);
    decayTimers.current.set(
      timerKey,
      setTimeout(() => {
        setActiveUsers((prev) => {
          const next = new Map(prev);
          next.delete(userId);
          return next;
        });
        decayTimers.current.delete(timerKey);
      }, 10000)
    );
  }, []);

  const connect = useCallback(() => {
    const url = observabilityApi.wsUrl();
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => setConnected(true);
    ws.onclose = () => {
      setConnected(false);
      reconnectTimer.current = setTimeout(connect, 3000);
    };
    ws.onerror = () => ws.close();

    ws.onmessage = (evt) => {
      try {
        const event: WSEvent = JSON.parse(evt.data);
        switch (event.type) {
          case "metrics":
            setMetrics(event.data as MetricsSnapshot);
            break;
          case "activity": {
            const activity = event.data as ActivityEvent;
            setActivities((prev) => [activity, ...prev].slice(0, 100));
            if (activity.target) {
              markActive(activity.target, setActiveTargets);
            }
            markUserActive(activity.user_id, activity.user_email);
            break;
          }
          case "session":
            setSessions((prev) => [event.data as SessionEvent, ...prev].slice(0, 50));
            break;
        }
      } catch {
        // ignore parse errors
      }
    };
  }, [markActive, markUserActive]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
      decayTimers.current.forEach((t) => clearTimeout(t));
      decayTimers.current.clear();
    };
  }, [connect]);

  return { connected, metrics, activities, sessions, activeTargets, activeUsers };
}

// --- Metric card ---
function MetricCard({
  title,
  value,
  icon: Icon,
  color = "text-foreground",
}: {
  title: string;
  value: string | number;
  icon: React.ElementType;
  color?: string;
}) {
  return (
    <Card>
      <CardContent className="p-5">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-sm text-muted-foreground">{title}</p>
            <p className={`text-2xl font-bold mt-1 ${color}`}>{value}</p>
          </div>
          <Icon className="h-5 w-5 text-muted-foreground" />
        </div>
      </CardContent>
    </Card>
  );
}

// --- Traffic Graph ---
function TrafficGraph({
  targets,
  targetStats,
  activeTargets,
  activeUsers,
}: {
  targets: Target[];
  targetStats: TargetStats[];
  activeTargets: Set<string>;
  activeUsers: Map<string, string>;
}) {
  const statsMap = useMemo(() => {
    const m = new Map<string, TargetStats>();
    for (const s of targetStats) m.set(s.name, s);
    return m;
  }, [targetStats]);

  const enabledTargets = useMemo(() => targets.filter((t) => t.enabled), [targets]);

  const USER_GAP = 80;
  const TARGET_GAP = 130;

  const nodes: Node[] = useMemo(() => {
    const result: Node[] = [];
    const users = Array.from(activeUsers.entries());

    // Compute the tallest column to center the gateway vertically
    const usersHeight = users.length > 0 ? (users.length - 1) * USER_GAP : 0;
    const targetsHeight = enabledTargets.length > 0 ? (enabledTargets.length - 1) * TARGET_GAP : 0;
    const maxHeight = Math.max(usersHeight, targetsHeight);
    const centerY = maxHeight / 2;

    // User nodes (left side)
    if (users.length === 0) {
      result.push({
        id: "client-idle",
        position: { x: 0, y: centerY },
        data: { label: nodeLabel("No active clients", Users) },
        sourcePosition: Position.Right,
        style: nodeStyle("#6b7280"),
      });
    } else {
      const userYStart = centerY - usersHeight / 2;
      users.forEach(([uid, label], i) => {
        result.push({
          id: `user-${uid}`,
          position: { x: 0, y: userYStart + i * USER_GAP },
          data: { label: nodeLabel(label, User) },
          sourcePosition: Position.Right,
          style: nodeStyle("#3b82f6"),
        });
      });
    }

    // Gateway node (center)
    result.push({
      id: "gateway",
      position: { x: 320, y: centerY },
      data: { label: nodeLabel("Gateway", Zap) },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      style: nodeStyle("#8b5cf6"),
    });

    // Target nodes (right side)
    const targetYStart = centerY - targetsHeight / 2;
    enabledTargets.forEach((t, i) => {
      const stats = statsMap.get(t.name);
      const health = getHealthColor(stats);
      result.push({
        id: `target-${t.id}`,
        position: { x: 640, y: targetYStart + i * TARGET_GAP },
        data: {
          label: targetNodeLabel(t.name, t.transport_type, stats, health),
        },
        targetPosition: Position.Left,
        style: nodeStyle(health.bg),
      });
    });

    return result;
  }, [enabledTargets, statsMap, activeUsers]);

  const edges: Edge[] = useMemo(() => {
    const hasAnyActive = activeTargets.size > 0;
    const result: Edge[] = [];

    // User → Gateway edges
    const users = Array.from(activeUsers.keys());
    if (users.length === 0) {
      result.push({
        id: "idle-gw",
        source: "client-idle",
        target: "gateway",
        style: { stroke: "#6b7280", strokeWidth: 1.5 },
        markerEnd: { type: MarkerType.ArrowClosed, color: "#6b7280" },
      });
    } else {
      users.forEach((uid) => {
        result.push({
          id: `user-gw-${uid}`,
          source: `user-${uid}`,
          target: "gateway",
          animated: true,
          style: { stroke: "#a78bfa", strokeWidth: 2.5 },
          markerEnd: { type: MarkerType.ArrowClosed, color: "#a78bfa" },
        });
      });
    }

    // Gateway → Target edges
    enabledTargets.forEach((t) => {
      const stats = statsMap.get(t.name);
      const health = getHealthColor(stats);
      const isActive = activeTargets.has(t.name);
      result.push({
        id: `gw-${t.id}`,
        source: "gateway",
        target: `target-${t.id}`,
        animated: isActive || (stats?.rps ?? 0) > 0,
        style: {
          stroke: isActive ? "#22d3ee" : health.stroke,
          strokeWidth: isActive ? 2.5 : 1.5,
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isActive ? "#22d3ee" : health.stroke,
        },
        label: stats ? `${stats.rps.toFixed(1)} req/s` : "",
        labelStyle: { fontSize: 11, fill: "#888" },
      });
    });

    return result;
  }, [enabledTargets, statsMap, activeTargets, activeUsers]);

  return (
    <div className="h-[450px] w-full rounded-lg border border-border bg-card">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        zoomOnScroll={false}
        panOnScroll={false}
      >
        <Background />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}

function nodeStyle(borderColor: string) {
  return {
    border: `2px solid ${borderColor}`,
    borderRadius: 12,
    padding: 0,
    background: "var(--card)",
    color: "var(--card-foreground)",
    fontSize: 13,
    width: 180,
    overflow: "hidden" as const,
  };
}

function nodeLabel(label: string, Icon: React.ElementType) {
  return (
    <div className="flex items-center gap-2 px-4 py-3 overflow-hidden">
      <Icon className="h-4 w-4 flex-shrink-0" />
      <span className="font-medium truncate" title={label}>{label}</span>
    </div>
  );
}

const transportLabels: Record<string, string> = {
  "streamable-http": "Streamable HTTP",
  sse: "SSE",
  stdio: "STDIO",
  kubernetes: "Kubernetes",
};

function targetNodeLabel(
  name: string,
  transportType: string,
  stats: TargetStats | undefined,
  health: { text: string }
) {
  return (
    <div className="px-4 py-3 text-left overflow-hidden">
      <div className="flex items-center gap-2">
        <Server className="h-4 w-4 flex-shrink-0" />
        <span className="font-medium truncate" title={name}>{name}</span>
      </div>
      <span className="inline-block mt-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-muted text-muted-foreground">
        {transportLabels[transportType] || transportType}
      </span>
      {stats && (
        <div className="mt-1 text-xs text-muted-foreground space-y-0.5">
          <div>Latency: {stats.avg_latency_ms.toFixed(0)}ms</div>
          <div className={health.text}>
            Errors: {(stats.error_rate * 100).toFixed(1)}%
          </div>
        </div>
      )}
    </div>
  );
}

function getHealthColor(stats?: TargetStats) {
  if (!stats || stats.request_count === 0)
    return { bg: "#6b7280", stroke: "#6b7280", text: "text-muted-foreground" };
  if (stats.error_rate > 0.05)
    return { bg: "#ef4444", stroke: "#ef4444", text: "text-red-500" };
  if (stats.error_rate > 0.01)
    return { bg: "#f59e0b", stroke: "#f59e0b", text: "text-amber-500" };
  return { bg: "#22c55e", stroke: "#22c55e", text: "text-emerald-500" };
}

// --- Activity feed ---
function ActivityFeed({ activities }: { activities: ActivityEvent[] }) {
  const [paused, setPaused] = useState(false);

  return (
    <Card>
      <CardContent className="p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold">Live Activity</h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setPaused(!paused)}
            className="text-xs"
          >
            {paused ? "Resume" : "Pause"}
          </Button>
        </div>
        <div
          className="space-y-1.5 max-h-[300px] overflow-y-auto"
          onMouseEnter={() => setPaused(true)}
          onMouseLeave={() => setPaused(false)}
        >
          {activities.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">
              No activity yet
            </p>
          ) : (
            activities.map((a, i) => (
              <div
                key={i}
                className="flex items-center justify-between py-2 px-3 bg-muted/30 rounded-md text-sm"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <span
                    className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${
                      a.status === "ok" ? "bg-emerald-500" : "bg-red-500"
                    }`}
                  />
                  <span className="font-mono text-xs text-muted-foreground flex-shrink-0">
                    {new Date(a.timestamp).toLocaleTimeString()}
                  </span>
                  <span className="font-medium truncate">{a.method}</span>
                  {a.target && (
                    <span className="text-muted-foreground truncate">
                      → {a.target}
                    </span>
                  )}
                  {a.tool && (
                    <span className="text-cyan-500 truncate">{a.tool}</span>
                  )}
                </div>
                <span className="text-xs text-muted-foreground flex-shrink-0 ml-2">
                  {a.duration_ms.toFixed(0)}ms
                </span>
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// --- Session monitor ---
function SessionMonitor({ sessions }: { sessions: SessionEvent[] }) {
  return (
    <Card>
      <CardContent className="p-5">
        <h3 className="font-semibold mb-4">Session Events</h3>
        <div className="space-y-1.5 max-h-[300px] overflow-y-auto">
          {sessions.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">
              No session events yet
            </p>
          ) : (
            sessions.map((s, i) => (
              <div
                key={i}
                className="flex items-center justify-between py-2 px-3 bg-muted/30 rounded-md text-sm"
              >
                <div className="flex items-center gap-2">
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-medium ${
                      s.event === "created"
                        ? "bg-emerald-500/20 text-emerald-500"
                        : s.event === "deleted"
                        ? "bg-red-500/20 text-red-500"
                        : "bg-amber-500/20 text-amber-500"
                    }`}
                  >
                    {s.event}
                  </span>
                  <span className="font-mono text-xs text-muted-foreground truncate">
                    {s.session_id.slice(0, 8)}...
                  </span>
                </div>
                <span className="text-xs text-muted-foreground">
                  {s.user_id.slice(0, 8)}...
                </span>
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// --- Main page ---
export default function ObservabilityPage() {
  const { connected, metrics, activities, sessions, activeTargets, activeUsers } = useObservabilityWS();

  const { data: targets = [] } = useQuery({
    queryKey: ["targets"],
    queryFn: () => targetsApi.list(),
  });

  const {
    data: snapshot,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ["observability", "snapshot"],
    queryFn: () => observabilityApi.snapshot(),
    refetchInterval: 5000,
  });

  // Use WebSocket metrics if available, otherwise fall back to REST snapshot
  const currentMetrics = metrics || snapshot;

  return (
    <DashboardLayout
      title="Observability"
      description="Real-time gateway traffic and metrics"
      actions={
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5 text-sm">
            {connected ? (
              <>
                <Wifi className="h-4 w-4 text-emerald-500" />
                <span className="text-emerald-500">Live</span>
              </>
            ) : (
              <>
                <WifiOff className="h-4 w-4 text-muted-foreground" />
                <span className="text-muted-foreground">Disconnected</span>
              </>
            )}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
          >
            <RefreshCw
              className={`h-4 w-4 mr-2 ${isFetching ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Metric cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <MetricCard
            title="Total Requests (5m)"
            value={currentMetrics?.total_requests ?? 0}
            icon={Activity}
          />
          <MetricCard
            title="Avg Latency"
            value={`${(currentMetrics?.avg_latency_ms ?? 0).toFixed(0)}ms`}
            icon={Clock}
          />
          <MetricCard
            title="Error Rate"
            value={`${((currentMetrics?.error_rate ?? 0) * 100).toFixed(1)}%`}
            icon={AlertTriangle}
            color={
              (currentMetrics?.error_rate ?? 0) > 0.05
                ? "text-red-500"
                : (currentMetrics?.error_rate ?? 0) > 0.01
                ? "text-amber-500"
                : "text-foreground"
            }
          />
          <MetricCard
            title="Active Sessions"
            value={currentMetrics?.active_sessions ?? 0}
            icon={Users}
          />
        </div>

        {/* Traffic graph */}
        <TrafficGraph
          targets={targets}
          targetStats={currentMetrics?.targets ?? []}
          activeTargets={activeTargets}
          activeUsers={activeUsers}
        />

        {/* Activity feed + Session monitor */}
        <div className="grid gap-6 lg:grid-cols-2">
          <ActivityFeed activities={activities} />
          <SessionMonitor sessions={sessions} />
        </div>
      </div>
    </DashboardLayout>
  );
}
