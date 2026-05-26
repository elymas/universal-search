// Package adapters_test — SnapshotForAdmin tests for SPEC-UI-002 REQ-AS-001,
// REQ-AK-001, REQ-AS-003.
package adapters_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// TestSnapshotForAdmin verifies that SnapshotForAdmin returns one entry per
// registered adapter, never includes secret values, and handles partial
// metadata gracefully.
func TestSnapshotForAdmin(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	// Register 9 adapters (matching production adapter count).
	names := []string{
		"reddit", "hackernews", "arxiv", "github", "youtube",
		"bluesky", "searxng", "naver", "koreanews",
	}
	for _, n := range names {
		a := &stubAdapter{
			name: n,
			caps: types.Capabilities{
				SourceID:    n,
				DisplayName: n,
				DocTypes:    []types.DocType{types.DocTypeOther},
			},
		}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register(%q): %v", n, err)
		}
	}

	views := reg.SnapshotForAdmin()

	if got, want := len(views), len(names); got != want {
		t.Fatalf("SnapshotForAdmin: got %d entries, want %d", got, want)
	}

	// Build a set of seen names to verify all are present.
	seen := make(map[string]bool, len(names))
	for _, v := range views {
		seen[v.ID] = true

		// CRITICAL: No secret values in payload.
		// Only the source identifier (env var name) and set/unset boolean.
		if v.SecretValue != "" {
			t.Errorf("adapter %q: SecretValue must be empty, got %q", v.ID, v.SecretValue)
		}
		// For non-auth adapters, KeySet=true with empty SecretSource is valid.
		// For auth-required adapters, SecretSource must be populated.
		// (Tested in TestSnapshotForAdminAuthFields.)
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("adapter %q missing from SnapshotForAdmin output", n)
		}
	}
}

// TestSnapshotForAdminAuthFields verifies that adapters with RequiresAuth=true
// report their AuthEnvVars as SecretSource and KeySet reflects whether the
// env var is actually set.
func TestSnapshotForAdminAuthFields(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	// Register an adapter requiring auth, with the env var set.
	const envKey = "USEARCH_TEST_ADAPTER_KEY"
	os.Setenv(envKey, "super-secret-value-do-not-leak")
	t.Cleanup(func() { os.Unsetenv(envKey) })

	a := &stubAdapter{
		name: "test-auth-adapter",
		caps: types.Capabilities{
			SourceID:     "test-auth-adapter",
			DisplayName:  "Test Auth Adapter",
			RequiresAuth: true,
			AuthEnvVars:  []string{envKey},
		},
	}
	if err := reg.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}

	views := reg.SnapshotForAdmin()
	if got := len(views); got != 1 {
		t.Fatalf("expected 1 view, got %d", got)
	}

	v := views[0]

	// SecretSource should list the env var name, not its value.
	if v.SecretSource != envKey {
		t.Errorf("SecretSource: got %q, want %q", v.SecretSource, envKey)
	}
	if !v.KeySet {
		t.Errorf("KeySet: got false, want true (env var %s is set)", envKey)
	}

	// CRITICAL: actual secret value must NEVER appear in the view.
	if v.SecretValue != "" {
		t.Errorf("SecretValue must be empty, got %q", v.SecretValue)
	}
}

// TestSnapshotForAdminMissingAuth verifies that adapters with RequiresAuth=true
// but unset env vars report KeySet=false.
func TestSnapshotForAdminMissingAuth(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	const envKey = "USEARCH_MISSING_KEY_12345"
	// Deliberately do NOT set this env var.

	a := &stubAdapter{
		name: "test-no-key-adapter",
		caps: types.Capabilities{
			SourceID:     "test-no-key-adapter",
			DisplayName:  "Test No Key",
			RequiresAuth: true,
			AuthEnvVars:  []string{envKey},
		},
	}
	if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	views := reg.SnapshotForAdmin()
	v := views[0]

	if v.KeySet {
		t.Errorf("KeySet: got true, want false (env var %s is not set)", envKey)
	}
	if v.SecretSource != envKey {
		t.Errorf("SecretSource: got %q, want %q", v.SecretSource, envKey)
	}
}

// TestSnapshotForAdminEmptyRegistry verifies behavior with no adapters.
func TestSnapshotForAdminEmptyRegistry(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	views := reg.SnapshotForAdmin()
	if len(views) != 0 {
		t.Errorf("empty registry: got %d views, want 0", len(views))
	}
}

// TestSnapshotForAdminNoSecretLeak scans all string fields for common secret
// patterns to ensure no accidental leakage.
func TestSnapshotForAdminNoSecretLeak(t *testing.T) {
	const secretVal = "sk-test-secret-do-not-leak-abc123"
	const envKey = "USEARCH_LEAK_TEST_KEY"
	os.Setenv(envKey, secretVal)
	t.Cleanup(func() { os.Unsetenv(envKey) })

	reg := adapters.NewRegistry(nil)
	a := &stubAdapter{
		name: "leak-test",
		caps: types.Capabilities{
			SourceID:     "leak-test",
			DisplayName:  "Leak Test",
			RequiresAuth: true,
			AuthEnvVars:  []string{envKey},
		},
	}
	if err := reg.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}

	views := reg.SnapshotForAdmin()
	v := views[0]

	// Check all string fields for the secret value.
	for _, field := range []string{
		v.ID, v.Status, v.SecretSource, v.LastError,
	} {
		if strings.Contains(field, secretVal) {
			t.Errorf("field contains secret value: %q", field)
		}
	}
}

// stubAdapter is a minimal types.Adapter for tests.
type stubAdapter struct {
	name string
	caps types.Capabilities
}

func (s *stubAdapter) Name() string                        { return s.name }
func (s *stubAdapter) Capabilities() types.Capabilities    { return s.caps }
func (s *stubAdapter) Healthcheck(_ context.Context) error { return nil }
func (s *stubAdapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	return nil, nil
}
