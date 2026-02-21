package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/database"
)

// Router creates and configures the API router
func Router(repo *database.Repository, jwtManager *auth.JWTManager, encryptor *auth.TokenEncryptor, authMiddleware *auth.Middleware, sessionRecycler SessionRecycler, instanceRestarter InstanceRestarter) chi.Router {
	r := chi.NewRouter()

	h := NewHandlers(repo, jwtManager, encryptor, sessionRecycler, instanceRestarter)
	policyHandlers := NewPolicyHandlers(repo, encryptor)
	envHandlers := NewEnvHandlers(repo, encryptor, instanceRestarter)

	// Public routes (no auth required)
	r.Group(func(r chi.Router) {
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
	})

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// User auth routes
		r.Get("/auth/me", h.GetCurrentUser)
		r.Get("/auth/tokens", h.ListAPITokens)
		r.Post("/auth/tokens", h.CreateAPIToken)
		r.Delete("/auth/tokens/{id}", h.RevokeAPIToken)

		// User management routes (admin only)
		r.Get("/users", h.ListUsers)
		r.Put("/users/{id}", h.UpdateUser)
		r.Post("/users/{id}/recycle", h.RecycleSessions)

		// Session recycle (own sessions)
		r.Post("/sessions/recycle", h.RecycleSessions)

		// Target routes
		r.Get("/targets", h.ListTargets)
		r.Post("/targets", h.CreateTarget)
		r.Get("/targets/{id}", h.GetTarget)
		r.Put("/targets/{id}", h.UpdateTarget)
		r.Delete("/targets/{id}", h.DeleteTarget)
		r.Post("/targets/{id}/restart-instances", h.RestartInstances)

		// Target token configuration (view all tokens for a target)
		r.Get("/targets/{id}/tokens", h.GetTargetTokenConfig)

		// User target token routes (current user)
		r.Get("/targets/{id}/token", h.GetUserTargetToken)
		r.Put("/targets/{id}/token", h.SetUserTargetToken)
		r.Delete("/targets/{id}/token", h.DeleteUserTargetToken)

		// Role target token routes (admin only)
		r.Put("/targets/{id}/tokens/role", h.SetRoleTargetToken)
		r.Delete("/targets/{id}/tokens/role/{role}", h.DeleteRoleTargetToken)

		// Group target token routes (admin only)
		r.Put("/targets/{id}/tokens/group", h.SetGroupTargetToken)
		r.Delete("/targets/{id}/tokens/group/{group}", h.DeleteGroupTargetToken)

		// Default target token routes (admin only)
		r.Put("/targets/{id}/tokens/default", h.SetDefaultTargetToken)
		r.Delete("/targets/{id}/tokens/default", h.DeleteDefaultTargetToken)

		// Logs routes
		r.Get("/logs", h.ListRequestLogs)

		// Authorization policies routes
		r.Get("/policies", policyHandlers.ListPolicies)
		r.Post("/policies", policyHandlers.CreatePolicy)
		r.Get("/policies/{id}", policyHandlers.GetPolicy)
		r.Put("/policies/{id}", policyHandlers.UpdatePolicy)
		r.Delete("/policies/{id}", policyHandlers.DeletePolicy)
		r.Post("/policies/{id}/subjects", policyHandlers.AddPolicySubject)
		r.Delete("/policies/{id}/subjects/{subjectId}", policyHandlers.DeletePolicySubject)

		// Environment configuration routes
		r.Get("/targets/{id}/env", envHandlers.ListEnvConfigs)
		r.Get("/targets/{id}/env/resolve", envHandlers.ResolveEnvConfigs)

		// Default env configs
		r.Get("/targets/{id}/env/default", envHandlers.GetEnvConfigsByScope)
		r.Put("/targets/{id}/env/default", envHandlers.BulkSetEnvConfigs)
		r.Post("/targets/{id}/env/default", envHandlers.SetEnvConfig)
		r.Delete("/targets/{id}/env/default/{key}", envHandlers.DeleteEnvConfig)

		// Role env configs
		r.Get("/targets/{id}/env/role/{scopeValue}", envHandlers.GetEnvConfigsByScope)
		r.Put("/targets/{id}/env/role/{scopeValue}", envHandlers.BulkSetEnvConfigs)
		r.Post("/targets/{id}/env/role/{scopeValue}", envHandlers.SetEnvConfig)
		r.Delete("/targets/{id}/env/role/{scopeValue}/{key}", envHandlers.DeleteEnvConfig)

		// Group env configs
		r.Get("/targets/{id}/env/group/{scopeValue}", envHandlers.GetEnvConfigsByScope)
		r.Put("/targets/{id}/env/group/{scopeValue}", envHandlers.BulkSetEnvConfigs)
		r.Post("/targets/{id}/env/group/{scopeValue}", envHandlers.SetEnvConfig)
		r.Delete("/targets/{id}/env/group/{scopeValue}/{key}", envHandlers.DeleteEnvConfig)

		// User env configs
		r.Get("/targets/{id}/env/user/{scopeValue}", envHandlers.GetEnvConfigsByScope)
		r.Put("/targets/{id}/env/user/{scopeValue}", envHandlers.BulkSetEnvConfigs)
		r.Post("/targets/{id}/env/user/{scopeValue}", envHandlers.SetEnvConfig)
		r.Delete("/targets/{id}/env/user/{scopeValue}/{key}", envHandlers.DeleteEnvConfig)
	})

	return r
}
