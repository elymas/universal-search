package tenancy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PublicSentinel is the reserved team_id value for public documents.
// REQ-IDX4-007: Rejected at 4 entry points (JWT claim, env var, backfill CLI, tier-promote CLI).
// Accepted ONLY as Adapter.Visibility() = public result.
const PublicSentinel = "__public__"

// IsPublicSentinel returns true if the given team_id is the reserved public sentinel.
func IsPublicSentinel(teamID string) bool {
	return teamID == PublicSentinel
}

// ValidateTeamID rejects the __public__ sentinel value for user-provided team_id.
// REQ-IDX4-007
func ValidateTeamID(teamID string) error {
	if IsPublicSentinel(teamID) {
		return fmt.Errorf("tenancy: %q is a reserved sentinel and cannot be used as team_id", PublicSentinel)
	}
	return nil
}

// HashTeamID returns SHA-256[:8] hex of the team_id for observability labels.
// NFR-IDX4-007: Plain team_id MUST NOT appear in labels/span attributes.
// __public__ is kept as plaintext per D8.
func HashTeamID(teamID string) string {
	if IsPublicSentinel(teamID) {
		return PublicSentinel
	}
	h := sha256.Sum256([]byte(teamID))
	return hex.EncodeToString(h[:])[:8]
}
