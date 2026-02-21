package database

import (
	"time"

	"github.com/google/uuid"
)

// User represents a gateway user
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Groups       []string  `json:"groups"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// APIToken represents a JWT token for API access
type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	JTI        string     `json:"jti"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// Target represents an MCP server upstream
type Target struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	URL                   string    `json:"url"`
	TransportType         string    `json:"transport_type"`                    // "streamable-http" (default), "sse", or "stdio"
	Command               string    `json:"command,omitempty"`                 // STDIO: command to execute
	Args                  []string  `json:"args,omitempty"`                    // STDIO: command arguments
	Image                 string    `json:"image,omitempty"`                   // Kubernetes: container image
	Port                  int       `json:"port,omitempty"`                    // Kubernetes: MCP server port (default 8080)
	HealthPath            string    `json:"health_path,omitempty"`             // Kubernetes: readiness probe path (default "/")
	Statefulness          string    `json:"statefulness"`                      // "stateless" or "stateful"
	IsolationBoundary     string    `json:"isolation_boundary"`                // "shared", "per_group", "per_role", "per_user"
	AuthType              string    `json:"auth_type"`
	AuthHeaderName        string    `json:"auth_header_name,omitempty"`
	Enabled               bool      `json:"enabled"`
	DefaultEncryptedToken *string   `json:"-"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// MCPInstance represents a running MCP process instance
type MCPInstance struct {
	ID         uuid.UUID  `json:"id"`
	TargetID   uuid.UUID  `json:"target_id"`
	SubjectKey string     `json:"subject_key"`
	PID        *int       `json:"pid,omitempty"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	LastUsedAt time.Time  `json:"last_used_at"`
	StoppedAt  *time.Time `json:"stopped_at,omitempty"`
}

// UserTargetToken represents an encrypted token for a specific user and target
type UserTargetToken struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	TargetID       uuid.UUID `json:"target_id"`
	EncryptedToken string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RoleTargetToken represents an encrypted token for a specific role and target
