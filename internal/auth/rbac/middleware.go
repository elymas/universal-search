package rbac

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// TeamScopeMiddleware extracts identity (user_id, team_id, roles) from the
// request context and headers, then injects them into the context for
// downstream EnforceMiddleware.
//
// REQ-AUTH2-003: Source priority:
//  1. AUTH-001 JWT context (costguard.UserIDKey + TeamIDKey + RolesKey)
//  2. Header fallback (X-User-Id / X-Team-Id / X-Roles)
//  3. Anonymous fallback
//
// REQ-AUTH2-004: Empty team_id handling with default fallback.
//
// @MX:ANCHOR: [AUTO] Identity extraction middleware; callers: all protected routes
// @MX:REASON: AUTH-001 forward-compat contract + empty team_id handling + header fallback
// source-priority depends on this middleware running after AUTH-001 JWT middleware.
func TeamScopeMiddleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract identity using source-priority.
			_, teamID, roles := extractIdentity(ctx, r)

			// REQ-AUTH2-004: Empty team_id handling.
			if teamID == "" {
				if cfg.DefaultTeamID != "" {
					teamID = cfg.DefaultTeamID
				} else {
					// No default -> HTTP 400.
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "team_id_required",
					})
					return
				}
			}

			// Inject into context.
			ctx = context.WithValue(ctx, TeamIDKey, teamID)
			ctx = context.WithValue(ctx, RolesKey, roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractIdentity reads identity from context (AUTH-001 JWT) or headers.
// REQ-AUTH2-003: AUTH-001 context has priority over headers.
// @MX:WARN: [AUTO] Header fallback allows X-Team-Id spoofing in AUTH-001-disabled environments.
// @MX:REASON: In production with AUTH-001 enabled, headers are ignored. Header fallback exists
// for dev environments where AUTH-001 is not configured. Guard with auth-001-ga=true in production.
func extractIdentity(ctx context.Context, r *http.Request) (userID string, teamID string, roles []string) {
	// Priority 1: AUTH-001 JWT context.
	if uid := costguard.UserIDFromContext(ctx); uid != "" && uid != "anonymous" {
		userID = uid
	}

	// TeamID from AUTH-001 context.
	if tid, ok := ctx.Value(TeamIDKey).(string); ok && tid != "" {
		teamID = tid
	}

	// Roles from AUTH-001 context.
	if r, ok := ctx.Value(RolesKey).([]string); ok && len(r) > 0 {
		roles = r
	}

	// Priority 2: Header fallback (when AUTH-001 context is missing).
	if userID == "" || userID == "anonymous" {
		if h := r.Header.Get("X-User-Id"); h != "" {
			userID = h
		}
	}

	if teamID == "" {
		if h := r.Header.Get("X-Team-Id"); h != "" {
			teamID = h
		}
	}

	if len(roles) == 0 {
		if h := r.Header.Get("X-Roles"); h != "" {
			roles = parseRolesHeader(h)
		}
	}

	// Priority 3: Anonymous fallback.
	if userID == "" {
		userID = "anonymous"
	}

	return userID, teamID, roles
}

// parseRolesHeader parses comma-separated roles from header value.
func parseRolesHeader(header string) []string {
	parts := strings.Split(header, ",")
	roles := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			roles = append(roles, p)
		}
	}
	return roles
}

// EnforceMiddleware creates middleware that checks RBAC policy for the given
// resource and action.
// REQ-AUTH2-005: allow -> next.ServeHTTP, deny -> HTTP 403.
func EnforceMiddleware(ef *Enforcer, resource string, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			userID := UserIDFromContext(ctx)
			teamID := TeamIDFromContext(ctx)

			allowed, err := ef.Enforce(userID, teamID, resource, action)
			if err != nil || !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error":    "forbidden",
					"resource": resource,
					"action":   action,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
