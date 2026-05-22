package tenancy

// Visibility represents the document visibility level.
// D4: team_shared (default), user_private, public.
type Visibility int

const (
	// VisibilityTeamShared is the default: visible to all team members.
	VisibilityTeamShared Visibility = iota
	// VisibilityUserPrivate is visible only to the user who ingested it.
	VisibilityUserPrivate
	// VisibilityPublic uses the __public__ sentinel for cross-team access.
	VisibilityPublic
)

// String returns the string representation of the visibility.
func (v Visibility) String() string {
	switch v {
	case VisibilityUserPrivate:
		return "user_private"
	case VisibilityPublic:
		return "public"
	default:
		return "team_shared"
	}
}

// VisibilityFromString parses a visibility string into a Visibility value.
// Returns VisibilityTeamShared for unknown values (safe default).
func VisibilityFromString(s string) Visibility {
	switch s {
	case "user_private":
		return VisibilityUserPrivate
	case "public":
		return VisibilityPublic
	default:
		return VisibilityTeamShared
	}
}
