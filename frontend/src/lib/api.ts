const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
};

class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const token = typeof window !== "undefined" ? localStorage.getItem("token") : null;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Request failed" }));
    throw new ApiError(error.error || "Request failed", response.status);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}

// Auth API
export const authApi = {
  register: (email: string, password: string) =>
    request<{ user: User; token: string }>("/api/auth/register", {
      method: "POST",
      body: { email, password },
    }),

  login: (email: string, password: string) =>
    request<{ user: User; token: string }>("/api/auth/login", {
      method: "POST",
      body: { email, password },
    }),

  me: () => request<User>("/api/auth/me"),

  listTokens: () => request<APIToken[]>("/api/auth/tokens"),

  createToken: (name: string) =>
    request<{ api_token: APIToken; token: string }>("/api/auth/tokens", {
      method: "POST",
      body: { name },
    }),

  revokeToken: (id: string) =>
    request<void>(`/api/auth/tokens/${id}`, { method: "DELETE" }),
};

// Users API (Admin only)
export const usersApi = {
  list: () => request<User[]>("/api/users"),

  update: (id: string, data: UpdateUserRequest) =>
    request<User>(`/api/users/${id}`, {
      method: "PUT",
      body: data,
    }),

  recycleSessions: (id: string) =>
    request<{ recycled: number; user_id: string }>(`/api/users/${id}/recycle`, {
      method: "POST",
    }),
};

// Sessions API
export const sessionsApi = {
  recycleSelf: () =>
    request<{ recycled: number; user_id: string }>("/api/sessions/recycle", {
      method: "POST",
    }),
};

// Targets API
export const targetsApi = {
  list: () => request<Target[]>("/api/targets"),

  create: (data: CreateTargetRequest) =>
    request<Target>("/api/targets", {
      method: "POST",
      body: data,
    }),

  get: (id: string) => request<Target>(`/api/targets/${id}`),

  update: (id: string, data: UpdateTargetRequest) =>
    request<Target>(`/api/targets/${id}`, {
      method: "PUT",
      body: data,
    }),

  delete: (id: string) =>
    request<void>(`/api/targets/${id}`, { method: "DELETE" }),

  restartInstances: (id: string) =>
    request<{ deleted: number; message: string }>(`/api/targets/${id}/restart-instances`, {
      method: "POST",
    }),

  // Default token (legacy, now sets default_encrypted_token)
  getToken: (id: string) =>
    request<{ has_token: boolean; created_at?: string; updated_at?: string }>(
      `/api/targets/${id}/token`
    ),

  setToken: (id: string, token: string) =>
    request<{ message: string }>(`/api/targets/${id}/token`, {
      method: "PUT",
      body: { token },
    }),

  deleteToken: (id: string) =>
    request<void>(`/api/targets/${id}/token`, { method: "DELETE" }),

  // Token configuration
  getTokenConfig: (id: string) =>
    request<TargetTokenConfig>(`/api/targets/${id}/tokens`),

  // Default token management
  setDefaultToken: (id: string, token: string) =>
    request<{ message: string }>(`/api/targets/${id}/tokens/default`, {
      method: "PUT",
      body: { token },
    }),

  deleteDefaultToken: (id: string) =>
    request<void>(`/api/targets/${id}/tokens/default`, { method: "DELETE" }),

  // Role token management
  setRoleToken: (id: string, role: string, token: string) =>
    request<{ message: string }>(`/api/targets/${id}/tokens/role`, {
      method: "PUT",
      body: { role, token },
    }),

  deleteRoleToken: (id: string, role: string) =>
    request<void>(`/api/targets/${id}/tokens/role/${role}`, { method: "DELETE" }),

  // Group token management
  setGroupToken: (id: string, group: string, token: string) =>
    request<{ message: string }>(`/api/targets/${id}/tokens/group`, {
      method: "PUT",
      body: { group, token },
    }),

  deleteGroupToken: (id: string, group: string) =>
    request<void>(`/api/targets/${id}/tokens/group/${group}`, { method: "DELETE" }),
};

