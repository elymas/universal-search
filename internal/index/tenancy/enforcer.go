package tenancy

import (
	"context"
	"fmt"
	"log/slog"
)

// EnforceSearch checks tenancy enforcement for Search operations.
// In enforced mode, returns ErrTeamIDRequired when team_id is empty.
// In permissive/legacy modes, always returns nil.
// REQ-IDX4-001
func EnforceSearch(mode Mode, teamID string) error {
	if mode == ModeEnforced && teamID == "" {
		return ErrTeamIDRequired
	}
	return nil
}

// InjectTeamID extracts team_id from context and silently overwrites any
// caller-provided Metadata["team_id"]. Emits WARN slog when override occurs.
// REQ-IDX4-002
func InjectTeamID(ctx context.Context, mode Mode, docMetadata map[string]any) (teamID string, err error) {
	teamID = ExtractTeamID(ctx)

	if mode == ModeEnforced && teamID == "" {
		return "", ErrTeamIDRequired
	}

	// If team_id is empty in non-enforced mode, use empty (backward compat).
	if teamID == "" {
		return "", nil
	}

	// Silent overwrite: check if caller tried to set team_id.
	if docMetadata != nil {
		if existing, ok := docMetadata["team_id"].(string); ok && existing != "" && existing != teamID {
			// Emit WARN slog per REQ-IDX4-002.
			slog.Warn("idx4.upsert.team_id_overridden",
				"event_type", "idx4.upsert.team_id_overridden",
				"caller_team_id", existing,
				"ctx_team_id", teamID,
			)
		}
		if docMetadata != nil {
			docMetadata["team_id"] = teamID
		}
	}

	return teamID, nil
}

// InjectUserID adds user_id to metadata when visibility is user_private.
// REQ-IDX4-002
func InjectUserID(ctx context.Context, visibility Visibility, docMetadata map[string]any) {
	if visibility != VisibilityUserPrivate {
		return
	}
	userID := ExtractUserID(ctx)
	if userID != "" && docMetadata != nil {
		docMetadata["user_id"] = userID
	}
}

// ValidateStartupTeamID validates the INDEX_DEFAULT_TEAM env var at startup.
// Rejects __public__ sentinel per REQ-IDX4-007.
func ValidateStartupTeamID(teamID string) error {
	if err := ValidateTeamID(teamID); err != nil {
		return fmt.Errorf("startup validation: %w", err)
	}
	return nil
}
