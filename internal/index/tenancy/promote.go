package tenancy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// DedicatedCollectionName generates the dedicated collection name for a team.
// REQ-IDX4-005: usearch_docs__team_<sha256(team_id)[:16]>
func DedicatedCollectionName(teamID string) string {
	h := sha256.Sum256([]byte(teamID))
	return fmt.Sprintf("usearch_docs__team_%s", hex.EncodeToString(h[:])[:16])
}

// TierConfig holds the tiering configuration for Qdrant collections.
type TierConfig struct {
	// DedicatedTeams is the manual list of teams that should use dedicated collections.
	// v1.0 manual list only; doc-count auto-tier deferred to SPEC-IDX-007.
	DedicatedTeams []string
}

// IsDedicatedTeam checks if a team should use a dedicated collection.
func (tc *TierConfig) IsDedicatedTeam(teamID string) bool {
	for _, t := range tc.DedicatedTeams {
		if t == teamID {
			return true
		}
	}
	return false
}

// CollectionForTeam returns the collection name to use for a given team.
// Returns the dedicated collection name if the team is in the dedicated list,
// otherwise returns the default collection name.
func (tc *TierConfig) CollectionForTeam(teamID, defaultCollection string) string {
	if tc.IsDedicatedTeam(teamID) {
		return DedicatedCollectionName(teamID)
	}
	return defaultCollection
}
