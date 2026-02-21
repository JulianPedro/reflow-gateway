package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/database"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	UserIDKey     contextKey = "user_id"
	UserEmailKey  contextKey = "user_email"
	UserRoleKey   contextKey = "user_role"
	UserGroupsKey contextKey = "user_groups"
	JTIKey        contextKey = "jti"
)

// Middleware provides JWT authentication middleware
type Middleware struct {
	jwtManager *JWTManager
	repo       *database.Repository
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(jwtManager *JWTManager, repo *database.Repository) *Middleware {
	return &Middleware{
		jwtManager: jwtManager,
		repo:       repo,
	}
}

// Authenticate is a middleware that validates JWT tokens
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}
			tokenString = parts[1]
		} else if qToken := r.URL.Query().Get("token"); qToken != "" {
			// Allow token via query param (needed for WebSocket connections)
			tokenString = qToken
		} else {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		claims, err := m.jwtManager.ValidateToken(tokenString)
		if err != nil {
			log.Debug().Err(err).Msg("Token validation failed")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Check if token is revoked
		apiToken, err := m.repo.GetAPITokenByJTI(r.Context(), claims.JTI)
		if err != nil {
			if err == database.ErrNotFound {
				http.Error(w, "Token not found", http.StatusUnauthorized)
				return
			}
			log.Error().Err(err).Msg("Failed to check token status")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if apiToken.RevokedAt != nil {
			http.Error(w, "Token has been revoked", http.StatusUnauthorized)
			return
		}

		// Update last used timestamp (async, don't block request)
		go func() {
			if err := m.repo.UpdateAPITokenLastUsed(context.Background(), claims.JTI); err != nil {
				log.Error().Err(err).Msg("Failed to update token last used")
			}
		}()

		// Add user info to context
		userID, err := claims.GetUserID()
		if err != nil {
			http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		ctx = context.WithValue(ctx, UserGroupsKey, claims.Groups)
		ctx = context.WithValue(ctx, JTIKey, claims.JTI)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID extracts the user ID from the request context
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// GetUserEmail extracts the user email from the request context
func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

// GetUserRole extracts the user role from the request context
func GetUserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(UserRoleKey).(string)
	return role, ok
}

// GetUserGroups extracts the user groups from the request context
func GetUserGroups(ctx context.Context) ([]string, bool) {
	groups, ok := ctx.Value(UserGroupsKey).([]string)
	return groups, ok
}

// GetJTI extracts the JTI from the request context
func GetJTI(ctx context.Context) (string, bool) {
	jti, ok := ctx.Value(JTIKey).(string)
	return jti, ok
}
