package rbac

import (
	"fmt"
	"sync"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestEnforcer creates an in-memory enforcer with the embedded model
// and policy for unit testing (no PG dependency).
func newTestEnforcer(t *testing.T) *Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(string(embeddedModel))
	require.NoError(t, err, "model must parse")

	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err, "enforcer must create")

	// Load default policy from embedded CSV (in-memory, no SavePolicy).
	err = loadDefaultPolicyInMemory(e)
	require.NoError(t, err, "default policy must load")

	return &Enforcer{inner: e}
}

// loadDefaultPolicyInMemory loads the embedded CSV into the enforcer without
// persisting to PG (for unit tests).
func loadDefaultPolicyInMemory(e *casbin.Enforcer) error {
	policies, err := parseEmbeddedPolicy()
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return nil
	}
	_, err = e.AddPolicies(policies)
	return err
}

// TestEnforcerInitFromEmbeddedModel verifies that an enforcer can be created
// from the embedded model.conf with the RBAC-with-domains 4-tuple model.
// REQ-AUTH2-001.
func TestEnforcerInitFromEmbeddedModel(t *testing.T) {
	ef := newTestEnforcer(t)
	require.NotNil(t, ef)

	policies, err := ef.inner.GetPolicy()
	require.NoError(t, err)
	assert.Greater(t, len(policies), 0, "enforcer must have policies after bootstrap")
}

// TestEnforcerInitFatalExitOnFailure verifies that MustInit panics when
// RBAC is enabled but init fails (empty DSN).
// REQ-AUTH2-001.
func TestEnforcerInitFatalExitOnFailure(t *testing.T) {
	assert.Panics(t, func() {
		MustInit(Config{Enabled: true, PGDSN: ""})
	}, "MustInit must panic on empty PGDSN when enabled")
}

// TestEnforcerInitDisabledDoesNotPanic verifies that MustInit does nothing
// when RBAC is disabled.
// REQ-AUTH2-001.
func TestEnforcerInitDisabledDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		MustInit(Config{Enabled: false})
	}, "MustInit must not panic when disabled")
}

// TestDenyByDefaultWhenNoAllowMatch verifies that a request with no matching
// allow policy is denied (deny-by-default).
// REQ-AUTH2-002.
func TestDenyByDefaultWhenNoAllowMatch(t *testing.T) {
	m, err := model.NewModelFromString(string(embeddedModel))
	require.NoError(t, err)
	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)

	// Add only the catch-all deny.
	_, err = e.AddPolicy([]string{"*", "*", "*", "*", "deny"})
	require.NoError(t, err)

	ef := &Enforcer{inner: e}

	allowed, err := ef.Enforce("alice", "engineering", "team_index", "write")
	require.NoError(t, err)
	assert.False(t, allowed, "must deny when no allow policy matches")
}

// TestExplicitDenyOverridesAllow verifies that an explicit deny policy
// overrides an allow policy for the same tuple.
// REQ-AUTH2-002.
func TestExplicitDenyOverridesAllow(t *testing.T) {
	m, err := model.NewModelFromString(string(embeddedModel))
	require.NoError(t, err)
	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)

	// Add both allow and deny for the same tuple.
	_, err = e.AddPolicy([]string{"alice", "t1", "r1", "read", "allow"})
	require.NoError(t, err)
	_, err = e.AddPolicy([]string{"alice", "t1", "r1", "read", "deny"})
	require.NoError(t, err)

	ef := &Enforcer{inner: e}

	allowed, err := ef.Enforce("alice", "t1", "r1", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "explicit deny must override allow")
}

// TestEnforcerThreadSafeUnderConcurrency verifies that concurrent Enforce
// calls do not trigger the race detector.
// REQ-AUTH2-002, NFR-AUTH2-001, Edge1.
func TestEnforcerThreadSafeUnderConcurrency(t *testing.T) {
	ef := newTestEnforcer(t)

	// Add a role assignment for testing.
	_, err := ef.inner.AddRoleForUserInDomain("alice", "role_member", "engineering")
	require.NoError(t, err)

	var wg sync.WaitGroup
	const goroutines = 1000
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := fmt.Sprintf("user%d", i%50)
			_, err := ef.Enforce(user, "engineering", "query:basic", "read")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("unexpected enforce error: %v", err)
	}
}

// TestMemberCanQueryBasic verifies that a member can read query:basic.
// §5.1: enforcer init + valid policy -> member can read query:basic.
func TestMemberCanQueryBasic(t *testing.T) {
	ef := newTestEnforcer(t)

	_, err := ef.inner.AddRoleForUserInDomain("alice", "role_member", "engineering")
	require.NoError(t, err)

	allowed, err := ef.Enforce("alice", "engineering", "query:basic", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "member must be allowed to read query:basic")
}

// TestAdminWildcardAllow verifies admin has wildcard access.
// §5.1 + REQ-AUTH2-002.
func TestAdminWildcardAllow(t *testing.T) {
	ef := newTestEnforcer(t)

	_, err := ef.inner.AddRoleForUserInDomain("eve", "role_admin", "engineering")
	require.NoError(t, err)

	allowed, err := ef.Enforce("eve", "engineering", "audit_log", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "admin must be allowed to read audit_log")

	allowed, err = ef.Enforce("eve", "engineering", "rbac_policy", "write")
	require.NoError(t, err)
	assert.True(t, allowed, "admin must be allowed to write rbac_policy")
}

// TestObserverDeniedWrite verifies observer cannot write team_index.
// §5.4: observer attempts write -> deny.
func TestObserverDeniedWrite(t *testing.T) {
	ef := newTestEnforcer(t)

	_, err := ef.inner.AddRoleForUserInDomain("david", "role_observer", "engineering")
	require.NoError(t, err)

	allowed, err := ef.Enforce("david", "engineering", "team_index", "write")
	require.NoError(t, err)
	assert.False(t, allowed, "observer must be denied write to team_index")
}

// TestObserverCanReadBasicQuery verifies observer can still read query:basic.
// §5.4: observer retains read-only access.
func TestObserverCanReadBasicQuery(t *testing.T) {
	ef := newTestEnforcer(t)

	_, err := ef.inner.AddRoleForUserInDomain("david", "role_observer", "engineering")
	require.NoError(t, err)

	allowed, err := ef.Enforce("david", "engineering", "query:basic", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "observer must be allowed to read query:basic")
}