type RoleTargetToken struct {
	ID             uuid.UUID `json:"id"`
	Role           string    `json:"role"`
	TargetID       uuid.UUID `json:"target_id"`
	EncryptedToken string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GroupTargetToken represents an encrypted token for a specific group and target
type GroupTargetToken struct {
	ID             uuid.UUID `json:"id"`
	GroupName      string    `json:"group_name"`
	TargetID       uuid.UUID `json:"target_id"`
	EncryptedToken string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// MCPSession represents an active MCP session
type MCPSession struct {
	ID           string    `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RequestLog represents a logged MCP request
type RequestLog struct {
	ID             uuid.UUID  `json:"id"`
	SessionID      string     `json:"session_id,omitempty"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	Method         string     `json:"method"`
	TargetName     string     `json:"target_name,omitempty"`
	RequestBody    []byte     `json:"request_body,omitempty"`
	ResponseStatus int        `json:"response_status"`
	DurationMS     int        `json:"duration_ms"`
	CreatedAt      time.Time  `json:"created_at"`
}

// TokenInfo represents the resolved token info for a target
type TokenInfo struct {
	Token  string `json:"token"`
	Source string `json:"source"` // "user", "role", "group", "default"
}

// CreateUserRequest is used for user registration
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is used for user login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateTargetRequest is used for creating a new target
type CreateTargetRequest struct {
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	TransportType     string   `json:"transport_type,omitempty"`     // "streamable-http" (default), "sse", or "stdio"
	Command           string   `json:"command,omitempty"`            // STDIO: command to execute
	Args              []string `json:"args,omitempty"`               // STDIO: command arguments
	Image             string   `json:"image,omitempty"`              // Kubernetes: container image
	Port              int      `json:"port,omitempty"`               // Kubernetes: MCP server port (default 8080)
	HealthPath        string   `json:"health_path,omitempty"`        // Kubernetes: readiness probe path (default "/")
	Statefulness      string   `json:"statefulness,omitempty"`       // "stateless" or "stateful"; default: "stateless"
	IsolationBoundary string   `json:"isolation_boundary,omitempty"` // default: "shared"
	AuthType          string   `json:"auth_type"`
	AuthHeaderName    string   `json:"auth_header_name,omitempty"`
}

// UpdateTargetRequest is used for updating an existing target
type UpdateTargetRequest struct {
	Name              *string   `json:"name,omitempty"`
	URL               *string   `json:"url,omitempty"`
	TransportType     *string   `json:"transport_type,omitempty"`
	Command           *string   `json:"command,omitempty"`
	Args              *[]string `json:"args,omitempty"`
	Image             *string   `json:"image,omitempty"`
	Port              *int      `json:"port,omitempty"`
	HealthPath        *string   `json:"health_path,omitempty"`
	Statefulness      *string   `json:"statefulness,omitempty"`
	IsolationBoundary *string   `json:"isolation_boundary,omitempty"`
	AuthType          *string   `json:"auth_type,omitempty"`
	AuthHeaderName    *string   `json:"auth_header_name,omitempty"`
	Enabled           *bool     `json:"enabled,omitempty"`
}

// SetTokenRequest is used for setting a user's token for a target
type SetTokenRequest struct {
	Token string `json:"token"`
}

// SetRoleTokenRequest is used for setting a role's token for a target
type SetRoleTokenRequest struct {
	Role  string `json:"role"`
	Token string `json:"token"`
}

// SetGroupTokenRequest is used for setting a group's token for a target
type SetGroupTokenRequest struct {
	GroupName string `json:"group_name"`
	Token     string `json:"token"`
}

// SetDefaultTokenRequest is used for setting the default token for a target
type SetDefaultTokenRequest struct {
	Token string `json:"token"`
}

// CreateAPITokenRequest is used for creating a new API token
type CreateAPITokenRequest struct {
	Name string `json:"name"`
}

// UpdateUserRequest is used for updating a user's role and groups
type UpdateUserRequest struct {
	Role   *string   `json:"role,omitempty"`
	Groups *[]string `json:"groups,omitempty"`
}

// TargetTokenConfig represents the token configuration for a target
type TargetTokenConfig struct {
	TargetID     uuid.UUID          `json:"target_id"`
	TargetName   string             `json:"target_name"`
	HasDefault   bool               `json:"has_default"`
	UserTokens   []UserTokenInfo    `json:"user_tokens,omitempty"`
	RoleTokens   []RoleTokenInfo    `json:"role_tokens,omitempty"`
	GroupTokens  []GroupTokenInfo   `json:"group_tokens,omitempty"`
}

// UserTokenInfo represents a user token info
type UserTokenInfo struct {
	UserID    uuid.UUID `json:"user_id"`
	UserEmail string    `json:"user_email"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RoleTokenInfo represents a role token info
type RoleTokenInfo struct {
	Role      string    `json:"role"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GroupTokenInfo represents a group token info
type GroupTokenInfo struct {
	GroupName string    `json:"group_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// AUTHORIZATION POLICIES
// ============================================================================

// AuthorizationPolicy represents an authorization policy
type AuthorizationPolicy struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	TargetID        *uuid.UUID      `json:"target_id,omitempty"` // NULL = applies to all targets
	ResourceType    string          `json:"resource_type"`       // "all", "tool", "resource", "prompt"
	ResourcePattern *string         `json:"resource_pattern,omitempty"`
	Effect          string          `json:"effect"`   // "allow" or "deny"
	Priority        int             `json:"priority"` // Higher = evaluated first
	Enabled         bool            `json:"enabled"`
	Subjects        []PolicySubject `json:"subjects,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// PolicySubject represents who a policy applies to
type PolicySubject struct {
	ID           uuid.UUID `json:"id"`
	PolicyID     uuid.UUID `json:"policy_id"`
	SubjectType  string    `json:"subject_type"`  // "user", "role", "group", "everyone"
	SubjectValue *string   `json:"subject_value"` // user_id, role name, group name (NULL for "everyone")
	CreatedAt    time.Time `json:"created_at"`
}

// CreatePolicyRequest is used for creating a new authorization policy
type CreatePolicyRequest struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description,omitempty"`
	TargetID        *uuid.UUID              `json:"target_id,omitempty"`
	ResourceType    string                  `json:"resource_type"`
	ResourcePattern *string                 `json:"resource_pattern,omitempty"`
	Effect          string                  `json:"effect"`
	Priority        int                     `json:"priority"`
	Enabled         *bool                   `json:"enabled,omitempty"`
	Subjects        []CreateSubjectRequest  `json:"subjects,omitempty"`
}

// UpdatePolicyRequest is used for updating an existing policy
type UpdatePolicyRequest struct {
	Name            *string    `json:"name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	TargetID        *uuid.UUID `json:"target_id,omitempty"`
	ResourceType    *string    `json:"resource_type,omitempty"`
	ResourcePattern *string    `json:"resource_pattern,omitempty"`
	Effect          *string    `json:"effect,omitempty"`
	Priority        *int       `json:"priority,omitempty"`
	Enabled         *bool      `json:"enabled,omitempty"`
}

// CreateSubjectRequest is used for adding a subject to a policy
type CreateSubjectRequest struct {
	SubjectType  string  `json:"subject_type"`
	SubjectValue *string `json:"subject_value,omitempty"`
}

// AuthorizationCheckRequest is used for checking authorization
type AuthorizationCheckRequest struct {
	TargetID     *uuid.UUID `json:"target_id,omitempty"`
	TargetName   *string    `json:"target_name,omitempty"`
	ResourceType string     `json:"resource_type"` // "tool", "resource", "prompt"
	ResourceName string     `json:"resource_name"`
}

// AuthorizationCheckResult is the result of an authorization check
type AuthorizationCheckResult struct {
	Allowed      bool    `json:"allowed"`
	MatchedPolicy *string `json:"matched_policy,omitempty"`
	Reason       string  `json:"reason"`
}

// ============================================================================
// ENVIRONMENT CONFIGURATIONS
// ============================================================================

// TargetEnvConfig represents an environment configuration for a target
type TargetEnvConfig struct {
	ID             uuid.UUID `json:"id"`
	TargetID       uuid.UUID `json:"target_id"`
	ScopeType      string    `json:"scope_type"`  // "default", "role", "group", "user"
	ScopeValue     *string   `json:"scope_value"` // NULL for default, otherwise role/group/user_id
	EnvKey         string    `json:"env_key"`
	EncryptedValue string    `json:"-"`
	Description    *string   `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EnvConfigInfo represents a resolved environment config
type EnvConfigInfo struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"` // Only shown in resolved view, not in list
	Source      string `json:"source"`          // "user", "group", "role", "default"
	ScopeValue  string `json:"scope_value,omitempty"`
	Description string `json:"description,omitempty"`
}

// SetEnvConfigRequest is used for setting an environment config
type SetEnvConfigRequest struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

// TargetEnvConfigList represents the complete env config for a target
type TargetEnvConfigList struct {
	TargetID      uuid.UUID                    `json:"target_id"`
	TargetName    string                       `json:"target_name"`
	DefaultConfigs []EnvConfigListItem         `json:"default_configs"`
	RoleConfigs   map[string][]EnvConfigListItem `json:"role_configs"`
	GroupConfigs  map[string][]EnvConfigListItem `json:"group_configs"`
	UserConfigs   map[string][]EnvConfigListItem `json:"user_configs"`
}

// EnvConfigListItem represents an item in the env config list
type EnvConfigListItem struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"`
	Description *string   `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ResolvedEnvConfig represents the fully resolved env config for a user
type ResolvedEnvConfig struct {
	TargetID   uuid.UUID       `json:"target_id"`
	TargetName string          `json:"target_name"`
	Configs    []EnvConfigInfo `json:"configs"`
}
