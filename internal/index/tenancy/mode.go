// Package tenancy provides multi-tenancy enforcement for the shared index.
// SPEC-IDX-004: Three modes — enforced (v1.0 default), permissive, legacy.
// Mode transition requires process restart; no hot-reload support.
package tenancy

import "errors"

// Mode represents the multi-tenancy enforcement mode.
// REQ-IDX4-001: When INDEX_MULTI_TENANCY_MODE is not set, defaults to enforced.
type Mode int

const (
	// ModeEnforced is the v1.0 default. Rejects TeamID == "" with ErrTeamIDRequired.
	ModeEnforced Mode = iota
	// ModePermissive allows NULL team_id for backward compatibility.
	ModePermissive
	// ModeLegacy preserves v0.1 behavior (team_id completely ignored).
	ModeLegacy
)

// ErrTeamIDRequired is the sentinel error returned when TeamID is empty in enforced mode.
// REQ-IDX4-001: Embedder and store fanout are skipped when this error is returned.
var ErrTeamIDRequired = errors.New("tenancy: team_id is required in enforced mode")

// ParseMode parses the INDEX_MULTI_TENANCY_MODE env var value into a Mode.
// Empty string or "enforced" returns ModeEnforced (v1.0 default).
// Unknown values fall back to ModeEnforced (safe default).
//
// @MX:ANCHOR: [AUTO] ParseMode — fan_in >= 3 (dispatch.go Search, dispatch.go Upsert, cmd/usearch/main.go startup)
// @MX:REASON: tenancy mode semantics are defined here; all enforcement paths depend on this function
// @MX:SPEC: SPEC-IDX-004
func ParseMode(env string) Mode {
	switch env {
	case "permissive":
		return ModePermissive
	case "legacy":
		return ModeLegacy
	default:
		return ModeEnforced
	}
}
