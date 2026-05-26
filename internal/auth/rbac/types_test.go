package rbac

import (
	"bufio"
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed model.conf
var modelConf []byte

//go:embed policy_default.csv
var policyDefaultCSV []byte

// TestModelConfMatchesEmbed verifies the embedded model.conf defines a valid
// RBAC-with-domains 4-tuple model with all 5 required sections.
// REQ-AUTH2-001.
func TestModelConfMatchesEmbed(t *testing.T) {
	content := string(modelConf)
	require.NotEmpty(t, content, "embedded model.conf must not be empty")

	// Verify all 5 required sections for RBAC-with-domains model.
	sections := []string{
		"[request_definition]",
		"[policy_definition]",
		"[role_definition]",
		"[policy_effect]",
		"[matchers]",
	}
	for _, section := range sections {
		assert.Contains(t, content, section,
			"model.conf must contain section %s", section)
	}

	// Verify 4-tuple request definition (sub, dom, obj, act).
	assert.Contains(t, content, "r = sub, dom, obj, act",
		"request_definition must use 4-tuple (sub, dom, obj, act)")

	// Verify 3-argument role definition for domains (g = _, _, _).
	assert.Contains(t, content, "g = _, _, _",
		"role_definition must support domains (g = _, _, _)")

	// Verify policy effect includes deny-by-default.
	assert.Contains(t, content, "some(where (p.eft == allow)) && !some(where (p.eft == deny))",
		"policy_effect must implement deny-by-default with explicit deny override")
}

// TestPolicyDefaultCSVMatchesEmbed verifies the embedded policy_default.csv
// contains the V1 default role policies.
// REQ-AUTH2-001, REQ-AUTH2-007.
func TestPolicyDefaultCSVMatchesEmbed(t *testing.T) {
	content := string(policyDefaultCSV)
	require.NotEmpty(t, content, "embedded policy_default.csv must not be empty")

	// Verify admin has wildcard allow.
	assert.Contains(t, content, "p, role_admin, *, *, *, allow",
		"admin role must have wildcard allow policy")

	// Verify member has query and adapter access.
	assert.Contains(t, content, "p, role_member, *, query:basic, read, allow",
		"member role must have query:basic read access")
	assert.Contains(t, content, "p, role_member, *, query:deep, read, allow",
		"member role must have query:deep read access")

	// Verify observer has basic query only.
	assert.Contains(t, content, "p, role_observer, *, query:basic, read, allow",
		"observer role must have query:basic read access")
}

// TestCatchAllDenyRowPresent verifies the last non-comment row in
// policy_default.csv is the catch-all deny.
// REQ-AUTH2-002: deny-by-default safety net.
func TestCatchAllDenyRowPresent(t *testing.T) {
	content := string(policyDefaultCSV)
	scanner := bufio.NewScanner(strings.NewReader(content))

	var lastPolicyLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lastPolicyLine = line
	}

	require.NotEmpty(t, lastPolicyLine, "policy_default.csv must have at least one policy row")
	assert.Equal(t, "p, *, *, *, *, deny", lastPolicyLine,
		"last policy row must be catch-all deny (REQ-AUTH2-002)")
}

// TestCasbinLibraryImportable verifies that the Casbin v2 and pg adapter
// libraries are importable (compile gate).
// REQ-AUTH2-001, D1, D2.
func TestCasbinLibraryImportable(t *testing.T) {
	// This test verifies the libraries are available at compile time.
	// The actual enforcer functionality is tested in Phase B.
	_ = modelConf        // embedded model.conf is available
	_ = policyDefaultCSV // embedded policy_default.csv is available

	// Verify types are usable.
	role := RoleMember
	assert.Equal(t, Role("role_member"), role)

	vis := VisibilityTeamShared
	assert.Equal(t, AdapterVisibility(0), vis)

	decision := Decision{Allowed: true, ReasonClass: "policy_matched"}
	assert.True(t, decision.Allowed)

	// Verify ValidRoles map.
	assert.True(t, ValidRoles[RoleObserver])
	assert.True(t, ValidRoles[RoleMember])
	assert.True(t, ValidRoles[RoleAdmin])
	assert.False(t, ValidRoles[Role("superuser")])
}
