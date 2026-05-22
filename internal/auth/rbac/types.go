// Package rbac provides team-scoped RBAC authorization via Casbin.
// SPEC-AUTH-002: RBAC-with-domains model, team-scoped queries, per-adapter visibility.
package rbac

// Role enumerates the V1 RBAC roles (D4: observer < member < admin).
type Role string

const (
	// RoleObserver grants read-only access to basic queries.
	RoleObserver Role = "role_observer"
	// RoleMember grants query + adapter read access.
	RoleMember Role = "role_member"
	// RoleAdmin grants full access to all resources and actions.
	RoleAdmin Role = "role_admin"
)

// ValidRoles is the set of valid role values for input validation.
var ValidRoles = map[Role]bool{
	RoleObserver: true,
	RoleMember:   true,
	RoleAdmin:    true,
}

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

// Decision represents the result of an RBAC policy evaluation.
type Decision struct {
	Allowed     bool
	UserID      string
	TeamID      string
	Resource    string
	Action      string
	ReasonClass string // policy_matched, no_policy_matched, explicit_deny, empty_team
}
