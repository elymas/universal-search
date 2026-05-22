package tenancy

import (
	"context"
	"os"
)

// TeamIDKey is the context key for team_id injected by AUTH-001 JWT middleware.
// Convention aligns with costguard.UserIDKey pattern from DEEP-004.
// REQ-IDX4-003
type contextKeyType string

const (
	// TeamIDKey extracts team_id from JWT context (AUTH-001).
	TeamIDKey contextKeyType = "tenancy.team_id"
	// UserIDKey extracts user_id from JWT context (AUTH-001).
	UserIDKey contextKeyType = "tenancy.user_id"
)

// ExtractTeamID extracts team_id from the context, falling back to the
// INDEX_DEFAULT_TEAM environment variable. Returns empty string when both
// are absent (triggers ErrTeamIDRequired in enforced mode).
// REQ-IDX4-003, NFR-IDX4-008
func ExtractTeamID(ctx context.Context) string {
	if v, ok := ctx.Value(TeamIDKey).(string); ok && v != "" {
		return v
	}
	// Fallback: INDEX_DEFAULT_TEAM env var (forward-compat when AUTH-001 not yet shipped).
	if v := defaultTeamFromEnv(); v != "" {
		return v
	}
	return ""
}

// ExtractUserID extracts user_id from the context. Returns empty string when absent.
// REQ-IDX4-003
func ExtractUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// defaultTeamFromEnv reads INDEX_DEFAULT_TEAM once at startup.
// REQ-IDX4-003, NFR-IDX4-008
var defaultTeamFromEnv = func() string {
	return envGetenv("INDEX_DEFAULT_TEAM")
}

// envGetenv is a variable for testing; production uses os.Getenv.
var envGetenv = os.Getenv
