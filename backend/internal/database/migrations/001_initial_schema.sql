-- Reflow Gateway — Complete Schema
-- Single consolidated migration for fresh installations

-- ─── Users ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(50)  NOT NULL DEFAULT 'user',
    groups        TEXT[]       NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── API Tokens ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS api_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    jti         VARCHAR(100) UNIQUE NOT NULL,
    name        VARCHAR(100),
    last_used_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at  TIMESTAMPTZ
);

-- ─── Targets (upstream MCP servers) ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS targets (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  VARCHAR(100) UNIQUE NOT NULL,
    -- HTTP / SSE transports
    url                   VARCHAR(500) NOT NULL DEFAULT '',
    -- STDIO transport
    command               VARCHAR(500) NOT NULL DEFAULT '',
    args                  TEXT[]       NOT NULL DEFAULT '{}',
    -- Kubernetes transport
    image                 VARCHAR(500) NOT NULL DEFAULT '',
    port                  INTEGER      NOT NULL DEFAULT 8080,
    health_path           VARCHAR(500) NOT NULL DEFAULT '',
    -- Transport and behaviour
    transport_type        VARCHAR(50)  NOT NULL DEFAULT 'streamable-http',
    statefulness          VARCHAR(50)  NOT NULL DEFAULT 'stateless',
    isolation_boundary    VARCHAR(20)  NOT NULL DEFAULT 'shared',
    -- Authentication (HTTP/SSE)
    auth_type             VARCHAR(50)  NOT NULL DEFAULT 'none',
    auth_header_name      VARCHAR(100),
    default_encrypted_token TEXT,
    enabled               BOOLEAN      NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── Per-target Credential Tables ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_target_tokens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    target_id       UUID        NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    encrypted_token TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, target_id)
);

CREATE TABLE IF NOT EXISTS role_target_tokens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    role            VARCHAR(50) NOT NULL,
    target_id       UUID        NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    encrypted_token TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(role, target_id)
);

CREATE TABLE IF NOT EXISTS group_target_tokens (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    group_name      VARCHAR(100) NOT NULL,
    target_id       UUID         NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    encrypted_token TEXT         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(group_name, target_id)
);

-- ─── MCP Sessions ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS mcp_sessions (
    id            VARCHAR(100) PRIMARY KEY,
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_activity TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ  NOT NULL
);

-- ─── Request Audit Logs ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS request_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      VARCHAR(100),
    user_id         UUID,
    method          VARCHAR(100),
    target_name     VARCHAR(100),
    request_body    JSONB,
    response_status INT,
    duration_ms     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Authorization Policies ───────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS authorization_policies (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    target_id        UUID         REFERENCES targets(id) ON DELETE CASCADE,
    resource_type    VARCHAR(50)  NOT NULL DEFAULT 'all',
    resource_pattern VARCHAR(255),
    effect           VARCHAR(10)  NOT NULL CHECK (effect IN ('allow', 'deny')),
    priority         INTEGER      NOT NULL DEFAULT 0,
    enabled          BOOLEAN      NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policy_subjects (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id     UUID        NOT NULL REFERENCES authorization_policies(id) ON DELETE CASCADE,
    subject_type  VARCHAR(20) NOT NULL CHECK (subject_type IN ('user', 'role', 'group', 'everyone')),
    subject_value VARCHAR(255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Environment Configurations ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS target_env_configs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id       UUID         NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    scope_type      VARCHAR(20)  NOT NULL CHECK (scope_type IN ('default', 'role', 'group', 'user')),
    scope_value     VARCHAR(255),
    env_key         VARCHAR(255) NOT NULL,
    encrypted_value TEXT         NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── MCP Instances (STDIO processes / Kubernetes pods) ───────────────────────
CREATE TABLE IF NOT EXISTS mcp_instances (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id   UUID         NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    subject_key VARCHAR(255) NOT NULL,
    pid         INTEGER,
    status      VARCHAR(20)  NOT NULL DEFAULT 'running',
    started_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    stopped_at  TIMESTAMPTZ,
    UNIQUE(target_id, subject_key)
);

-- ─── Indexes ─────────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id              ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_jti                  ON api_tokens(jti);
CREATE INDEX IF NOT EXISTS idx_users_role                      ON users(role);
CREATE INDEX IF NOT EXISTS idx_user_target_tokens_user_id      ON user_target_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_user_target_tokens_target_id    ON user_target_tokens(target_id);
CREATE INDEX IF NOT EXISTS idx_role_target_tokens_role         ON role_target_tokens(role);
CREATE INDEX IF NOT EXISTS idx_role_target_tokens_target_id    ON role_target_tokens(target_id);
CREATE INDEX IF NOT EXISTS idx_group_target_tokens_group       ON group_target_tokens(group_name);
CREATE INDEX IF NOT EXISTS idx_group_target_tokens_target_id   ON group_target_tokens(target_id);
CREATE INDEX IF NOT EXISTS idx_mcp_sessions_user_id            ON mcp_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_mcp_sessions_expires_at         ON mcp_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_request_logs_user_id            ON request_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_session_id         ON request_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_created_at         ON request_logs(created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_policy_subject_unique    ON policy_subjects(policy_id, subject_type, COALESCE(subject_value, ''));
CREATE INDEX IF NOT EXISTS idx_policies_target                 ON authorization_policies(target_id, enabled);
CREATE INDEX IF NOT EXISTS idx_policies_priority               ON authorization_policies(priority DESC) WHERE enabled = true;
CREATE UNIQUE INDEX IF NOT EXISTS idx_env_config_unique        ON target_env_configs(target_id, scope_type, COALESCE(scope_value, ''), env_key);
CREATE INDEX IF NOT EXISTS idx_env_configs_target              ON target_env_configs(target_id, scope_type);
CREATE INDEX IF NOT EXISTS idx_env_configs_lookup              ON target_env_configs(target_id, scope_type, scope_value);

-- ─── Default Policies ────────────────────────────────────────────────────────
INSERT INTO authorization_policies (name, description, target_id, resource_type, effect, priority)
VALUES
    ('Admin Full Access',  'Administrators have full access to all targets and resources', NULL, 'all', 'allow', 1000),
    ('Default Allow All',  'Default policy allowing all authenticated users (disable to enforce stricter access)', NULL, 'all', 'allow', 1)
ON CONFLICT DO NOTHING;

INSERT INTO policy_subjects (policy_id, subject_type, subject_value)
SELECT id, 'role', 'admin' FROM authorization_policies WHERE name = 'Admin Full Access'
ON CONFLICT DO NOTHING;

INSERT INTO policy_subjects (policy_id, subject_type, subject_value)
SELECT id, 'everyone', NULL FROM authorization_policies WHERE name = 'Default Allow All'
ON CONFLICT DO NOTHING;
