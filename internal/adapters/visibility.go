package adapters

// AdapterVisibility enumerates per-adapter visibility levels.
// REQ-AUTH2-007: V1 adapters are team_shared.
// REQ-AUTH2-008: V1.1 reserves personal visibility for SPEC-AUTH-005.
type AdapterVisibility int

const (
	// VisibilityTeamShared means the adapter is accessible to all team members.
	VisibilityTeamShared AdapterVisibility = iota
	// VisibilityPersonal means the adapter is scoped to a specific owner.
	// V1.1 reserve: no personal adapters in V1.
	VisibilityPersonal
	// VisibilityAdminOnly means the adapter is restricted to admin role.
	VisibilityAdminOnly
)

// MakePersonalPolicyRow generates a policy row for a personal adapter.
// V1.1 reserve: this helper is defined but no V1 code calls it.
// REQ-AUTH2-008: Policy shape p, <owner>, <team>, adapter:<name>:<owner>, read, allow.
// @MX:TODO: [AUTO] V1.1 SPEC-AUTH-005 will call this when OAuth + per-user policy generation activates.
func MakePersonalPolicyRow(owner, team, name string) []string {
	return []string{owner, team, "adapter:" + name + ":" + owner, "read", "allow"}
}
