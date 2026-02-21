package gateway

import (
	"context"
	"regexp"
	"sync"

	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/telemetry"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Authorizer handles authorization policy evaluation
type Authorizer struct {
	repo        *database.Repository
	policyCache map[string][]*database.AuthorizationPolicy
	cacheMu     sync.RWMutex
}

// NewAuthorizer creates a new authorizer
func NewAuthorizer(repo *database.Repository) *Authorizer {
	return &Authorizer{
		repo:        repo,
		policyCache: make(map[string][]*database.AuthorizationPolicy),
	}
}

// CanAccessTarget checks if a user can access a specific target
func (a *Authorizer) CanAccessTarget(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID uuid.UUID) (bool, string, error) {
	return a.CanAccess(ctx, userID, role, groups, &targetID, "all", "")
}

// CanAccess checks if a user can access a specific resource
func (a *Authorizer) CanAccess(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID *uuid.UUID, resourceType, resourceName string) (bool, string, error) {
	ctx, span := tracer.Start(ctx, "Authorizer.CanAccess",
		trace.WithAttributes(
			attribute.String("authz.resource_type", resourceType),
			attribute.String("authz.resource_name", resourceName),
		),
	)
	defer span.End()

	// Load policies for target (and global policies)
	policies, err := a.loadPolicies(ctx, targetID)
	if err != nil {
		return false, "", err
	}

	// Sort by priority (already sorted in query)
	for _, policy := range policies {
		// Check if user matches any subject
		if !a.matchesSubject(policy, userID, role, groups) {
			continue
		}

		// Check resource type
		if policy.ResourceType != "all" && policy.ResourceType != resourceType {
			continue
		}

		// Check resource pattern
		if policy.ResourcePattern != nil && *policy.ResourcePattern != "" && resourceName != "" {
			matched, err := regexp.MatchString(*policy.ResourcePattern, resourceName)
			if err != nil || !matched {
				continue
			}
		}

		// First matching policy determines outcome
		allowed := policy.Effect == "allow"
		decision := "deny"
		if allowed {
			decision = "allow"
		}

		span.SetAttributes(
			attribute.String("authz.decision", decision),
			attribute.String("authz.policy_name", policy.Name),
		)
		telemetry.MCPAuthzDecisionsTotal.Add(ctx, 1,
			otelmetric.WithAttributes(
				attribute.String("decision", decision),
				attribute.String("resource_type", resourceType),
			),
		)

		log.Debug().
			Str("policy", policy.Name).
			Bool("allowed", allowed).
			Str("resource_type", resourceType).
			Str("resource_name", resourceName).
			Msg("Authorization policy matched")

		return allowed, policy.Name, nil
	}

	// Default deny if no policy matches
	span.SetAttributes(attribute.String("authz.decision", "deny"))
	telemetry.MCPAuthzDecisionsTotal.Add(ctx, 1,
		otelmetric.WithAttributes(
			attribute.String("decision", "deny"),
			attribute.String("resource_type", resourceType),
		),
	)

	log.Debug().
		Str("user_id", userID.String()).
		Str("resource_type", resourceType).
		Str("resource_name", resourceName).
		Msg("No authorization policy matched, defaulting to deny")

	return false, "", nil
}

// loadPolicies loads policies for a target (and global policies)
func (a *Authorizer) loadPolicies(ctx context.Context, targetID *uuid.UUID) ([]*database.AuthorizationPolicy, error) {
	// Try cache first
	cacheKey := "global"
	if targetID != nil {
		cacheKey = targetID.String()
	}

	a.cacheMu.RLock()
	if cached, ok := a.policyCache[cacheKey]; ok {
		a.cacheMu.RUnlock()
		return cached, nil
	}
	a.cacheMu.RUnlock()

	// Load from database
	policies, err := a.repo.GetPoliciesForTarget(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	a.cacheMu.Lock()
	a.policyCache[cacheKey] = policies
	a.cacheMu.Unlock()

	return policies, nil
}

// matchesSubject checks if a user matches any of the policy's subjects
func (a *Authorizer) matchesSubject(policy *database.AuthorizationPolicy, userID uuid.UUID, role string, groups []string) bool {
	for _, subject := range policy.Subjects {
		switch subject.SubjectType {
		case "everyone":
			return true
		case "user":
			if subject.SubjectValue != nil && *subject.SubjectValue == userID.String() {
				return true
			}
		case "role":
			if subject.SubjectValue != nil && *subject.SubjectValue == role {
				return true
			}
		case "group":
			if subject.SubjectValue != nil {
				for _, g := range groups {
					if g == *subject.SubjectValue {
						return true
					}
				}
			}
		}
	}
	return false
}

// InvalidateCache clears the policy cache
func (a *Authorizer) InvalidateCache() {
	a.cacheMu.Lock()
	a.policyCache = make(map[string][]*database.AuthorizationPolicy)
	a.cacheMu.Unlock()
}

// InvalidateCacheForTarget clears the cache for a specific target
func (a *Authorizer) InvalidateCacheForTarget(targetID *uuid.UUID) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	if targetID != nil {
		delete(a.policyCache, targetID.String())
	}
	// Also invalidate global cache as it might be affected
	delete(a.policyCache, "global")
}

// FilterTools filters tools based on authorization
func (a *Authorizer) FilterTools(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID uuid.UUID, tools []string) []string {
	var allowed []string
	for _, tool := range tools {
		canAccess, _, err := a.CanAccess(ctx, userID, role, groups, &targetID, "tool", tool)
		if err == nil && canAccess {
			allowed = append(allowed, tool)
		}
	}
	return allowed
}

// FilterResources filters resources based on authorization
func (a *Authorizer) FilterResources(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID uuid.UUID, resources []string) []string {
	var allowed []string
	for _, resource := range resources {
		canAccess, _, err := a.CanAccess(ctx, userID, role, groups, &targetID, "resource", resource)
		if err == nil && canAccess {
			allowed = append(allowed, resource)
		}
	}
	return allowed
}

// FilterPrompts filters prompts based on authorization
func (a *Authorizer) FilterPrompts(ctx context.Context, userID uuid.UUID, role string, groups []string, targetID uuid.UUID, prompts []string) []string {
	var allowed []string
	for _, prompt := range prompts {
		canAccess, _, err := a.CanAccess(ctx, userID, role, groups, &targetID, "prompt", prompt)
		if err == nil && canAccess {
			allowed = append(allowed, prompt)
		}
	}
	return allowed
}
