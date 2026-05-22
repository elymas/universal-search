package tenancy

import (
	"context"
	"testing"
)

// Test 13: EnforceSearch rejects empty team_id in enforced mode.
func TestEnforceSearchRejectsEmptyTeamIDInEnforcedMode(t *testing.T) {
	t.Parallel()
	err := EnforceSearch(ModeEnforced, "")
	if err == nil {
		t.Error("EnforceSearch(enforced, '') should return ErrTeamIDRequired")
	}
}

// Test 14: EnforceSearch allows non-empty team_id in enforced mode.
func TestEnforceSearchAllowsNonEmptyTeamID(t *testing.T) {
	t.Parallel()
	err := EnforceSearch(ModeEnforced, "team-T")
	if err != nil {
		t.Errorf("EnforceSearch(enforced, 'team-T') returned unexpected error: %v", err)
	}
}

// Test 15: EnforceSearch allows empty team_id in permissive mode.
func TestEnforceSearchPermissiveAllowsNullTeamID(t *testing.T) {
	t.Parallel()
	err := EnforceSearch(ModePermissive, "")
	if err != nil {
		t.Errorf("EnforceSearch(permissive, '') returned unexpected error: %v", err)
	}
}

// Test 16: EnforceSearch allows empty team_id in legacy mode.
func TestEnforceSearchLegacyPreservesV01Behavior(t *testing.T) {
	t.Parallel()
	err := EnforceSearch(ModeLegacy, "")
	if err != nil {
		t.Errorf("EnforceSearch(legacy, '') returned unexpected error: %v", err)
	}
}

// Test 17: InjectTeamID overwrites caller-provided team_id.
func TestInjectTeamIDIgnoresCallerProvidedTeamID(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), TeamIDKey, "team-T")
	meta := map[string]any{"team_id": "team-EVIL"}
	teamID, err := InjectTeamID(ctx, ModeEnforced, meta)
	if err != nil {
		t.Fatalf("InjectTeamID returned error: %v", err)
	}
	if teamID != "team-T" {
		t.Errorf("InjectTeamID returned %q, want 'team-T'", teamID)
	}
	if meta["team_id"] != "team-T" {
		t.Errorf("metadata['team_id'] = %v, want 'team-T'", meta["team_id"])
	}
}

// Test 18: InjectTeamID uses context team_id when metadata has no team_id.
func TestInjectTeamIDUsesContextTeamID(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), TeamIDKey, "team-T")
	meta := map[string]any{}
	teamID, err := InjectTeamID(ctx, ModeEnforced, meta)
	if err != nil {
		t.Fatalf("InjectTeamID returned error: %v", err)
	}
	if teamID != "team-T" {
		t.Errorf("InjectTeamID returned %q, want 'team-T'", teamID)
	}
	if meta["team_id"] != "team-T" {
		t.Errorf("metadata['team_id'] = %v, want 'team-T'", meta["team_id"])
	}
}

// Test 19: InjectTeamID returns ErrTeamIDRequired when no team_id available in enforced mode.
func TestInjectTeamIDRejectsEmptyInEnforcedMode(t *testing.T) {
	// Clear env var to ensure no fallback.
	orig := envGetenv
	envGetenv = func(key string) string { return "" }
	defer func() { envGetenv = orig }()

	ctx := context.Background()
	meta := map[string]any{}
	_, err := InjectTeamID(ctx, ModeEnforced, meta)
	if err == nil {
		t.Error("InjectTeamID(enforced, no team) should return ErrTeamIDRequired")
	}
}

// Test 20: InjectUserID adds user_id for user_private visibility.
func TestInjectUserIDSetsUserIDForPrivateVisibility(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), UserIDKey, "alice@example.com")
	meta := map[string]any{}
	InjectUserID(ctx, VisibilityUserPrivate, meta)
	if meta["user_id"] != "alice@example.com" {
		t.Errorf("metadata['user_id'] = %v, want 'alice@example.com'", meta["user_id"])
	}
}

// Test 21: InjectUserID does not add user_id for team_shared visibility.
func TestInjectUserIDSkipsForTeamShared(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), UserIDKey, "alice@example.com")
	meta := map[string]any{}
	InjectUserID(ctx, VisibilityTeamShared, meta)
	if _, ok := meta["user_id"]; ok {
		t.Error("metadata should not have 'user_id' for team_shared visibility")
	}
}

// Test 22: ValidateStartupTeamID rejects __public__.
func TestValidateStartupTeamIDRejectsPublicSentinel(t *testing.T) {
	t.Parallel()
	err := ValidateStartupTeamID("__public__")
	if err == nil {
		t.Error("ValidateStartupTeamID('__public__') should return error")
	}
}

// Test 23: ValidateStartupTeamID accepts normal team ID.
func TestValidateStartupTeamIDAcceptsNormal(t *testing.T) {
	t.Parallel()
	err := ValidateStartupTeamID("default")
	if err != nil {
		t.Errorf("ValidateStartupTeamID('default') returned unexpected error: %v", err)
	}
}
