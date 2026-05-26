package tenancy

import (
	"context"
	"os"
	"testing"
)

// Test 7: ExtractTeamID returns team_id from JWT context key (AUTH-001).
// REQ-IDX4-003
func TestExtractTeamIDFromJWTContext(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), TeamIDKey, "team-T")
	got := ExtractTeamID(ctx)
	if got != "team-T" {
		t.Errorf("ExtractTeamID(ctx with TeamIDKey='team-T') = %q, want 'team-T'", got)
	}
}

// Test 8: ExtractTeamID falls back to INDEX_DEFAULT_TEAM env var.
// REQ-IDX4-003, NFR-IDX4-008
func TestExtractTeamIDFallsBackToEnvVar(t *testing.T) {
	// Cannot use t.Parallel: tests in this file mutate the shared
	// INDEX_DEFAULT_TEAM env var and would race on CI.
	t.Setenv("INDEX_DEFAULT_TEAM", "default-team")

	ctx := context.Background()
	got := ExtractTeamID(ctx)
	if got != "default-team" {
		t.Errorf("ExtractTeamID(empty ctx) = %q, want 'default-team' from env", got)
	}
}

// Test 9: ExtractTeamID returns empty when both context and env var are missing.
// REQ-IDX4-003
func TestExtractTeamIDReturnsEmptyOnMissingBoth(t *testing.T) {
	// Cannot use t.Parallel: depends on INDEX_DEFAULT_TEAM being unset
	// while sibling tests mutate it.
	if prev, ok := os.LookupEnv("INDEX_DEFAULT_TEAM"); ok {
		os.Unsetenv("INDEX_DEFAULT_TEAM")
		t.Cleanup(func() { os.Setenv("INDEX_DEFAULT_TEAM", prev) })
	}
	ctx := context.Background()
	got := ExtractTeamID(ctx)
	if got != "" {
		t.Errorf("ExtractTeamID(nothing) = %q, want empty string", got)
	}
}

// Test 10: JWT context takes precedence over env var.
func TestExtractTeamIDJWTTakesPrecedence(t *testing.T) {
	// Cannot use t.Parallel: shares INDEX_DEFAULT_TEAM with sibling tests.
	t.Setenv("INDEX_DEFAULT_TEAM", "env-team")

	ctx := context.WithValue(context.Background(), TeamIDKey, "jwt-team")
	got := ExtractTeamID(ctx)
	if got != "jwt-team" {
		t.Errorf("ExtractTeamID(JWT+env) = %q, want 'jwt-team' (JWT wins)", got)
	}
}

// Test 11: ExtractUserID returns user_id from JWT context key.
func TestExtractUserIDFromJWTContext(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), UserIDKey, "alice@example.com")
	got := ExtractUserID(ctx)
	if got != "alice@example.com" {
		t.Errorf("ExtractUserID(ctx with UserIDKey) = %q, want 'alice@example.com'", got)
	}
}

// Test 12: ExtractUserID returns empty when context has no user_id.
func TestExtractUserIDReturnsEmptyOnMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := ExtractUserID(ctx)
	if got != "" {
		t.Errorf("ExtractUserID(empty ctx) = %q, want empty string", got)
	}
}
