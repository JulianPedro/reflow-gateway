package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// Repository provides CRUD operations for all models
type Repository struct {
	db *DB
}

// NewRepository creates a new repository
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// ==================== User Operations ====================

// CountUsers returns the total number of users
func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         "user",
		Groups:       []string{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, groups, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Email, user.PasswordHash, user.Role, user.Groups, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, email, password_hash, COALESCE(role, 'user'), COALESCE(groups, '{}'), created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.Groups, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, email, password_hash, COALESCE(role, 'user'), COALESCE(groups, '{}'), created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.Groups, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateUser updates a user's role and groups
func (r *Repository) UpdateUser(ctx context.Context, id uuid.UUID, req *UpdateUserRequest) (*User, error) {
	user, err := r.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Groups != nil {
		user.Groups = *req.Groups
	}
	user.UpdatedAt = time.Now()

	_, err = r.db.Pool.Exec(ctx, `
		UPDATE users SET role = $2, groups = $3, updated_at = $4 WHERE id = $1
	`, id, user.Role, user.Groups, user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetAllUsers retrieves all users
func (r *Repository) GetAllUsers(ctx context.Context) ([]*User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, email, password_hash, COALESCE(role, 'user'), COALESCE(groups, '{}'), created_at, updated_at
		FROM users ORDER BY email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.Groups, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// DeleteUser deletes a user
func (r *Repository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== API Token Operations ====================

// CreateAPIToken creates a new API token
func (r *Repository) CreateAPIToken(ctx context.Context, userID uuid.UUID, jti, name string) (*APIToken, error) {
	token := &APIToken{
		ID:        uuid.New(),
		UserID:    userID,
		JTI:       jti,
		Name:      name,
		CreatedAt: time.Now(),
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO api_tokens (id, user_id, jti, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token.ID, token.UserID, token.JTI, token.Name, token.CreatedAt)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// GetAPITokenByJTI retrieves an API token by JTI
func (r *Repository) GetAPITokenByJTI(ctx context.Context, jti string) (*APIToken, error) {
	token := &APIToken{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, jti, name, last_used_at, created_at, revoked_at
		FROM api_tokens WHERE jti = $1
	`, jti).Scan(&token.ID, &token.UserID, &token.JTI, &token.Name,
		&token.LastUsedAt, &token.CreatedAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

// GetAPITokensByUserID retrieves all API tokens for a user
func (r *Repository) GetAPITokensByUserID(ctx context.Context, userID uuid.UUID) ([]*APIToken, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, jti, name, last_used_at, created_at, revoked_at
		FROM api_tokens WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*APIToken
	for rows.Next() {
		token := &APIToken{}
		err := rows.Scan(&token.ID, &token.UserID, &token.JTI, &token.Name,
			&token.LastUsedAt, &token.CreatedAt, &token.RevokedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// UpdateAPITokenLastUsed updates the last_used_at timestamp
func (r *Repository) UpdateAPITokenLastUsed(ctx context.Context, jti string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE api_tokens SET last_used_at = NOW() WHERE jti = $1
	`, jti)
	return err
}

// RevokeAPIToken revokes an API token
func (r *Repository) RevokeAPIToken(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE api_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Target Operations ====================

// CreateTarget creates a new target
func (r *Repository) CreateTarget(ctx context.Context, req *CreateTargetRequest) (*Target, error) {
	transportType := req.TransportType
	if transportType == "" {
		transportType = "streamable-http"
	}
	statefulness := req.Statefulness
	if statefulness == "" {
		statefulness = "stateless"
	}
	isolationBoundary := req.IsolationBoundary
	if isolationBoundary == "" {
		isolationBoundary = "shared"
	}
	args := req.Args
	if args == nil {
		args = []string{}
	}
	port := req.Port
	if port == 0 {
		port = 8080
	}
	healthPath := req.HealthPath

	target := &Target{
		ID:                uuid.New(),
		Name:              req.Name,
		URL:               req.URL,
		TransportType:     transportType,
		Command:           req.Command,
		Args:              args,
		Image:             req.Image,
		Port:              port,
		HealthPath:        healthPath,
		Statefulness:      statefulness,
		IsolationBoundary: isolationBoundary,
		AuthType:          req.AuthType,
		AuthHeaderName:    req.AuthHeaderName,
		Enabled:           true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO targets (id, name, url, transport_type, command, args, image, port, health_path, statefulness, isolation_boundary,
			auth_type, auth_header_name, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, target.ID, target.Name, target.URL, target.TransportType, target.Command, target.Args,
		target.Image, target.Port, target.HealthPath,
		target.Statefulness, target.IsolationBoundary,
		target.AuthType, target.AuthHeaderName,
		target.Enabled, target.CreatedAt, target.UpdatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	return target, nil
}

// GetTargetByID retrieves a target by ID
func (r *Repository) GetTargetByID(ctx context.Context, id uuid.UUID) (*Target, error) {
	target := &Target{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, url, transport_type, command, args, image, port, health_path, statefulness, isolation_boundary,
			auth_type, auth_header_name, enabled, default_encrypted_token, created_at, updated_at
		FROM targets WHERE id = $1
	`, id).Scan(&target.ID, &target.Name, &target.URL, &target.TransportType,
		&target.Command, &target.Args, &target.Image, &target.Port, &target.HealthPath, &target.Statefulness, &target.IsolationBoundary,
		&target.AuthType, &target.AuthHeaderName, &target.Enabled, &target.DefaultEncryptedToken,
		&target.CreatedAt, &target.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return target, nil
}

// GetTargetByName retrieves a target by name
func (r *Repository) GetTargetByName(ctx context.Context, name string) (*Target, error) {
	target := &Target{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, url, transport_type, command, args, image, port, health_path, statefulness, isolation_boundary,
			auth_type, auth_header_name, enabled, default_encrypted_token, created_at, updated_at
		FROM targets WHERE name = $1
	`, name).Scan(&target.ID, &target.Name, &target.URL, &target.TransportType,
		&target.Command, &target.Args, &target.Image, &target.Port, &target.HealthPath, &target.Statefulness, &target.IsolationBoundary,
		&target.AuthType, &target.AuthHeaderName, &target.Enabled, &target.DefaultEncryptedToken,
		&target.CreatedAt, &target.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return target, nil
}

// GetAllTargets retrieves all targets
func (r *Repository) GetAllTargets(ctx context.Context) ([]*Target, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, url, transport_type, command, args, image, port, health_path, statefulness, isolation_boundary,
			auth_type, auth_header_name, enabled, default_encrypted_token, created_at, updated_at
		FROM targets ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*Target
	for rows.Next() {
		target := &Target{}
		err := rows.Scan(&target.ID, &target.Name, &target.URL, &target.TransportType,
			&target.Command, &target.Args, &target.Image, &target.Port, &target.HealthPath, &target.Statefulness, &target.IsolationBoundary,
			&target.AuthType, &target.AuthHeaderName, &target.Enabled, &target.DefaultEncryptedToken,
			&target.CreatedAt, &target.UpdatedAt)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	return targets, nil
}

// GetEnabledTargets retrieves all enabled targets
func (r *Repository) GetEnabledTargets(ctx context.Context) ([]*Target, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, url, transport_type, command, args, image, port, health_path, statefulness, isolation_boundary,
			auth_type, auth_header_name, enabled, default_encrypted_token, created_at, updated_at
		FROM targets WHERE enabled = true ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*Target
	for rows.Next() {
		target := &Target{}
		err := rows.Scan(&target.ID, &target.Name, &target.URL, &target.TransportType,
			&target.Command, &target.Args, &target.Image, &target.Port, &target.HealthPath, &target.Statefulness, &target.IsolationBoundary,
			&target.AuthType, &target.AuthHeaderName, &target.Enabled, &target.DefaultEncryptedToken,
			&target.CreatedAt, &target.UpdatedAt)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	return targets, nil
}

// UpdateTarget updates a target
func (r *Repository) UpdateTarget(ctx context.Context, id uuid.UUID, req *UpdateTargetRequest) (*Target, error) {
	target, err := r.GetTargetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		target.Name = *req.Name
	}
	if req.URL != nil {
		target.URL = *req.URL
	}
	if req.TransportType != nil {
		target.TransportType = *req.TransportType
	}
	if req.Command != nil {
		target.Command = *req.Command
	}
	if req.Args != nil {
		target.Args = *req.Args
	}
	if req.Image != nil {
		target.Image = *req.Image
	}
	if req.Port != nil {
		target.Port = *req.Port
	}
	if req.HealthPath != nil {
		target.HealthPath = *req.HealthPath
	}
	if req.Statefulness != nil {
		target.Statefulness = *req.Statefulness
	}
	if req.IsolationBoundary != nil {
		target.IsolationBoundary = *req.IsolationBoundary
	}
	if req.AuthType != nil {
		target.AuthType = *req.AuthType
	}
	if req.AuthHeaderName != nil {
		target.AuthHeaderName = *req.AuthHeaderName
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	target.UpdatedAt = time.Now()

	_, err = r.db.Pool.Exec(ctx, `
		UPDATE targets SET name = $2, url = $3, transport_type = $4, command = $5, args = $6,
		image = $7, port = $8, health_path = $9, statefulness = $10, isolation_boundary = $11,
		auth_type = $12, auth_header_name = $13, enabled = $14, updated_at = $15
		WHERE id = $1
	`, id, target.Name, target.URL, target.TransportType, target.Command, target.Args,
		target.Image, target.Port, target.HealthPath,
		target.Statefulness, target.IsolationBoundary,
		target.AuthType, target.AuthHeaderName, target.Enabled, target.UpdatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	return target, nil
}

// SetTargetDefaultToken sets the default token for a target
func (r *Repository) SetTargetDefaultToken(ctx context.Context, id uuid.UUID, encryptedToken string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE targets SET default_encrypted_token = $2, updated_at = NOW() WHERE id = $1
	`, id, encryptedToken)
	return err
}

// DeleteTargetDefaultToken removes the default token for a target
func (r *Repository) DeleteTargetDefaultToken(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE targets SET default_encrypted_token = NULL, updated_at = NOW() WHERE id = $1
	`, id)
	return err
}

// DeleteTarget deletes a target
func (r *Repository) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM targets WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== User Target Token Operations ====================

// SetUserTargetToken sets or updates a user's token for a target
func (r *Repository) SetUserTargetToken(ctx context.Context, userID, targetID uuid.UUID, encryptedToken string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO user_target_tokens (id, user_id, target_id, encrypted_token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (user_id, target_id) DO UPDATE SET
			encrypted_token = EXCLUDED.encrypted_token,
			updated_at = NOW()
	`, uuid.New(), userID, targetID, encryptedToken)
	return err
}

// GetUserTargetToken retrieves a user's encrypted token for a target
func (r *Repository) GetUserTargetToken(ctx context.Context, userID, targetID uuid.UUID) (*UserTargetToken, error) {
	token := &UserTargetToken{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, target_id, encrypted_token, created_at, updated_at
		FROM user_target_tokens WHERE user_id = $1 AND target_id = $2
	`, userID, targetID).Scan(&token.ID, &token.UserID, &token.TargetID,
		&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

// GetUserTargetTokens retrieves all tokens for a user
func (r *Repository) GetUserTargetTokens(ctx context.Context, userID uuid.UUID) ([]*UserTargetToken, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, target_id, encrypted_token, created_at, updated_at
		FROM user_target_tokens WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*UserTargetToken
	for rows.Next() {
		token := &UserTargetToken{}
		err := rows.Scan(&token.ID, &token.UserID, &token.TargetID,
			&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// GetAllUserTargetTokensForTarget retrieves all user tokens for a target
func (r *Repository) GetAllUserTargetTokensForTarget(ctx context.Context, targetID uuid.UUID) ([]*UserTargetToken, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, target_id, encrypted_token, created_at, updated_at
		FROM user_target_tokens WHERE target_id = $1
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*UserTargetToken
	for rows.Next() {
		token := &UserTargetToken{}
		err := rows.Scan(&token.ID, &token.UserID, &token.TargetID,
			&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// DeleteUserTargetToken deletes a user's token for a target
func (r *Repository) DeleteUserTargetToken(ctx context.Context, userID, targetID uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM user_target_tokens WHERE user_id = $1 AND target_id = $2
	`, userID, targetID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Role Target Token Operations ====================

// SetRoleTargetToken sets or updates a role's token for a target
func (r *Repository) SetRoleTargetToken(ctx context.Context, role string, targetID uuid.UUID, encryptedToken string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO role_target_tokens (id, role, target_id, encrypted_token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (role, target_id) DO UPDATE SET
			encrypted_token = EXCLUDED.encrypted_token,
			updated_at = NOW()
	`, uuid.New(), role, targetID, encryptedToken)
	return err
}

// GetRoleTargetToken retrieves a role's encrypted token for a target
func (r *Repository) GetRoleTargetToken(ctx context.Context, role string, targetID uuid.UUID) (*RoleTargetToken, error) {
	token := &RoleTargetToken{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, role, target_id, encrypted_token, created_at, updated_at
		FROM role_target_tokens WHERE role = $1 AND target_id = $2
	`, role, targetID).Scan(&token.ID, &token.Role, &token.TargetID,
		&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

// GetAllRoleTargetTokensForTarget retrieves all role tokens for a target
func (r *Repository) GetAllRoleTargetTokensForTarget(ctx context.Context, targetID uuid.UUID) ([]*RoleTargetToken, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, role, target_id, encrypted_token, created_at, updated_at
		FROM role_target_tokens WHERE target_id = $1
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*RoleTargetToken
	for rows.Next() {
		token := &RoleTargetToken{}
		err := rows.Scan(&token.ID, &token.Role, &token.TargetID,
			&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// DeleteRoleTargetToken deletes a role's token for a target
func (r *Repository) DeleteRoleTargetToken(ctx context.Context, role string, targetID uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM role_target_tokens WHERE role = $1 AND target_id = $2
	`, role, targetID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Group Target Token Operations ====================

// SetGroupTargetToken sets or updates a group's token for a target
func (r *Repository) SetGroupTargetToken(ctx context.Context, groupName string, targetID uuid.UUID, encryptedToken string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO group_target_tokens (id, group_name, target_id, encrypted_token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (group_name, target_id) DO UPDATE SET
			encrypted_token = EXCLUDED.encrypted_token,
			updated_at = NOW()
	`, uuid.New(), groupName, targetID, encryptedToken)
	return err
}

// GetGroupTargetToken retrieves a group's encrypted token for a target
func (r *Repository) GetGroupTargetToken(ctx context.Context, groupName string, targetID uuid.UUID) (*GroupTargetToken, error) {
	token := &GroupTargetToken{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, group_name, target_id, encrypted_token, created_at, updated_at
		FROM group_target_tokens WHERE group_name = $1 AND target_id = $2
	`, groupName, targetID).Scan(&token.ID, &token.GroupName, &token.TargetID,
		&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

// GetAllGroupTargetTokensForTarget retrieves all group tokens for a target
func (r *Repository) GetAllGroupTargetTokensForTarget(ctx context.Context, targetID uuid.UUID) ([]*GroupTargetToken, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, group_name, target_id, encrypted_token, created_at, updated_at
		FROM group_target_tokens WHERE target_id = $1
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*GroupTargetToken
	for rows.Next() {
		token := &GroupTargetToken{}
		err := rows.Scan(&token.ID, &token.GroupName, &token.TargetID,
			&token.EncryptedToken, &token.CreatedAt, &token.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// DeleteGroupTargetToken deletes a group's token for a target
func (r *Repository) DeleteGroupTargetToken(ctx context.Context, groupName string, targetID uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM group_target_tokens WHERE group_name = $1 AND target_id = $2
	`, groupName, targetID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Token Resolution ====================

// ResolveTokenForTarget resolves the token for a target based on user, role, groups, or default
// Priority: User > Role > Group > Default
func (r *Repository) ResolveTokenForTarget(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID uuid.UUID) (*TokenInfo, error) {
	// 1. Try user-specific token
	userToken, err := r.GetUserTargetToken(ctx, userID, targetID)
	if err == nil {
		return &TokenInfo{Token: userToken.EncryptedToken, Source: "user"}, nil
	}
	if err != ErrNotFound {
		return nil, err
	}

	// 2. Try role-specific token
	if role != "" {
		roleToken, err := r.GetRoleTargetToken(ctx, role, targetID)
		if err == nil {
			return &TokenInfo{Token: roleToken.EncryptedToken, Source: "role"}, nil
		}
		if err != ErrNotFound {
			return nil, err
		}
	}

	// 3. Try group-specific tokens (first match wins)
	for _, group := range groups {
		groupToken, err := r.GetGroupTargetToken(ctx, group, targetID)
		if err == nil {
			return &TokenInfo{Token: groupToken.EncryptedToken, Source: "group"}, nil
		}
		if err != ErrNotFound {
			return nil, err
		}
	}

	// 4. Try default token
	target, err := r.GetTargetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.DefaultEncryptedToken != nil && *target.DefaultEncryptedToken != "" {
		return &TokenInfo{Token: *target.DefaultEncryptedToken, Source: "default"}, nil
	}

	return nil, ErrNotFound
}

// ==================== MCP Session Operations ====================

// CreateMCPSession creates a new MCP session
func (r *Repository) CreateMCPSession(ctx context.Context, sessionID string, userID uuid.UUID, expiresAt time.Time) (*MCPSession, error) {
	session := &MCPSession{
		ID:           sessionID,
		UserID:       userID,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    expiresAt,
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO mcp_sessions (id, user_id, created_at, last_activity, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, session.ID, session.UserID, session.CreatedAt, session.LastActivity, session.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetMCPSession retrieves an MCP session by ID
func (r *Repository) GetMCPSession(ctx context.Context, sessionID string) (*MCPSession, error) {
	session := &MCPSession{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, last_activity, expires_at
		FROM mcp_sessions WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.UserID, &session.CreatedAt,
		&session.LastActivity, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return session, nil
}

// UpdateMCPSessionActivity updates the last activity timestamp
func (r *Repository) UpdateMCPSessionActivity(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE mcp_sessions SET last_activity = NOW() WHERE id = $1
	`, sessionID)
	return err
}

// DeleteMCPSession deletes an MCP session
func (r *Repository) DeleteMCPSession(ctx context.Context, sessionID string) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM mcp_sessions WHERE id = $1", sessionID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CleanupExpiredSessions removes expired MCP sessions
func (r *Repository) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM mcp_sessions WHERE expires_at < NOW()
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ==================== Request Log Operations ====================

// CreateRequestLog creates a new request log entry
func (r *Repository) CreateRequestLog(ctx context.Context, log *RequestLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now()

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO request_logs (id, session_id, user_id, method, target_name, request_body, response_status, duration_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, log.ID, log.SessionID, log.UserID, log.Method, log.TargetName,
		log.RequestBody, log.ResponseStatus, log.DurationMS, log.CreatedAt)
	return err
}

// GetRequestLogs retrieves request logs with pagination
func (r *Repository) GetRequestLogs(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]*RequestLog, error) {
	var rows pgx.Rows
	var err error

	if userID != nil {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, user_id, method, target_name, request_body, response_status, duration_ms, created_at
			FROM request_logs WHERE user_id = $1
			ORDER BY created_at DESC LIMIT $2 OFFSET $3
		`, userID, limit, offset)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, user_id, method, target_name, request_body, response_status, duration_ms, created_at
			FROM request_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2
		`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		log := &RequestLog{}
		err := rows.Scan(&log.ID, &log.SessionID, &log.UserID, &log.Method,
			&log.TargetName, &log.RequestBody, &log.ResponseStatus, &log.DurationMS, &log.CreatedAt)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// Helper function to check for duplicate key errors
func isDuplicateKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsRune(s, substr))
}

func containsRune(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ==================== Authorization Policy Operations ====================

// CreatePolicy creates a new authorization policy
func (r *Repository) CreatePolicy(ctx context.Context, req *CreatePolicyRequest) (*AuthorizationPolicy, error) {
	policy := &AuthorizationPolicy{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		TargetID:        req.TargetID,
		ResourceType:    req.ResourceType,
		ResourcePattern: req.ResourcePattern,
		Effect:          req.Effect,
		Priority:        req.Priority,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO authorization_policies (id, name, description, target_id, resource_type, resource_pattern, effect, priority, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, policy.ID, policy.Name, policy.Description, policy.TargetID, policy.ResourceType,
		policy.ResourcePattern, policy.Effect, policy.Priority, policy.Enabled, policy.CreatedAt, policy.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Add subjects if provided
	for _, sub := range req.Subjects {
		_, err := r.AddPolicySubject(ctx, policy.ID, &sub)
		if err != nil {
			return nil, err
		}
	}

	return r.GetPolicyByID(ctx, policy.ID)
}

// GetPolicyByID retrieves a policy by ID with its subjects
func (r *Repository) GetPolicyByID(ctx context.Context, id uuid.UUID) (*AuthorizationPolicy, error) {
	policy := &AuthorizationPolicy{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, description, target_id, resource_type, resource_pattern, effect, priority, enabled, created_at, updated_at
		FROM authorization_policies WHERE id = $1
	`, id).Scan(&policy.ID, &policy.Name, &policy.Description, &policy.TargetID, &policy.ResourceType,
		&policy.ResourcePattern, &policy.Effect, &policy.Priority, &policy.Enabled, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Load subjects
	subjects, err := r.GetPolicySubjects(ctx, id)
	if err != nil {
		return nil, err
	}
	policy.Subjects = subjects

	return policy, nil
}

// GetAllPolicies retrieves all policies with their subjects
func (r *Repository) GetAllPolicies(ctx context.Context) ([]*AuthorizationPolicy, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, description, target_id, resource_type, resource_pattern, effect, priority, enabled, created_at, updated_at
		FROM authorization_policies ORDER BY priority DESC, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*AuthorizationPolicy
	for rows.Next() {
		policy := &AuthorizationPolicy{}
		err := rows.Scan(&policy.ID, &policy.Name, &policy.Description, &policy.TargetID, &policy.ResourceType,
			&policy.ResourcePattern, &policy.Effect, &policy.Priority, &policy.Enabled, &policy.CreatedAt, &policy.UpdatedAt)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}

	// Load subjects for each policy
	for _, policy := range policies {
		subjects, err := r.GetPolicySubjects(ctx, policy.ID)
		if err != nil {
			return nil, err
		}
		policy.Subjects = subjects
	}

	return policies, nil
}

// GetPoliciesForTarget retrieves policies for a specific target (and global policies)
func (r *Repository) GetPoliciesForTarget(ctx context.Context, targetID *uuid.UUID) ([]*AuthorizationPolicy, error) {
	var rows pgx.Rows
	var err error

	if targetID != nil {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, name, description, target_id, resource_type, resource_pattern, effect, priority, enabled, created_at, updated_at
			FROM authorization_policies
			WHERE enabled = true AND (target_id IS NULL OR target_id = $1)
			ORDER BY priority DESC
		`, targetID)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, name, description, target_id, resource_type, resource_pattern, effect, priority, enabled, created_at, updated_at
			FROM authorization_policies
			WHERE enabled = true AND target_id IS NULL
			ORDER BY priority DESC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*AuthorizationPolicy
	for rows.Next() {
		policy := &AuthorizationPolicy{}
		err := rows.Scan(&policy.ID, &policy.Name, &policy.Description, &policy.TargetID, &policy.ResourceType,
			&policy.ResourcePattern, &policy.Effect, &policy.Priority, &policy.Enabled, &policy.CreatedAt, &policy.UpdatedAt)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}

	// Load subjects for each policy
	for _, policy := range policies {
		subjects, err := r.GetPolicySubjects(ctx, policy.ID)
		if err != nil {
			return nil, err
		}
		policy.Subjects = subjects
	}

	return policies, nil
}

// UpdatePolicy updates an existing policy
func (r *Repository) UpdatePolicy(ctx context.Context, id uuid.UUID, req *UpdatePolicyRequest) (*AuthorizationPolicy, error) {
	policy, err := r.GetPolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = *req.Description
	}
	if req.TargetID != nil {
		policy.TargetID = req.TargetID
	}
	if req.ResourceType != nil {
		policy.ResourceType = *req.ResourceType
	}
	if req.ResourcePattern != nil {
		policy.ResourcePattern = req.ResourcePattern
	}
	if req.Effect != nil {
		policy.Effect = *req.Effect
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	policy.UpdatedAt = time.Now()

	_, err = r.db.Pool.Exec(ctx, `
		UPDATE authorization_policies
		SET name = $2, description = $3, target_id = $4, resource_type = $5, resource_pattern = $6,
		    effect = $7, priority = $8, enabled = $9, updated_at = $10
		WHERE id = $1
	`, id, policy.Name, policy.Description, policy.TargetID, policy.ResourceType,
		policy.ResourcePattern, policy.Effect, policy.Priority, policy.Enabled, policy.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return policy, nil
}

// DeletePolicy deletes a policy
func (r *Repository) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM authorization_policies WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Policy Subject Operations ====================

// AddPolicySubject adds a subject to a policy
func (r *Repository) AddPolicySubject(ctx context.Context, policyID uuid.UUID, req *CreateSubjectRequest) (*PolicySubject, error) {
	subject := &PolicySubject{
		ID:           uuid.New(),
		PolicyID:     policyID,
		SubjectType:  req.SubjectType,
		SubjectValue: req.SubjectValue,
		CreatedAt:    time.Now(),
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO policy_subjects (id, policy_id, subject_type, subject_value, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (policy_id, subject_type, COALESCE(subject_value, '')) DO NOTHING
	`, subject.ID, subject.PolicyID, subject.SubjectType, subject.SubjectValue, subject.CreatedAt)
	if err != nil {
		return nil, err
	}

	return subject, nil
}

// GetPolicySubjects retrieves all subjects for a policy
func (r *Repository) GetPolicySubjects(ctx context.Context, policyID uuid.UUID) ([]PolicySubject, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, policy_id, subject_type, subject_value, created_at
		FROM policy_subjects WHERE policy_id = $1
	`, policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []PolicySubject
	for rows.Next() {
		var subject PolicySubject
		err := rows.Scan(&subject.ID, &subject.PolicyID, &subject.SubjectType, &subject.SubjectValue, &subject.CreatedAt)
		if err != nil {
			return nil, err
		}
		subjects = append(subjects, subject)
	}

	return subjects, nil
}

// DeletePolicySubject removes a subject from a policy
func (r *Repository) DeletePolicySubject(ctx context.Context, subjectID uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM policy_subjects WHERE id = $1", subjectID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Environment Config Operations ====================

// SetEnvConfig sets or updates an environment config
func (r *Repository) SetEnvConfig(ctx context.Context, targetID uuid.UUID, scopeType string, scopeValue *string, key, encryptedValue string, description *string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO target_env_configs (id, target_id, scope_type, scope_value, env_key, encrypted_value, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (target_id, scope_type, COALESCE(scope_value, ''), env_key) DO UPDATE SET
			encrypted_value = EXCLUDED.encrypted_value,
			description = EXCLUDED.description,
			updated_at = NOW()
	`, uuid.New(), targetID, scopeType, scopeValue, key, encryptedValue, description)
	return err
}

// GetEnvConfigs retrieves env configs for a target by scope
func (r *Repository) GetEnvConfigs(ctx context.Context, targetID uuid.UUID, scopeType string, scopeValue *string) ([]*TargetEnvConfig, error) {
	var rows pgx.Rows
	var err error

	if scopeValue != nil {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, target_id, scope_type, scope_value, env_key, encrypted_value, description, created_at, updated_at
			FROM target_env_configs
			WHERE target_id = $1 AND scope_type = $2 AND scope_value = $3
			ORDER BY env_key
		`, targetID, scopeType, *scopeValue)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, target_id, scope_type, scope_value, env_key, encrypted_value, description, created_at, updated_at
			FROM target_env_configs
			WHERE target_id = $1 AND scope_type = $2 AND scope_value IS NULL
			ORDER BY env_key
		`, targetID, scopeType)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*TargetEnvConfig
	for rows.Next() {
		config := &TargetEnvConfig{}
		err := rows.Scan(&config.ID, &config.TargetID, &config.ScopeType, &config.ScopeValue,
			&config.EnvKey, &config.EncryptedValue, &config.Description, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// GetAllEnvConfigsForTarget retrieves all env configs for a target
func (r *Repository) GetAllEnvConfigsForTarget(ctx context.Context, targetID uuid.UUID) ([]*TargetEnvConfig, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, target_id, scope_type, scope_value, env_key, encrypted_value, description, created_at, updated_at
		FROM target_env_configs
		WHERE target_id = $1
		ORDER BY scope_type, scope_value, env_key
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*TargetEnvConfig
	for rows.Next() {
		config := &TargetEnvConfig{}
		err := rows.Scan(&config.ID, &config.TargetID, &config.ScopeType, &config.ScopeValue,
			&config.EnvKey, &config.EncryptedValue, &config.Description, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// DeleteEnvConfig deletes an environment config
func (r *Repository) DeleteEnvConfig(ctx context.Context, targetID uuid.UUID, scopeType string, scopeValue *string, key string) error {
	if scopeValue != nil {
		result, err := r.db.Pool.Exec(ctx, `
			DELETE FROM target_env_configs
			WHERE target_id = $1 AND scope_type = $2 AND scope_value = $3 AND env_key = $4
		`, targetID, scopeType, *scopeValue, key)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}

	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM target_env_configs
		WHERE target_id = $1 AND scope_type = $2 AND scope_value IS NULL AND env_key = $3
	`, targetID, scopeType, key)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== MCP Instance Operations ====================

// UpsertMCPInstance creates or updates an MCP instance record
func (r *Repository) UpsertMCPInstance(ctx context.Context, targetID uuid.UUID, subjectKey string, pid int) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO mcp_instances (id, target_id, subject_key, pid, status, started_at, last_used_at)
		VALUES ($1, $2, $3, $4, 'running', NOW(), NOW())
		ON CONFLICT (target_id, subject_key) DO UPDATE SET
			pid = EXCLUDED.pid,
			status = 'running',
			started_at = NOW(),
			last_used_at = NOW(),
			stopped_at = NULL
	`, uuid.New(), targetID, subjectKey, pid)
	return err
}

// UpdateMCPInstanceLastUsed updates the last_used_at timestamp
func (r *Repository) UpdateMCPInstanceLastUsed(ctx context.Context, targetID uuid.UUID, subjectKey string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE mcp_instances SET last_used_at = NOW() WHERE target_id = $1 AND subject_key = $2 AND status = 'running'
	`, targetID, subjectKey)
	return err
}

// StopMCPInstance marks an instance as stopped
func (r *Repository) StopMCPInstance(ctx context.Context, targetID uuid.UUID, subjectKey string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE mcp_instances SET status = 'stopped', stopped_at = NOW()
		WHERE target_id = $1 AND subject_key = $2 AND status = 'running'
	`, targetID, subjectKey)
	return err
}

// CleanupMCPInstances marks all running instances as stopped (for gateway restart)
func (r *Repository) CleanupMCPInstances(ctx context.Context) (int64, error) {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE mcp_instances SET status = 'stopped', stopped_at = NOW() WHERE status = 'running'
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ResolveEnvConfigsForTarget resolves all env configs for a user and target
// Priority: User > Group > Role > Default
func (r *Repository) ResolveEnvConfigsForTarget(ctx context.Context, targetID uuid.UUID, userID uuid.UUID, role string, groups []string) (map[string]*EnvConfigInfo, error) {
	configs := make(map[string]*EnvConfigInfo)

	// 1. Load DEFAULT configs (lowest priority)
	defaultConfigs, err := r.GetEnvConfigs(ctx, targetID, "default", nil)
	if err != nil {
		return nil, err
	}
	for _, c := range defaultConfigs {
		desc := ""
		if c.Description != nil {
			desc = *c.Description
		}
		configs[c.EnvKey] = &EnvConfigInfo{
			Key:         c.EnvKey,
			Value:       c.EncryptedValue,
			Source:      "default",
			Description: desc,
		}
	}

	// 2. ROLE configs override defaults
	if role != "" {
		roleConfigs, err := r.GetEnvConfigs(ctx, targetID, "role", &role)
		if err != nil {
			return nil, err
		}
		for _, c := range roleConfigs {
			desc := ""
			if c.Description != nil {
				desc = *c.Description
			}
			configs[c.EnvKey] = &EnvConfigInfo{
				Key:         c.EnvKey,
				Value:       c.EncryptedValue,
				Source:      "role",
				ScopeValue:  role,
				Description: desc,
			}
		}
	}

	// 3. GROUP configs override role (process all groups, later groups can override)
	for _, group := range groups {
		groupConfigs, err := r.GetEnvConfigs(ctx, targetID, "group", &group)
		if err != nil {
			return nil, err
		}
		for _, c := range groupConfigs {
			// Only override if not already set by user
			if existing, exists := configs[c.EnvKey]; !exists || existing.Source != "user" {
				desc := ""
				if c.Description != nil {
					desc = *c.Description
				}
				configs[c.EnvKey] = &EnvConfigInfo{
					Key:         c.EnvKey,
					Value:       c.EncryptedValue,
					Source:      "group",
					ScopeValue:  group,
					Description: desc,
				}
			}
		}
	}

	// 4. USER configs override everything
	userIDStr := userID.String()
	userConfigs, err := r.GetEnvConfigs(ctx, targetID, "user", &userIDStr)
	if err != nil {
		return nil, err
	}
	for _, c := range userConfigs {
		desc := ""
		if c.Description != nil {
			desc = *c.Description
		}
		configs[c.EnvKey] = &EnvConfigInfo{
			Key:         c.EnvKey,
			Value:       c.EncryptedValue,
			Source:      "user",
			ScopeValue:  userIDStr,
			Description: desc,
		}
	}

	return configs, nil
}