// Health API (no auth required)
export const healthApi = {
  check: async (): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE_URL}/health`);
    if (!response.ok) {
      throw new Error("Health check failed");
    }
    return response.json();
  },
};

// Logs API
export const logsApi = {
  list: (limit = 50, offset = 0) =>
    request<RequestLog[]>(`/api/logs?limit=${limit}&offset=${offset}`),
};

// Policies API
export const policiesApi = {
  list: () => request<AuthorizationPolicy[]>("/api/policies"),

  create: (data: CreatePolicyRequest) =>
    request<AuthorizationPolicy>("/api/policies", {
      method: "POST",
      body: data,
    }),

  get: (id: string) => request<AuthorizationPolicy>(`/api/policies/${id}`),

  update: (id: string, data: UpdatePolicyRequest) =>
    request<AuthorizationPolicy>(`/api/policies/${id}`, {
      method: "PUT",
      body: data,
    }),

  delete: (id: string) =>
    request<void>(`/api/policies/${id}`, { method: "DELETE" }),

  addSubject: (policyId: string, data: CreateSubjectRequest) =>
    request<PolicySubject>(`/api/policies/${policyId}/subjects`, {
      method: "POST",
      body: data,
    }),

  deleteSubject: (policyId: string, subjectId: string) =>
    request<void>(`/api/policies/${policyId}/subjects/${subjectId}`, {
      method: "DELETE",
    }),
};

// Environment Configs API
export const envConfigsApi = {
  listAll: (targetId: string) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env`),

  resolve: (targetId: string) =>
    request<ResolvedEnvConfig[]>(`/api/targets/${targetId}/env/resolve`),

  // Default scope
  getDefault: (targetId: string) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env/default`),

  setDefault: (targetId: string, key: string, value: string, description?: string) =>
    request<TargetEnvConfig>(`/api/targets/${targetId}/env/default`, {
      method: "POST",
      body: { key, value, description },
    }),

  bulkSetDefault: (targetId: string, configs: Record<string, string>) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env/default`, {
      method: "PUT",
      body: { configs },
    }),

  deleteDefault: (targetId: string, key: string) =>
    request<void>(`/api/targets/${targetId}/env/default/${key}`, {
      method: "DELETE",
    }),

  // Role scope
  getRole: (targetId: string, role: string) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env/role/${role}`),

  setRole: (targetId: string, role: string, key: string, value: string, description?: string) =>
    request<TargetEnvConfig>(`/api/targets/${targetId}/env/role/${role}`, {
      method: "POST",
      body: { key, value, description },
    }),

  deleteRole: (targetId: string, role: string, key: string) =>
    request<void>(`/api/targets/${targetId}/env/role/${role}/${key}`, {
      method: "DELETE",
    }),

  // Group scope
  getGroup: (targetId: string, group: string) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env/group/${group}`),

  setGroup: (targetId: string, group: string, key: string, value: string, description?: string) =>
    request<TargetEnvConfig>(`/api/targets/${targetId}/env/group/${group}`, {
      method: "POST",
      body: { key, value, description },
    }),

  deleteGroup: (targetId: string, group: string, key: string) =>
    request<void>(`/api/targets/${targetId}/env/group/${group}/${key}`, {
      method: "DELETE",
    }),

  // User scope
  getUser: (targetId: string, userId: string) =>
    request<TargetEnvConfig[]>(`/api/targets/${targetId}/env/user/${userId}`),

  setUser: (targetId: string, userId: string, key: string, value: string, description?: string) =>
    request<TargetEnvConfig>(`/api/targets/${targetId}/env/user/${userId}`, {
      method: "POST",
      body: { key, value, description },
    }),

  deleteUser: (targetId: string, userId: string, key: string) =>
    request<void>(`/api/targets/${targetId}/env/user/${userId}/${key}`, {
      method: "DELETE",
    }),
};

// Types
export interface User {
  id: string;
  email: string;
  role: string;
  groups?: string[];
  created_at: string;
  updated_at: string;
}

export interface UpdateUserRequest {
  role?: string;
  groups?: string[];
}

export interface APIToken {
  id: string;
  user_id: string;
  jti: string;
  name: string;
  last_used_at: string | null;
  created_at: string;
  revoked_at: string | null;
}

export type TransportType = "streamable-http" | "sse" | "stdio" | "kubernetes";
export type Statefulness = "stateless" | "stateful";
export type IsolationBoundary = "shared" | "per_group" | "per_role" | "per_user";

