package rbac

import (
	"context"

	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// contextKey is the internal type for RBAC context keys.
type contextKey string

const (
	// TeamIDKey stores the resolved team ID in request context.
	// @MX:NOTE: [AUTO] auth.TeamIDKey is SPEC-AUTH-002 specific; deployment-level tenant
	// uses costguard.TenantIDKey instead. These keys are strictly additive to AUTH-001.
	TeamIDKey contextKey = "auth.team_id"

	// RolesKey stores the user's roles in request context.
	RolesKey contextKey = "auth.roles"
)

// TeamIDFromContext extracts the team ID from the request context.
// Returns empty string if not set.
func TeamIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(TeamIDKey).(string)
	return v
}

// RolesFromContext extracts the user roles from the request context.
// Returns nil if not set.
func RolesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(RolesKey).([]string)
	return v
}

// UserIDFromContext extracts the user ID from the request context.
// REQ-AUTH2-003: Priority is AUTH-001 costguard.UserIDKey (set by JWT middleware).
func UserIDFromContext(ctx context.Context) string {
	if v := costguard.UserIDFromContext(ctx); v != "" {
		return v
	}
	return "anonymous"
}
