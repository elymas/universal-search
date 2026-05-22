package tenancy

import (
	"errors"
	"testing"
)

// Test 1: ParseMode defaults to enforced when env var is not set.
// REQ-IDX4-001
func TestModeParsesEnforcedDefault(t *testing.T) {
	t.Parallel()
	got := ParseMode("")
	if got != ModeEnforced {
		t.Errorf("ParseMode('') = %v, want ModeEnforced", got)
	}
}

// Test 2: ParseMode recognizes "permissive".
func TestModeParsesPermissive(t *testing.T) {
	t.Parallel()
	got := ParseMode("permissive")
	if got != ModePermissive {
		t.Errorf("ParseMode('permissive') = %v, want ModePermissive", got)
	}
}

// Test 3: ParseMode recognizes "legacy".
func TestModeParsesLegacy(t *testing.T) {
	t.Parallel()
	got := ParseMode("legacy")
	if got != ModeLegacy {
		t.Errorf("ParseMode('legacy') = %v, want ModeLegacy", got)
	}
}

// Test 4: ParseMode recognizes "enforced" explicitly.
func TestModeParsesEnforcedExplicit(t *testing.T) {
	t.Parallel()
	got := ParseMode("enforced")
	if got != ModeEnforced {
		t.Errorf("ParseMode('enforced') = %v, want ModeEnforced", got)
	}
}

// Test 5: ParseMode treats unknown values as enforced (safe default).
func TestModeParsesInvalidFallsBackToEnforced(t *testing.T) {
	t.Parallel()
	got := ParseMode("unknown_mode")
	if got != ModeEnforced {
		t.Errorf("ParseMode('unknown_mode') = %v, want ModeEnforced (safe default)", got)
	}
}

// Test 6: ErrTeamIDRequired is a sentinel error that callers can use errors.Is on.
func TestErrTeamIDRequiredIsSentinel(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrTeamIDRequired, ErrTeamIDRequired) {
		t.Error("ErrTeamIDRequired should be identifiable via errors.Is")
	}
	if ErrTeamIDRequired.Error() == "" {
		t.Error("ErrTeamIDRequired.Error() should return non-empty string")
	}
}
