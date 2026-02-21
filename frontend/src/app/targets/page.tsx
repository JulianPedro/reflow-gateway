"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { targetsApi, envConfigsApi, Target, CreateTargetRequest, UpdateTargetRequest, TargetEnvConfig, TransportType, Statefulness, IsolationBoundary } from "@/lib/api";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Plus,
  Trash2,
  Server,
  ExternalLink,
  MoreVertical,
  Loader2,
  Variable,
  Crown,
  Users,
  UserCircle,
  ChevronDown,
  ChevronRight,
  Shield,
  Pencil,
  Radio,
  Terminal,
  Cloud,
  RefreshCw,
} from "lucide-react";

function TargetCard({
  target,
  onToggle,
  onDelete,
  onEdit,
  onEnvConfig,
  onRestartInstances,
}: {
  target: Target;
  onToggle: (enabled: boolean) => void;
  onDelete: () => void;
  onEdit: () => void;
  onEnvConfig: () => void;
  onRestartInstances?: () => void;
}) {
  const [showMenu, setShowMenu] = useState(false);

  return (
    <Card className="relative">
      <CardContent className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className={`flex h-11 w-11 items-center justify-center rounded-lg ${target.enabled ? "bg-cyan-500 text-white" : "bg-muted text-muted-foreground"
              }`}>
              <Server className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <h3 className="font-semibold">{target.name}</h3>
              {target.transport_type === "stdio" ? (
                <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                  <Terminal className="h-3.5 w-3.5" />
                  {target.command} {target.args?.join(" ")}
                </p>
              ) : target.transport_type === "kubernetes" ? (
                <div className="space-y-0.5">
                  <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                    <Cloud className="h-3.5 w-3.5" />
                    {target.image}{target.port ? `:${target.port}` : ""}
                  </p>
                  {(target.command || (target.args && target.args.length > 0)) && (
                    <p className="flex items-center gap-1.5 text-xs text-muted-foreground/70 pl-5">
                      <Terminal className="h-3 w-3" />
                      {target.command} {target.args?.join(" ")}
                    </p>
                  )}
                </div>
              ) : (
                <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                  <ExternalLink className="h-3.5 w-3.5" />
                  {target.url}
                </p>
              )}
              <div className="flex flex-wrap gap-2 pt-1">
                <span className={`inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs ${
                  target.transport_type === "stdio" ? "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300" :
                  target.transport_type === "kubernetes" ? "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300" : "bg-muted"
                }`}>
                  {target.transport_type === "stdio" ? <Terminal className="h-3 w-3" /> :
                   target.transport_type === "kubernetes" ? <Cloud className="h-3 w-3" /> :
                   <Radio className="h-3 w-3" />}
                  {target.transport_type === "stdio" ? "STDIO" :
                   target.transport_type === "kubernetes" ? "K8s" :
                   target.transport_type === "sse" ? "SSE" : "HTTP"}
                </span>
                <span className="inline-flex items-center gap-1 rounded bg-muted px-2 py-0.5 text-xs">
                  <Shield className="h-3 w-3" />
                  {target.auth_type}
                </span>
                {target.statefulness && target.statefulness !== "stateless" && (
                  <span className="rounded bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 px-2 py-0.5 text-xs">
                    {target.statefulness.replace(/_/g, " ")}
                  </span>
                )}
                {target.isolation_boundary && target.isolation_boundary !== "shared" && (
                  <span className="rounded bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 px-2 py-0.5 text-xs">
                    {target.isolation_boundary.replace(/_/g, " ")}
                  </span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <span className={`text-sm ${target.enabled ? "text-emerald-600" : "text-muted-foreground"}`}>
                {target.enabled ? "Active" : "Disabled"}
              </span>
              <Switch checked={target.enabled} onCheckedChange={onToggle} />
            </div>

            <div className="relative">
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => setShowMenu(!showMenu)}
              >
                <MoreVertical className="h-4 w-4" />
              </Button>

              {showMenu && (
                <>
                  <div className="fixed inset-0 z-10" onClick={() => setShowMenu(false)} />
                  <div className="absolute right-0 top-full z-20 mt-1 w-48 rounded-lg border bg-popover p-1 shadow-lg">
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted"
                      onClick={() => { onEdit(); setShowMenu(false); }}
                    >
                      <Pencil className="h-4 w-4" />
                      Edit Target
                    </button>
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted"
                      onClick={() => { onEnvConfig(); setShowMenu(false); }}
                    >
                      <Variable className="h-4 w-4" />
                      Environment Config
                    </button>
                    {target.transport_type === "kubernetes" && onRestartInstances && (
                      <button
                        className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted"
                        onClick={() => { onRestartInstances(); setShowMenu(false); }}
                      >
                        <RefreshCw className="h-4 w-4" />
                        Restart Instances
                      </button>
                    )}
                    <hr className="my-1 border-border" />
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-950"
                      onClick={() => { onDelete(); setShowMenu(false); }}
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete Target
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function EnvConfigSection({
  title,
  icon: Icon,
  configs,
  onAdd,
  onDelete,
  isExpanded,
  onToggle,
}: {
  title: string;
  icon: React.ElementType;
  configs: TargetEnvConfig[];
  onAdd: () => void;
  onDelete: (key: string) => void;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="rounded-lg border">
      <button
        className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50"
        onClick={onToggle}
      >
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4" />
          {title}
          <span className="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs">
            {configs.length}
          </span>
        </div>
        {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
      </button>
      {isExpanded && (
        <div className="border-t p-4 space-y-2">
          {configs.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-3">
              No environment variables configured
            </p>
          ) : (
            configs.map((config) => (
              <div key={config.id} className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2">
                <div>
                  <p className="text-sm font-medium">{config.env_key}</p>
                  {config.description && (
                    <p className="text-xs text-muted-foreground">{config.description}</p>
                  )}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7 text-muted-foreground hover:text-red-600"
                  onClick={() => onDelete(config.env_key)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))
          )}
          <Button variant="outline" size="sm" className="w-full mt-2" onClick={onAdd}>
            <Plus className="h-4 w-4 mr-1" />
            Add Variable
          </Button>
        </div>
      )}
    </div>
  );
}

export default function TargetsPage() {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isEnvConfigOpen, setIsEnvConfigOpen] = useState(false);
  const [isAddEnvOpen, setIsAddEnvOpen] = useState(false);
  const [selectedTarget, setSelectedTarget] = useState<Target | null>(null);
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    default: true, role: false, group: false, user: false,
  });
  const [currentScope, setCurrentScope] = useState<{
    type: "default" | "role" | "group" | "user";
    value?: string;
  }>({ type: "default" });
  const [newEnvConfig, setNewEnvConfig] = useState({ key: "", value: "", description: "" });
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<UpdateTargetRequest & { id: string }>({
    id: "", name: "", url: "", transport_type: "streamable-http", command: "", args: [],
    image: "", port: 8080, health_path: "",
    statefulness: "stateless", isolation_boundary: "shared", auth_type: "none", auth_header_name: "",
  });
  const [newTarget, setNewTarget] = useState<CreateTargetRequest>({
    name: "", url: "", transport_type: "streamable-http", command: "", args: [],
    image: "", port: 8080, health_path: "",
    statefulness: "stateless", isolation_boundary: "shared", auth_type: "none", auth_header_name: "",
  });
  const [newTargetEnvVars, setNewTargetEnvVars] = useState<{
    scopeType: "default" | "role" | "group" | "user";
    scopeValue?: string;
    key: string;
    value: string;
    description: string;
  }[]>([]);
  const [createEnvExpanded, setCreateEnvExpanded] = useState<Record<string, boolean>>({
    default: true, role: false, group: false, user: false,
  });
  const [isCreateEnvAddOpen, setIsCreateEnvAddOpen] = useState(false);
  const [createEnvScope, setCreateEnvScope] = useState<{
    type: "default" | "role" | "group" | "user";
    value?: string;
  }>({ type: "default" });
  const [createEnvForm, setCreateEnvForm] = useState({ key: "", value: "", description: "", scopeValue: "" });

  const { data: targets = [], isLoading } = useQuery({
    queryKey: ["targets"],
    queryFn: targetsApi.list,
  });

  const { data: envConfigs = [], refetch: refetchEnvConfigs } = useQuery({
    queryKey: ["envConfigs", selectedTarget?.id],
    queryFn: () => (selectedTarget ? envConfigsApi.listAll(selectedTarget.id) : Promise.resolve([])),
    enabled: !!selectedTarget && isEnvConfigOpen,
  });

  const createMutation = useMutation({
    mutationFn: async (data: CreateTargetRequest) => {
      const target = await targetsApi.create(data);
      // Save env configs for all scopes
      for (const envVar of newTargetEnvVars) {
        if (envVar.key && envVar.value) {
          const desc = envVar.description || undefined;
          switch (envVar.scopeType) {
            case "default":
              await envConfigsApi.setDefault(target.id, envVar.key, envVar.value, desc);
              break;
            case "role":
              if (envVar.scopeValue) await envConfigsApi.setRole(target.id, envVar.scopeValue, envVar.key, envVar.value, desc);
              break;
            case "group":
              if (envVar.scopeValue) await envConfigsApi.setGroup(target.id, envVar.scopeValue, envVar.key, envVar.value, desc);
              break;
            case "user":
              if (envVar.scopeValue) await envConfigsApi.setUser(target.id, envVar.scopeValue, envVar.key, envVar.value, desc);
              break;
          }
        }
      }
      return target;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["targets"] });
      setIsCreateOpen(false);
      setNewTarget({ name: "", url: "", transport_type: "streamable-http", command: "", args: [], image: "", port: 8080, health_path: "", statefulness: "stateless", isolation_boundary: "shared", auth_type: "none", auth_header_name: "" });
      setNewTargetEnvVars([]);
      setCreateEnvExpanded({ default: true, role: false, group: false, user: false });
    },
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => targetsApi.update(id, { enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["targets"] }),
  });

  const editMutation = useMutation({
    mutationFn: ({ id, ...data }: UpdateTargetRequest & { id: string }) => targetsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["targets"] });
      setIsEditOpen(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => targetsApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["targets"] }),
  });

  const restartInstancesMutation = useMutation({
    mutationFn: (id: string) => targetsApi.restartInstances(id),
  });

  const setEnvConfigMutation = useMutation({
    mutationFn: async ({ targetId, scope, key, value, description }: {
      targetId: string;
      scope: { type: "default" | "role" | "group" | "user"; value?: string };
      key: string; value: string; description?: string;
    }) => {
      switch (scope.type) {
        case "default": return envConfigsApi.setDefault(targetId, key, value, description);
        case "role": return envConfigsApi.setRole(targetId, scope.value!, key, value, description);
        case "group": return envConfigsApi.setGroup(targetId, scope.value!, key, value, description);
        case "user": return envConfigsApi.setUser(targetId, scope.value!, key, value, description);
      }
    },
    onSuccess: () => {
      refetchEnvConfigs();
      setIsAddEnvOpen(false);
      setNewEnvConfig({ key: "", value: "", description: "" });
    },
  });

  const deleteEnvConfigMutation = useMutation({
    mutationFn: async ({ targetId, scope, key }: {
      targetId: string;
      scope: { type: "default" | "role" | "group" | "user"; value?: string };
      key: string;
    }) => {
      switch (scope.type) {
        case "default": return envConfigsApi.deleteDefault(targetId, key);
        case "role": return envConfigsApi.deleteRole(targetId, scope.value!, key);
        case "group": return envConfigsApi.deleteGroup(targetId, scope.value!, key);
        case "user": return envConfigsApi.deleteUser(targetId, scope.value!, key);
      }
    },
    onSuccess: () => refetchEnvConfigs(),
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate(newTarget);
  };

  const handleEdit = (e: React.FormEvent) => {
    e.preventDefault();
    editMutation.mutate(editTarget);
  };

  const openEditDialog = (target: Target) => {
    setEditTarget({
      id: target.id,
      name: target.name,
      url: target.url,
      transport_type: target.transport_type,
      command: target.command || "",
      args: target.args || [],
      image: target.image || "",
      port: target.port || 8080,
      health_path: target.health_path || "",
      statefulness: target.statefulness || "stateless",
      isolation_boundary: target.isolation_boundary || "shared",
      auth_type: target.auth_type,
      auth_header_name: target.auth_header_name || "",
    });
    setIsEditOpen(true);
  };

  const handleAddEnvConfig = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedTarget) {
      setEnvConfigMutation.mutate({
        targetId: selectedTarget.id,
        scope: currentScope,
        key: newEnvConfig.key,
        value: newEnvConfig.value,
        description: newEnvConfig.description || undefined,
      });
    }
  };

  const groupedConfigs = {
    default: envConfigs.filter((c) => c.scope_type === "default"),
    role: envConfigs.filter((c) => c.scope_type === "role"),
    group: envConfigs.filter((c) => c.scope_type === "group"),
    user: envConfigs.filter((c) => c.scope_type === "user"),
  };

  return (
    <DashboardLayout
      title="MCP Servers"
      description="Configure upstream MCP servers"
      actions={
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              Add MCP Server
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg max-h-[85vh] overflow-y-auto">
            <form onSubmit={handleCreate}>
              <DialogHeader>
                <DialogTitle>Add MCP Server</DialogTitle>
                <DialogDescription>Connect to an upstream MCP server</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Name</Label>
                  <Input
                    id="name"
                    placeholder="e.g., github-mcp"
                    value={newTarget.name}
                    onChange={(e) => setNewTarget({ ...newTarget, name: e.target.value })}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="transport_type">Transport</Label>
                  <select
                    id="transport_type"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={newTarget.transport_type}
                    onChange={(e) => setNewTarget({ ...newTarget, transport_type: e.target.value as TransportType })}
                  >
                    <option value="streamable-http">Streamable HTTP (auto-detects SSE)</option>
                    <option value="sse">SSE (legacy)</option>
                    <option value="stdio">STDIO (local process)</option>
                    <option value="kubernetes">Kubernetes (CRD-managed)</option>
                  </select>
                </div>
                {newTarget.transport_type === "stdio" ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="command">Command</Label>
                      <Input
                        id="command"
                        placeholder="npx"
                        value={newTarget.command || ""}
                        onChange={(e) => setNewTarget({ ...newTarget, command: e.target.value })}
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="args">Arguments (comma-separated)</Label>
                      <Input
                        id="args"
                        placeholder="@modelcontextprotocol/server-github"
                        value={(newTarget.args || []).join(", ")}
                        onChange={(e) => setNewTarget({ ...newTarget, args: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })}
                      />
                    </div>
                  </>
                ) : newTarget.transport_type === "kubernetes" ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="image">Container Image</Label>
                      <Input
                        id="image"
                        placeholder="ghcr.io/org/mcp-server:latest"
                        value={newTarget.image || ""}
                        onChange={(e) => setNewTarget({ ...newTarget, image: e.target.value })}
                        required
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label htmlFor="port">Port (default: 8080)</Label>
                        <Input
                          id="port"
                          type="number"
                          placeholder="8080"
                          value={newTarget.port || ""}
                          onChange={(e) => setNewTarget({ ...newTarget, port: e.target.value ? parseInt(e.target.value) : undefined })}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="k8s_command">Command (optional)</Label>
                        <Input
                          id="k8s_command"
                          placeholder="e.g., node"
                          value={newTarget.command || ""}
                          onChange={(e) => setNewTarget({ ...newTarget, command: e.target.value })}
                        />
                      </div>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="k8s_args">Arguments (comma-separated)</Label>
                      <Input
                        id="k8s_args"
                        placeholder="e.g., --transport, streamable-http, --port, 8080"
                        value={(newTarget.args || []).join(", ")}
                        onChange={(e) => setNewTarget({ ...newTarget, args: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })}
                      />
                      <p className="text-xs text-muted-foreground">
                        Override container entrypoint and arguments. Useful for images that default to STDIO mode.
                      </p>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="health_path">Health Check Path (optional)</Label>
                      <Input
                        id="health_path"
                        placeholder="e.g., /health, /mcp"
                        value={newTarget.health_path || ""}
                        onChange={(e) => setNewTarget({ ...newTarget, health_path: e.target.value })}
                      />
                      <p className="text-xs text-muted-foreground">
                        HTTP path for readiness probes. Leave empty to skip health checks.
                      </p>
                    </div>
                  </>
                ) : (
                  <div className="space-y-2">
                    <Label htmlFor="url">URL</Label>
                    <Input
                      id="url"
                      placeholder="http://localhost:3001/mcp"
                      value={newTarget.url || ""}
                      onChange={(e) => setNewTarget({ ...newTarget, url: e.target.value })}
                      required
                    />
                  </div>
                )}
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <Label htmlFor="statefulness">Statefulness</Label>
                    <select
                      id="statefulness"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={newTarget.statefulness || "stateless"}
                      onChange={(e) => setNewTarget({ ...newTarget, statefulness: e.target.value as Statefulness })}
                    >
                      <option value="stateless">Stateless</option>
                      <option value="stateful">Stateful</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="isolation_boundary">Isolation</Label>
                    <select
                      id="isolation_boundary"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={newTarget.isolation_boundary || "shared"}
                      onChange={(e) => setNewTarget({ ...newTarget, isolation_boundary: e.target.value as IsolationBoundary })}
                    >
                      <option value="shared">Shared</option>
                      <option value="per_role">Per Role</option>
                      <option value="per_group">Per Group</option>
                      <option value="per_user">Per User</option>
                    </select>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="auth_type">Authentication</Label>
                  <select
                    id="auth_type"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={newTarget.auth_type}
                    onChange={(e) => setNewTarget({ ...newTarget, auth_type: e.target.value })}
                  >
                    <option value="none">None</option>
                    <option value="bearer">Bearer Token</option>
                    <option value="header">Custom Header</option>
                  </select>
                </div>
                {newTarget.auth_type === "header" && (
                  <div className="space-y-2">
                    <Label htmlFor="auth_header_name">Header Name</Label>
                    <Input
                      id="auth_header_name"
                      placeholder="X-API-Key"
                      value={newTarget.auth_header_name}
                      onChange={(e) => setNewTarget({ ...newTarget, auth_header_name: e.target.value })}
                    />
                  </div>
                )}

                {/* Environment Variables Section */}
                <div className="border-t border-border pt-4 space-y-3">
                  <Label className="flex items-center gap-2">
                    <Variable className="h-4 w-4" />
                    Environment Variables
                  </Label>
                  <div className="rounded-lg bg-muted/50 p-3 text-xs">
                    <strong>Priority:</strong> User &gt; Group &gt; Role &gt; Default.{" "}
                    <strong>Reserved:</strong>{" "}
                    <code className="bg-muted px-1 rounded">AUTH_TOKEN</code>,{" "}
                    <code className="bg-muted px-1 rounded">AUTH_HEADER</code>,{" "}
                    <code className="bg-muted px-1 rounded">BASE_URL</code>,{" "}
                    <code className="bg-muted px-1 rounded">TIMEOUT</code>
                  </div>

                  {/* Default */}
                  {(["default", "role", "group", "user"] as const).map((scope) => {
                    const scopeConfigs = newTargetEnvVars.filter((v) => v.scopeType === scope);
                    const ScopeIcon = scope === "default" ? Shield : scope === "role" ? Crown : scope === "group" ? Users : UserCircle;
                    const scopeTitle = scope === "default" ? "Default" : scope === "role" ? "By Role" : scope === "group" ? "By Group" : "By User";
                    return (
                      <div key={scope} className="rounded-lg border">
                        <button
                          type="button"
                          className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50"
                          onClick={() => setCreateEnvExpanded((s) => ({ ...s, [scope]: !s[scope] }))}
                        >
                          <div className="flex items-center gap-2">
                            <ScopeIcon className="h-4 w-4" />
                            {scopeTitle}
                            <span className="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs">
                              {scopeConfigs.length}
                            </span>
                          </div>
                          {createEnvExpanded[scope] ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                        </button>
                        {createEnvExpanded[scope] && (
                          <div className="border-t p-4 space-y-2">
                            {scopeConfigs.length === 0 ? (
                              <p className="text-sm text-muted-foreground text-center py-3">
                                No environment variables configured
                              </p>
                            ) : (
                              scopeConfigs.map((envVar, idx) => {
                                const globalIdx = newTargetEnvVars.indexOf(envVar);
                                return (
                                  <div key={idx} className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2">
                                    <div>
                                      <p className="text-sm font-medium">
                                        {envVar.key}
                                        {envVar.scopeValue && (
                                          <span className="ml-2 text-xs text-muted-foreground">({envVar.scopeValue})</span>
                                        )}
                                      </p>
                                      {envVar.description && (
                                        <p className="text-xs text-muted-foreground">{envVar.description}</p>
                                      )}
                                    </div>
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="icon"
                                      className="h-7 w-7 text-muted-foreground hover:text-red-600"
                                      onClick={() => setNewTargetEnvVars(newTargetEnvVars.filter((_, i) => i !== globalIdx))}
                                    >
                                      <Trash2 className="h-3.5 w-3.5" />
                                    </Button>
                                  </div>
                                );
                              })
                            )}
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="w-full mt-2"
                              onClick={() => {
                                setCreateEnvScope({ type: scope, value: scope === "default" ? undefined : "" });
                                setCreateEnvForm({ key: "", value: "", description: "", scopeValue: "" });
                                setIsCreateEnvAddOpen(true);
                              }}
                            >
                              <Plus className="h-4 w-4 mr-1" />
                              Add Variable
                            </Button>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setIsCreateOpen(false)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      }
    >
      {isLoading ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <Card key={i} className="h-32 animate-pulse bg-muted/50" />
          ))}
        </div>
      ) : targets.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <Server className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No targets configured</h3>
            <p className="text-muted-foreground mb-6">
              Add your first MCP server to start routing requests
            </p>
            <Button onClick={() => setIsCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Target
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {targets.map((target: Target) => (
            <TargetCard
              key={target.id}
              target={target}
              onToggle={(enabled) => toggleMutation.mutate({ id: target.id, enabled })}
              onDelete={() => deleteMutation.mutate(target.id)}
              onEdit={() => openEditDialog(target)}
              onEnvConfig={() => { setSelectedTarget(target); setIsEnvConfigOpen(true); }}
              onRestartInstances={() => restartInstancesMutation.mutate(target.id)}
            />
          ))}
        </div>
      )}

      {/* Edit Target Dialog */}
      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent>
          <form onSubmit={handleEdit}>
            <DialogHeader>
              <DialogTitle>Edit Target</DialogTitle>
              <DialogDescription>Update MCP server configuration</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="edit_name">Name</Label>
                <Input
                  id="edit_name"
                  value={editTarget.name}
                  onChange={(e) => setEditTarget({ ...editTarget, name: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit_transport_type">Transport</Label>
                <select
                  id="edit_transport_type"
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={editTarget.transport_type}
                  onChange={(e) => setEditTarget({ ...editTarget, transport_type: e.target.value as TransportType })}
                >
                  <option value="streamable-http">Streamable HTTP (auto-detects SSE)</option>
                  <option value="sse">SSE (legacy)</option>
                  <option value="stdio">STDIO (local process)</option>
                  <option value="kubernetes">Kubernetes (CRD-managed)</option>
                </select>
              </div>
              {editTarget.transport_type === "stdio" ? (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="edit_command">Command</Label>
                    <Input
                      id="edit_command"
                      placeholder="npx"
                      value={editTarget.command || ""}
                      onChange={(e) => setEditTarget({ ...editTarget, command: e.target.value })}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit_args">Arguments (comma-separated)</Label>
                    <Input
                      id="edit_args"
                      placeholder="@modelcontextprotocol/server-github"
                      value={(editTarget.args || []).join(", ")}
                      onChange={(e) => setEditTarget({ ...editTarget, args: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })}
                    />
                  </div>
                </>
              ) : editTarget.transport_type === "kubernetes" ? (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="edit_image">Container Image</Label>
                    <Input
                      id="edit_image"
                      placeholder="ghcr.io/org/mcp-server:latest"
                      value={editTarget.image || ""}
                      onChange={(e) => setEditTarget({ ...editTarget, image: e.target.value })}
                      required
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-2">
                      <Label htmlFor="edit_port">Port (default: 8080)</Label>
                      <Input
                        id="edit_port"
                        type="number"
                        placeholder="8080"
                        value={editTarget.port || ""}
                        onChange={(e) => setEditTarget({ ...editTarget, port: e.target.value ? parseInt(e.target.value) : undefined })}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="edit_k8s_command">Command (optional)</Label>
                      <Input
                        id="edit_k8s_command"
                        placeholder="e.g., node"
                        value={editTarget.command || ""}
                        onChange={(e) => setEditTarget({ ...editTarget, command: e.target.value })}
                      />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit_k8s_args">Arguments (comma-separated)</Label>
                    <Input
                      id="edit_k8s_args"
                      placeholder="e.g., --transport, streamable-http, --port, 8080"
                      value={(editTarget.args || []).join(", ")}
                      onChange={(e) => setEditTarget({ ...editTarget, args: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })}
                    />
                    <p className="text-xs text-muted-foreground">
                      Override container entrypoint and arguments. Useful for images that default to STDIO mode.
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit_health_path">Health Check Path (optional)</Label>
                    <Input
                      id="edit_health_path"
                      placeholder="e.g., /health, /mcp"
                      value={editTarget.health_path || ""}
                      onChange={(e) => setEditTarget({ ...editTarget, health_path: e.target.value })}
                    />
                    <p className="text-xs text-muted-foreground">
                      HTTP path for readiness probes. Leave empty to skip health checks.
                    </p>
                  </div>
                </>
              ) : (
                <div className="space-y-2">
                  <Label htmlFor="edit_url">URL</Label>
                  <Input
                    id="edit_url"
                    value={editTarget.url}
                    onChange={(e) => setEditTarget({ ...editTarget, url: e.target.value })}
                    required
                  />
                </div>
              )}
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label htmlFor="edit_statefulness">Statefulness</Label>
                  <select
                    id="edit_statefulness"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={editTarget.statefulness || "stateless"}
                    onChange={(e) => setEditTarget({ ...editTarget, statefulness: e.target.value as Statefulness })}
                  >
                    <option value="stateless">Stateless</option>
                    <option value="stateful">Stateful</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit_isolation_boundary">Isolation</Label>
                  <select
                    id="edit_isolation_boundary"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={editTarget.isolation_boundary || "shared"}
                    onChange={(e) => setEditTarget({ ...editTarget, isolation_boundary: e.target.value as IsolationBoundary })}
                  >
                    <option value="shared">Shared</option>
                    <option value="per_role">Per Role</option>
                    <option value="per_group">Per Group</option>
                    <option value="per_user">Per User</option>
                  </select>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit_auth_type">Authentication</Label>
                <select
                  id="edit_auth_type"
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={editTarget.auth_type}
                  onChange={(e) => setEditTarget({ ...editTarget, auth_type: e.target.value })}
                >
                  <option value="none">None</option>
                  <option value="bearer">Bearer Token</option>
                  <option value="header">Custom Header</option>
                </select>
              </div>
              {editTarget.auth_type === "header" && (
                <div className="space-y-2">
                  <Label htmlFor="edit_auth_header_name">Header Name</Label>
                  <Input
                    id="edit_auth_header_name"
                    placeholder="X-API-Key"
                    value={editTarget.auth_header_name}
                    onChange={(e) => setEditTarget({ ...editTarget, auth_header_name: e.target.value })}
                  />
                </div>
              )}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsEditOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={editMutation.isPending}>
                {editMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Save
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Environment Config Dialog */}
      <Dialog open={isEnvConfigOpen} onOpenChange={setIsEnvConfigOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Environment Configuration</DialogTitle>
            <DialogDescription>
              Configure environment variables for {selectedTarget?.name}. Priority: User &gt; Group &gt; Role &gt; Default
            </DialogDescription>
          </DialogHeader>
          <div className="py-4 space-y-4">
            <div className="rounded-lg bg-muted/50 p-3 text-sm">
              <strong>Reserved Keys:</strong>{" "}
              <code className="bg-muted px-1 rounded">AUTH_TOKEN</code>,{" "}
              <code className="bg-muted px-1 rounded">AUTH_HEADER</code>,{" "}
              <code className="bg-muted px-1 rounded">BASE_URL</code>,{" "}
              <code className="bg-muted px-1 rounded">TIMEOUT</code>
            </div>

            <EnvConfigSection
              title="Default"
              icon={Shield}
              configs={groupedConfigs.default}
              isExpanded={expandedSections.default}
              onToggle={() => setExpandedSections((s) => ({ ...s, default: !s.default }))}
              onAdd={() => { setCurrentScope({ type: "default" }); setIsAddEnvOpen(true); }}
              onDelete={(key) => selectedTarget && deleteEnvConfigMutation.mutate({
                targetId: selectedTarget.id, scope: { type: "default" }, key,
              })}
            />

            <EnvConfigSection
              title="By Role"
              icon={Crown}
              configs={groupedConfigs.role}
              isExpanded={expandedSections.role}
              onToggle={() => setExpandedSections((s) => ({ ...s, role: !s.role }))}
              onAdd={() => { setCurrentScope({ type: "role", value: "" }); setIsAddEnvOpen(true); }}
              onDelete={(key) => {
                const config = groupedConfigs.role.find((c) => c.env_key === key);
                if (selectedTarget && config?.scope_value) {
                  deleteEnvConfigMutation.mutate({
                    targetId: selectedTarget.id, scope: { type: "role", value: config.scope_value }, key,
                  });
                }
              }}
            />

            <EnvConfigSection
              title="By Group"
              icon={Users}
              configs={groupedConfigs.group}
              isExpanded={expandedSections.group}
              onToggle={() => setExpandedSections((s) => ({ ...s, group: !s.group }))}
              onAdd={() => { setCurrentScope({ type: "group", value: "" }); setIsAddEnvOpen(true); }}
              onDelete={(key) => {
                const config = groupedConfigs.group.find((c) => c.env_key === key);
                if (selectedTarget && config?.scope_value) {
                  deleteEnvConfigMutation.mutate({
                    targetId: selectedTarget.id, scope: { type: "group", value: config.scope_value }, key,
                  });
                }
              }}
            />

            <EnvConfigSection
              title="By User"
              icon={UserCircle}
              configs={groupedConfigs.user}
              isExpanded={expandedSections.user}
              onToggle={() => setExpandedSections((s) => ({ ...s, user: !s.user }))}
              onAdd={() => { setCurrentScope({ type: "user", value: "" }); setIsAddEnvOpen(true); }}
              onDelete={(key) => {
                const config = groupedConfigs.user.find((c) => c.env_key === key);
                if (selectedTarget && config?.scope_value) {
                  deleteEnvConfigMutation.mutate({
                    targetId: selectedTarget.id, scope: { type: "user", value: config.scope_value }, key,
                  });
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsEnvConfigOpen(false)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add Env Config Dialog */}
      <Dialog open={isAddEnvOpen} onOpenChange={setIsAddEnvOpen}>
        <DialogContent>
          <form onSubmit={handleAddEnvConfig}>
            <DialogHeader>
              <DialogTitle>Add Environment Variable</DialogTitle>
              <DialogDescription>
                Scope: {currentScope.type === "default" ? "Default" : currentScope.type}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              {currentScope.type !== "default" && (
                <div className="space-y-2">
                  <Label htmlFor="scope_value">
                    {currentScope.type === "role" ? "Role Name" : currentScope.type === "group" ? "Group Name" : "User ID"}
                  </Label>
                  <Input
                    id="scope_value"
                    placeholder={currentScope.type === "role" ? "e.g., admin" : currentScope.type === "group" ? "e.g., developers" : "User UUID"}
                    value={currentScope.value || ""}
                    onChange={(e) => setCurrentScope({ ...currentScope, value: e.target.value })}
                    required
                  />
                </div>
              )}
              <div className="space-y-2">
                <Label htmlFor="env_key">Key</Label>
                <Input
                  id="env_key"
                  placeholder="e.g., AUTH_TOKEN"
                  value={newEnvConfig.key}
                  onChange={(e) => setNewEnvConfig({ ...newEnvConfig, key: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="env_value">Value</Label>
                <Input
                  id="env_value"
                  type="password"
                  placeholder="Enter value (will be encrypted)"
                  value={newEnvConfig.value}
                  onChange={(e) => setNewEnvConfig({ ...newEnvConfig, value: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="env_description">Description (Optional)</Label>
                <Input
                  id="env_description"
                  placeholder="Optional description"
                  value={newEnvConfig.description}
                  onChange={(e) => setNewEnvConfig({ ...newEnvConfig, description: e.target.value })}
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsAddEnvOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={setEnvConfigMutation.isPending}>
                {setEnvConfigMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Add
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      {/* Add Env Config Dialog (Create Flow) */}
      <Dialog open={isCreateEnvAddOpen} onOpenChange={setIsCreateEnvAddOpen}>
        <DialogContent>
          <form onSubmit={(e) => {
            e.preventDefault();
            setNewTargetEnvVars([...newTargetEnvVars, {
              scopeType: createEnvScope.type,
              scopeValue: createEnvScope.type !== "default" ? createEnvForm.scopeValue : undefined,
              key: createEnvForm.key,
              value: createEnvForm.value,
              description: createEnvForm.description,
            }]);
            setIsCreateEnvAddOpen(false);
            setCreateEnvForm({ key: "", value: "", description: "", scopeValue: "" });
          }}>
            <DialogHeader>
              <DialogTitle>Add Environment Variable</DialogTitle>
              <DialogDescription>
                Scope: {createEnvScope.type === "default" ? "Default" : createEnvScope.type}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              {createEnvScope.type !== "default" && (
                <div className="space-y-2">
                  <Label htmlFor="create_scope_value">
                    {createEnvScope.type === "role" ? "Role Name" : createEnvScope.type === "group" ? "Group Name" : "User ID"}
                  </Label>
                  <Input
                    id="create_scope_value"
                    placeholder={createEnvScope.type === "role" ? "e.g., admin" : createEnvScope.type === "group" ? "e.g., developers" : "User UUID"}
                    value={createEnvForm.scopeValue}
                    onChange={(e) => setCreateEnvForm({ ...createEnvForm, scopeValue: e.target.value })}
                    required
                  />
                </div>
              )}
              <div className="space-y-2">
                <Label htmlFor="create_env_key">Key</Label>
                <Input
                  id="create_env_key"
                  placeholder="e.g., AUTH_TOKEN"
                  value={createEnvForm.key}
                  onChange={(e) => setCreateEnvForm({ ...createEnvForm, key: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="create_env_value">Value</Label>
                <Input
                  id="create_env_value"
                  type="password"
                  placeholder="Enter value (will be encrypted)"
                  value={createEnvForm.value}
                  onChange={(e) => setCreateEnvForm({ ...createEnvForm, value: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="create_env_description">Description (Optional)</Label>
                <Input
                  id="create_env_description"
                  placeholder="Optional description"
                  value={createEnvForm.description}
                  onChange={(e) => setCreateEnvForm({ ...createEnvForm, description: e.target.value })}
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateEnvAddOpen(false)}>Cancel</Button>
              <Button type="submit">Add</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}
