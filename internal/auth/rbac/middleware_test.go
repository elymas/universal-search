package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/deepagent/costguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextKeysExported verifies that TeamIDKey and RolesKey are exported.
// REQ-AUTH2-003.
func TestContextKeysExported(t *testing.T) {
	assert.Equal(t, contextKey("auth.team_id"), TeamIDKey)
	assert.Equal(t, contextKey("auth.roles"), RolesKey)
}

// TestExtractsFromAUTH001JWTContext verifies that TeamScopeMiddleware extracts
// identity from AUTH-001 JWT context (priority 1).
// REQ-AUTH2-003, §5.2 case A.
func TestExtractsFromAUTH001JWTContext(t *testing.T) {
	var capturedUserID, capturedTeamID string
	var capturedRoles []string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		capturedTeamID = TeamIDFromContext(r.Context())
		capturedRoles = RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "default"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	ctx := context.WithValue(req.Context(), costguard.UserIDKey, "alice")
	ctx = context.WithValue(ctx, TeamIDKey, "engineering")
	ctx = context.WithValue(ctx, RolesKey, []string{"member"})
	// Add spoofing headers that should be IGNORED.
	req = req.WithContext(ctx)
	req.Header.Set("X-User-Id", "malicious")
	req.Header.Set("X-Team-Id", "marketing")
	req.Header.Set("X-Roles", "admin")

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", capturedUserID)
	assert.Equal(t, "engineering", capturedTeamID)
	assert.Equal(t, []string{"member"}, capturedRoles)
}

// TestFallsBackToHeadersWhenContextMissing verifies header fallback when
// AUTH-001 context is absent.
// REQ-AUTH2-003, §5.2 case B.
func TestFallsBackToHeadersWhenContextMissing(t *testing.T) {
	var capturedUserID, capturedTeamID string
	var capturedRoles []string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		capturedTeamID = TeamIDFromContext(r.Context())
		capturedRoles = RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "default"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "bob")
	req.Header.Set("X-Team-Id", "research")
	req.Header.Set("X-Roles", "member")

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "bob", capturedUserID)
	assert.Equal(t, "research", capturedTeamID)
	assert.Equal(t, []string{"member"}, capturedRoles)
}

// TestFallsBackToAnonymousWhenAllMissing verifies anonymous fallback when
// both context and headers are missing.
// REQ-AUTH2-003, §5.2 case C.
func TestFallsBackToAnonymousWhenAllMissing(t *testing.T) {
	var capturedUserID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "default"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "anonymous", capturedUserID)
}

// TestForwardCompatWithAUTH001Keys verifies that costguard.UserIDKey and
// costguard.TenantIDKey semantics are unchanged by AUTH-002.
// NFR-AUTH2-005.
func TestForwardCompatWithAUTH001Keys(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, costguard.UserIDKey, "alice")
	ctx = context.WithValue(ctx, costguard.TenantIDKey, "tenant-1")

	// AUTH-001 keys must still work.
	assert.Equal(t, "alice", costguard.UserIDFromContext(ctx))
	assert.Equal(t, "tenant-1", costguard.TenantIDFromContext(ctx))

	// AUTH-002 keys are additive.
	assert.Equal(t, "", TeamIDFromContext(ctx)) // not set
	assert.Nil(t, RolesFromContext(ctx))        // not set

	// Set AUTH-002 keys — no interference with AUTH-001.
	ctx = context.WithValue(ctx, TeamIDKey, "engineering")
	ctx = context.WithValue(ctx, RolesKey, []string{"member"})

	assert.Equal(t, "alice", costguard.UserIDFromContext(ctx), "AUTH-001 UserIDKey must be unchanged")
	assert.Equal(t, "tenant-1", costguard.TenantIDFromContext(ctx), "AUTH-001 TenantIDKey must be unchanged")
	assert.Equal(t, "engineering", TeamIDFromContext(ctx))
	assert.Equal(t, []string{"member"}, RolesFromContext(ctx))
}

// TestEmptyTeamIDFallsBackToDefault verifies that empty team_id uses default fallback.
// REQ-AUTH2-004, §5.3 case A.
func TestEmptyTeamIDFallsBackToDefault(t *testing.T) {
	var capturedTeamID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTeamID = TeamIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "default"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "charlie")
	req.Header.Set("X-Roles", "member")
	// No X-Team-Id header.

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "default", capturedTeamID)
}

// TestEmptyTeamIDReturns400WhenDefaultBlank verifies HTTP 400 when default_team_id is empty.
// REQ-AUTH2-004, §5.3 case B.
func TestEmptyTeamIDReturns400WhenDefaultBlank(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when team_id is required but missing")
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: ""})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "charlie")
	req.Header.Set("X-Roles", "member")
	// No X-Team-Id header.

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "team_id_required")
}

// TestDefaultTeamIDConfigurable verifies custom default_team_id value.
// REQ-AUTH2-004.
func TestDefaultTeamIDConfigurable(t *testing.T) {
	var capturedTeamID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTeamID = TeamIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "engineering"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "charlie")

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "engineering", capturedTeamID)
}

// TestTeamIDFromContextHelper verifies TeamIDFromContext helper consistency.
// REQ-AUTH2-006.
func TestTeamIDFromContextHelper(t *testing.T) {
	ctx := context.Background()

	// Not set.
	assert.Equal(t, "", TeamIDFromContext(ctx))

	// Set directly.
	ctx = context.WithValue(ctx, TeamIDKey, "team-a")
	assert.Equal(t, "team-a", TeamIDFromContext(ctx))

	// Set via middleware.
	var captured string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = TeamIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := TeamScopeMiddleware(Config{DefaultTeamID: "default"})
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-Team-Id", "team-b")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, "team-b", captured)
}

// TestEnforceMiddlewareReturns403OnDeny verifies deny response format.
// REQ-AUTH2-005, §5.5.
func TestEnforceMiddlewareReturns403OnDeny(t *testing.T) {
	ef := newTestEnforcer(t)
	_, err := ef.inner.AddRoleForUserInDomain("david", "role_observer", "engineering")
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called on deny")
	})

	mw := EnforceMiddleware(ef, "audit_log", "read")
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/admin/audit", nil)
	ctx := context.WithValue(req.Context(), costguard.UserIDKey, "david")
	ctx = context.WithValue(ctx, TeamIDKey, "engineering")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")
	assert.Contains(t, rec.Body.String(), "audit_log")
}

// TestEnforceMiddlewareAllowsValidRequest verifies allow passes through.
// REQ-AUTH2-005, §5.5.
func TestEnforceMiddlewareAllowsValidRequest(t *testing.T) {
	ef := newTestEnforcer(t)
	_, err := ef.inner.AddRoleForUserInDomain("eve", "role_admin", "engineering")
	require.NoError(t, err)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := EnforceMiddleware(ef, "audit_log", "read")
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/admin/audit", nil)
	ctx := context.WithValue(req.Context(), costguard.UserIDKey, "eve")
	ctx = context.WithValue(ctx, TeamIDKey, "engineering")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.True(t, called, "handler should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}
