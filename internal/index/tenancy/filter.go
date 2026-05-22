package tenancy

import (
	"fmt"

	pb "github.com/qdrant/go-client/qdrant"
)

// TenancyFilter builds a complete Qdrant Filter for multi-tenancy enforcement.
// REQ-IDX4-006: must clause for team_id, optional user_id should, optional __public__ should.
// Returns nil when teamID is empty (no tenancy filter needed).
//
// @MX:WARN: [AUTO] Filter expression omission causes cross-team data exposure.
// @MX:REASON: NFR-IDX4-001 (0 leak) load-bearing — every query path MUST include team_id must clause.
func TenancyFilter(teamID, userID string, includePublic bool) (*pb.Filter, error) {
	if teamID == "" {
		return nil, nil
	}

	// Reject __public__ as user input (REQ-IDX4-007).
	if IsPublicSentinel(teamID) {
		return nil, fmt.Errorf("tenancy: %q sentinel cannot be used as query team_id", PublicSentinel)
	}

	var must []*pb.Condition
	var should []*pb.Condition

	// Team ID: always in must (or should if includePublic).
	if includePublic {
		// should: [team_id == $T, team_id == "__public__"]
		should = append(should,
			pb.NewMatch("team_id", teamID),
			pb.NewMatch("team_id", PublicSentinel),
		)
	} else {
		must = append(must, pb.NewMatch("team_id", teamID))
	}

	// User-private visibility (REQ-IDX4-006).
	// team_id = $T AND (user_id = $U OR user_id = "")
	if userID != "" {
		must = append(must, pb.NewFilterAsCondition(&pb.Filter{
			Should: []*pb.Condition{
				pb.NewMatch("user_id", userID),
				pb.NewMatch("user_id", ""),
			},
		}))
	}

	return &pb.Filter{
		Must:   must,
		Should: should,
	}, nil
}

// BuildMeiliTenancyFilter builds a Meilisearch filter string for multi-tenancy.
// REQ-IDX4-006
func BuildMeiliTenancyFilter(teamID, userID string, includePublic bool) (string, error) {
	if teamID == "" {
		return "", nil
	}
	if IsPublicSentinel(teamID) {
		return "", fmt.Errorf("tenancy: %q sentinel cannot be used as query team_id", PublicSentinel)
	}

	var parts []string

	if includePublic {
		parts = append(parts, fmt.Sprintf("(team_id = \"%s\" OR team_id = \"__public__\")", teamID))
	} else {
		parts = append(parts, fmt.Sprintf("team_id = \"%s\"", teamID))
	}

	if userID != "" {
		parts = append(parts, fmt.Sprintf("(user_id = \"%s\" OR user_id = \"\")", userID))
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " AND " + parts[i]
	}
	return result, nil
}
