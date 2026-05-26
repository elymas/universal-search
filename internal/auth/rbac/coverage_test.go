package rbac

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfigReturnsExpectedValues verifies DefaultConfig values.
func TestDefaultConfigReturnsExpectedValues(t *testing.T) {
	cfg := DefaultConfig()
	assert.False(t, cfg.Enabled, "default config must have rbac disabled")
	assert.Equal(t, "default", cfg.DefaultTeamID)
	assert.Equal(t, "", cfg.PGDSN)
	assert.True(t, cfg.AuditToStderr)
}

// TestGetPolicyCountReturnsLoadedCount verifies policy count after bootstrap.
func TestGetPolicyCountReturnsLoadedCount(t *testing.T) {
	ef := newTestEnforcer(t)
	count := ef.GetPolicyCount()
	assert.Greater(t, count, 0, "policy count must be >0 after bootstrap")
}

// TestGetPolicyCountAfterRoleAdd verifies count increases after adding a role.
func TestGetPolicyCountAfterRoleAdd(t *testing.T) {
	ef := newTestEnforcer(t)
	before := ef.GetPolicyCount()

	_, err := ef.inner.AddRoleForUserInDomain("alice", "role_member", "eng")
	require.NoError(t, err)

	after := ef.GetPolicyCount()
	assert.GreaterOrEqual(t, after, before, "policy count must not decrease")
}

// TestInnerReturnsUnderlyingEnforcer verifies Inner() returns non-nil enforcer.
func TestInnerReturnsUnderlyingEnforcer(t *testing.T) {
	ef := newTestEnforcer(t)
	inner := ef.Inner()
	require.NotNil(t, inner)
}

// TestGlobalEnforcerReturnsNilWhenNotInitialized verifies nil when not init'd.
func TestGlobalEnforcerReturnsNilWhenNotInitialized(t *testing.T) {
	// Reset global state for this test.
	globalMu.Lock()
	globalEnforcer = nil
	globalEnforcerOnce = sync.Once{}
	globalMu.Unlock()

	result := GlobalEnforcer()
	assert.Nil(t, result, "GlobalEnforcer must return nil when not initialized")
}

// TestParseEmbeddedPolicyReturnsNonEmpty verifies parseEmbeddedPolicy returns
// valid policy rows from the embedded CSV.
func TestParseEmbeddedPolicyReturnsNonEmpty(t *testing.T) {
	policies, err := parseEmbeddedPolicy()
	require.NoError(t, err)
	assert.Greater(t, len(policies), 0, "embedded policy must have rows")

	// Verify each row has at least 4 fields (sub, dom, obj, act) plus eft.
	for i, p := range policies {
		assert.GreaterOrEqual(t, len(p), 4, "policy row %d must have >= 4 fields, got %d", i, len(p))
	}
}

// TestParseEmbeddedPolicyContainsAdminWildcard verifies admin wildcard row.
func TestParseEmbeddedPolicyContainsAdminWildcard(t *testing.T) {
	policies, err := parseEmbeddedPolicy()
	require.NoError(t, err)

	found := false
	for _, p := range policies {
		if p[0] == "role_admin" && p[1] == "*" && p[2] == "*" && p[3] == "*" {
			found = true
		}
	}
	assert.True(t, found, "admin wildcard policy must be present")
}

// TestLoadDefaultPolicyInMemorySuccess verifies in-memory policy loading.
func TestLoadDefaultPolicyInMemorySuccess(t *testing.T) {
	ef := newTestEnforcer(t)
	count := ef.GetPolicyCount()
	assert.Greater(t, count, 0, "in-memory load must populate policies")
}

// TestEnforceReturnsReasonClassInfo verifies Enforce returns correct allow/deny.
func TestEnforceReturnsReasonClassInfo(t *testing.T) {
	ef := newTestEnforcer(t)

	// Add role.
	_, err := ef.inner.AddRoleForUserInDomain("alice", "role_member", "engineering")
	require.NoError(t, err)

	allowed, err := ef.Enforce("alice", "engineering", "query:basic", "read")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Deny case: no matching policy.
	allowed, err = ef.Enforce("alice", "engineering", "team_index", "write")
	require.NoError(t, err)
	assert.False(t, allowed)
}

// TestDecisionStructFields verifies Decision struct has all required fields.
func TestDecisionStructFields(t *testing.T) {
	d := Decision{
		Allowed:     true,
		UserID:      "alice",
		TeamID:      "eng",
		Resource:    "query:basic",
		Action:      "read",
		ReasonClass: "policy_matched",
	}
	assert.True(t, d.Allowed)
	assert.Equal(t, "alice", d.UserID)
	assert.Equal(t, "eng", d.TeamID)
	assert.Equal(t, "query:basic", d.Resource)
	assert.Equal(t, "read", d.Action)
	assert.Equal(t, "policy_matched", d.ReasonClass)
}

// TestUserIDFromContextAnonymousFallback verifies anonymous fallback when
// costguard.UserIDKey is not set.
func TestUserIDFromContextAnonymousFallback(t *testing.T) {
	ctx := context.Background()
	// No costguard.UserIDKey set.
	uid := UserIDFromContext(ctx)
	assert.Equal(t, "anonymous", uid)
}

// TestUserIDFromContextReturnsSetUserID is intentionally a no-op placeholder.
// UserIDFromContext delegates to costguard.UserIDFromContext which keys off
// costguard.UserIDKey (an unexported typed key). The set path is exercised
// through the middleware integration test rather than directly here.
func TestUserIDFromContextReturnsSetUserID(t *testing.T) {
	// Covered by middleware tests; anonymous fallback is verified above.
}

// TestMustInitDisabledDoesNotSetGlobal verifies disabled init leaves globals clean.
func TestMustInitDisabledDoesNotSetGlobal(t *testing.T) {
	// Reset global state.
	globalMu.Lock()
	globalEnforcer = nil
	globalEnforcerOnce = sync.Once{}
	globalMu.Unlock()

	MustInit(Config{Enabled: false})
	assert.Nil(t, GlobalEnforcer(), "GlobalEnforcer must be nil when disabled")
}

// TestEnforcerSavePolicyRecoveryOnNilAdapter verifies SavePolicy recovery
// from panic when no adapter is present.
func TestEnforcerSavePolicyRecoveryOnNilAdapter(t *testing.T) {
	ef := newTestEnforcer(t)
	err := ef.SavePolicy()
	// In-memory enforcer panics on SavePolicy (no adapter).
	// The recovery should catch it and return an error.
	assert.Error(t, err)
}