export interface Target {
  id: string;
  name: string;
  url: string;
  transport_type: TransportType;
  command?: string;
  args?: string[];
  image?: string;
  port?: number;
  health_path?: string;
  statefulness: Statefulness;
  isolation_boundary: IsolationBoundary;
  auth_type: string;
  auth_header_name?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateTargetRequest {
  name: string;
  url?: string;
  transport_type?: TransportType;
  command?: string;
  args?: string[];
  image?: string;
  port?: number;
  health_path?: string;
  statefulness?: Statefulness;
  isolation_boundary?: IsolationBoundary;
  auth_type?: string;
  auth_header_name?: string;
}

export interface UpdateTargetRequest {
  name?: string;
  url?: string;
  transport_type?: TransportType;
  command?: string;
  args?: string[];
  image?: string;
  port?: number;
  health_path?: string;
  statefulness?: Statefulness;
  isolation_boundary?: IsolationBoundary;
  auth_type?: string;
  auth_header_name?: string;
  enabled?: boolean;
}

export interface RequestLog {
  id: string;
  session_id: string;
  user_id: string;
  method: string;
  target_name: string;
  request_body: unknown;
  response_status: number;
  duration_ms: number;
  created_at: string;
}

export interface TargetTokenConfig {
  target_id: string;
  target_name: string;
  has_default_token: boolean;
  role_tokens: RoleTokenInfo[];
  group_tokens: GroupTokenInfo[];
}

export interface RoleTokenInfo {
  role: string;
  created_at: string;
  updated_at: string;
}

export interface GroupTokenInfo {
  group_name: string;
  created_at: string;
  updated_at: string;
}

// Authorization Policy types
export interface AuthorizationPolicy {
  id: string;
  name: string;
  description?: string;
  target_id?: string;
  resource_type: string;
  resource_pattern?: string;
  effect: "allow" | "deny";
  priority: number;
  enabled: boolean;
  subjects?: PolicySubject[];
  created_at: string;
  updated_at: string;
}

export interface PolicySubject {
  id: string;
  policy_id: string;
  subject_type: "user" | "role" | "group" | "everyone";
  subject_value?: string;
  created_at: string;
}

export interface CreatePolicyRequest {
  name: string;
  description?: string;
  target_id?: string;
  resource_type?: string;
  resource_pattern?: string;
  effect: "allow" | "deny";
  priority?: number;
  enabled?: boolean;
  subjects?: CreateSubjectRequest[];
}

export interface UpdatePolicyRequest {
  name?: string;
  description?: string;
  target_id?: string;
  resource_type?: string;
  resource_pattern?: string;
  effect?: "allow" | "deny";
  priority?: number;
  enabled?: boolean;
}

export interface CreateSubjectRequest {
  subject_type: "user" | "role" | "group" | "everyone";
  subject_value?: string;
}

// Environment Config types
export interface TargetEnvConfig {
  id: string;
  target_id: string;
  scope_type: "default" | "role" | "group" | "user";
  scope_value?: string;
  env_key: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface ResolvedEnvConfig {
  key: string;
  scope_type: string;
  scope_value?: string;
}

// Observability API
export const observabilityApi = {
  snapshot: () => request<MetricsSnapshot>("/api/observability/snapshot"),

  wsUrl: () => {
    const base = API_BASE_URL.replace(/^http/, "ws");
    const token = typeof window !== "undefined" ? localStorage.getItem("token") : null;
    return `${base}/api/observability/ws${token ? `?token=${token}` : ""}`;
  },
};

// Observability types
export interface MetricsSnapshot {
  timestamp: string;
  total_requests: number;
  total_errors: number;
  avg_latency_ms: number;
  error_rate: number;
  active_sessions: number;
  targets: TargetStats[];
}

export interface TargetStats {
  name: string;
  request_count: number;
  error_count: number;
  avg_latency_ms: number;
  error_rate: number;
  rps: number;
}

export interface WSEvent {
  type: "activity" | "metrics" | "session" | "error";
  data: unknown;
}

export interface ActivityEvent {
  timestamp: string;
  user_id: string;
  user_email?: string;
  method: string;
  target?: string;
  tool?: string;
  duration_ms: number;
  status: string;
  trace_id?: string;
}

export interface SessionEvent {
  event: "created" | "deleted" | "recycled";
  session_id: string;
  user_id: string;
  targets?: string[];
}

export interface ErrorEvent {
  timestamp: string;
  user_id?: string;
  target?: string;
  error_type: string;
  message: string;
}

export { ApiError };
