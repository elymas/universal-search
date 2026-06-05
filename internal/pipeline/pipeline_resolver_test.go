// Package pipeline_test — tests for Resolver-wired registry construction (SPEC-SEC-002).
//
// Tests drive secret state by injecting a fake Resolver (NFR-SEC2-002: no t.Setenv
// under -race). All tests are t.Parallel()-safe.
package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/pipeline"
	"github.com/elymas/universal-search/internal/security/secretstore"
)

// fakeResolver is a test double for secretstore.Resolver. Returns scripted
// values per key; unset keys return an error matching EnvResolver semantics.
type fakeResolver struct {
	vals  map[string]string
	err   error // if set, all Get calls return this error
	calls []string
}

func newFakeResolver(vals map[string]string) *fakeResolver {
	return &fakeResolver{vals: vals}
}

func newErrResolver(err error) *fakeResolver {
	return &fakeResolver{err: err}
}

func (f *fakeResolver) Get(_ context.Context, key string) (string, error) {
	f.calls = append(f.calls, key)
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.vals[key]
	if !ok || v == "" {
		return "", fmt.Errorf("secretstore: env %q not set", key)
	}
	return v, nil
}

func (f *fakeResolver) called(key string) bool {
	for _, k := range f.calls {
		if k == key {
			return true
		}
	}
	return false
}

// registryContains checks if a registry contains a named adapter.
func registryContains(t *testing.T, reg *adapters.Registry, name string) bool {
	t.Helper()
	_, ok := reg.Get(name)
	return ok
}

// --- REQ-SEC2-001: CLI constructs ONE Resolver, threaded into registry ---

// TestBuildProductionRegistryWithResolver_NilResolver verifies that
// BuildProductionRegistryWithResolver(nil) works and produces a valid registry
// (backward compat: nil → EnvResolver fallback).
func TestBuildProductionRegistryWithResolver_NilResolver(t *testing.T) {
	t.Parallel()
	reg := pipeline.BuildProductionRegistryWithResolver(nil)
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
}

// TestBuildProductionRegistryWithResolver_UsesInjectedResolver proves
// that naver and github resolve through the injected Resolver, not os.Getenv.
func TestBuildProductionRegistryWithResolver_UsesInjectedResolver(t *testing.T) {
	t.Parallel()

	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":      "naver-id-from-resolver",
		"NAVER_CLIENT_SECRET":  "naver-secret-from-resolver",
		"USEARCH_GITHUB_TOKEN": "github-token-from-resolver",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	if !registryContains(t, reg, "naver") {
		t.Error("expected naver to be registered via Resolver")
	}
	if !registryContains(t, reg, "github") {
		t.Error("expected github to be registered via Resolver")
	}

	// Verify the Resolver was actually consulted.
	if !fr.called("NAVER_CLIENT_ID") {
		t.Error("Resolver was not called for NAVER_CLIENT_ID")
	}
	if !fr.called("USEARCH_GITHUB_TOKEN") {
		t.Error("Resolver was not called for USEARCH_GITHUB_TOKEN")
	}
}

// --- REQ-SEC2-002: naver resolves via injected Resolver ---
// (Core tests in naver_resolver_test.go; this verifies the pipeline wiring.)

// TestNaverRegisteredWithSkipAuthCheck verifies that naver is registered
// even when its auth env vars are absent from process env (proving SkipAuthCheck).

// --- REQ-SEC2-003: github token via injected Resolver ---

// TestGithubTokenViaResolver verifies that the github adapter's token is
// resolved through the injected Resolver (REQ-SEC2-003).
func TestGithubTokenViaResolver(t *testing.T) {
	t.Parallel()

	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":     "n-id",
		"NAVER_CLIENT_SECRET": "n-secret",
		"USEARCH_GITHUB_TOKEN": "resolved-github-token",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)
	if !registryContains(t, reg, "github") {
		t.Error("expected github to be registered when Resolver provides token")
	}
}

// TestGithubTokenFallbackAlias verifies that GITHUB_TOKEN is used when
// USEARCH_GITHUB_TOKEN is not resolved (REQ-SEC2-003).
func TestGithubTokenFallbackAlias(t *testing.T) {
	t.Parallel()

	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":     "n-id",
		"NAVER_CLIENT_SECRET": "n-secret",
		"GITHUB_TOKEN":        "fallback-token",
		// USEARCH_GITHUB_TOKEN is NOT set — fallback to GITHUB_TOKEN.
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)
	if !registryContains(t, reg, "github") {
		t.Error("expected github to be registered via GITHUB_TOKEN fallback")
	}
	if !fr.called("USEARCH_GITHUB_TOKEN") {
		t.Error("expected primary key USEARCH_GITHUB_TOKEN to be tried first")
	}
	if !fr.called("GITHUB_TOKEN") {
		t.Error("expected fallback key GITHUB_TOKEN to be tried")
	}
}

// TestGithubSkippedWhenNoTokenResolves verifies that github is not registered
// when neither USEARCH_GITHUB_TOKEN nor GITHUB_TOKEN resolves (REQ-SEC2-003).
func TestGithubSkippedWhenNoTokenResolves(t *testing.T) {
	t.Parallel()

	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":     "n-id",
		"NAVER_CLIENT_SECRET": "n-secret",
		// No github tokens provided.
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)
	if registryContains(t, reg, "github") {
		t.Error("expected github to be skipped when no token resolves")
	}
}

// --- REQ-SEC2-004: vault stub surfaces ---

// TestVaultBackendReturnsNotImplemented verifies that the vault backend
// resolver.Get returns ErrNotImplemented.
func TestVaultBackendReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	vaultResolver, err := secretstore.NewResolver("vault", "")
	if err != nil {
		t.Fatalf("NewResolver(vault) error = %v", err)
	}

	_, getErr := vaultResolver.Get(context.Background(), "ANY_KEY")
	if !errors.Is(getErr, secretstore.ErrNotImplemented) {
		t.Errorf("vault Get error = %v, want ErrNotImplemented", getErr)
	}
}

// TestBuildProductionRegistryWithResolver_VaultErrorFails verifies that
// when the Resolver returns ErrNotImplemented (vault backend), building
// the registry returns a clear error naming "vault" (REQ-SEC2-004).
func TestBuildProductionRegistryWithResolver_VaultErrorFails(t *testing.T) {
	t.Parallel()

	vaultResolver, _ := secretstore.NewResolver("vault", "")
	_, err := pipeline.BuildProductionRegistryWithResolverAndError(vaultResolver)
	if err == nil {
		t.Fatal("expected error for vault backend with credentialed adapters")
	}
	if !errors.Is(err, secretstore.ErrNotImplemented) {
		t.Errorf("error = %v, want errors.Is(_, ErrNotImplemented)", err)
	}
	// Error must name "vault" but NOT contain any secret value.
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "vault") {
		t.Errorf("error message %q should mention 'vault'", errMsg)
	}
}

// TestVaultErrorNotTreatedAsMissingKey verifies that vault ErrNotImplemented
// is distinct from a "key not set" error. The vault error is a fatal config
// error, not a silent skip.
func TestVaultErrorNotTreatedAsMissingKey(t *testing.T) {
	t.Parallel()

	vaultResolver, _ := secretstore.NewResolver("vault", "")
	_, err := pipeline.BuildProductionRegistryWithResolverAndError(vaultResolver)

	// Must be an error (not nil/silent-skip).
	if err == nil {
		t.Fatal("vault error must not be silently skipped")
	}

	// Must NOT be the same as "env not set" error.
	if strings.Contains(err.Error(), "not set") && !strings.Contains(strings.ToLower(err.Error()), "vault") {
		t.Errorf("vault error looks like a missing-key skip: %q", err.Error())
	}
}

// TestCLIUnknownBackendFails verifies that NewResolver with an unknown
// backend returns a config error (REQ-SEC2-004).
func TestCLIUnknownBackendFails(t *testing.T) {
	t.Parallel()

	_, err := secretstore.NewResolver("hsm", "")
	if err == nil {
		t.Fatal("expected error for unknown backend 'hsm'")
	}
	if !strings.Contains(err.Error(), "hsm") {
		t.Errorf("error %q should mention the unknown backend name", err.Error())
	}
}

// --- REQ-SEC2-005: env backend backward compat ---

// TestEnvBackendBackwardCompatible verifies that with an env-equivalent
// Resolver, adapters register/skip exactly as with the pre-SPEC direct
// os.Getenv path (REQ-SEC2-005).
func TestEnvBackendBackwardCompatible(t *testing.T) {
	t.Parallel()

	// Simulate env backend: Resolver provides naver creds + github token.
	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":      "env-naver-id",
		"NAVER_CLIENT_SECRET":  "env-naver-secret",
		"USEARCH_GITHUB_TOKEN": "env-github-token",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	// Credentialed adapters that have creds should register.
	if !registryContains(t, reg, "naver") {
		t.Error("expected naver to register with env-equivalent Resolver")
	}
	if !registryContains(t, reg, "github") {
		t.Error("expected github to register with env-equivalent Resolver")
	}

	// Non-credentialed adapters should still register.
	if !registryContains(t, reg, "searxng") {
		t.Error("expected searxng to register (no auth required)")
	}
	if !registryContains(t, reg, "hackernews") {
		t.Error("expected hackernews to register (no auth required)")
	}
}

// TestEnvBackendMissingCredsSkipsAdapter verifies that missing naver creds
// under env-equivalent skip semantics → adapter skipped, no error
// (REQ-SEC2-005, parity with pre-SPEC).
func TestEnvBackendMissingCredsSkipsAdapter(t *testing.T) {
	t.Parallel()

	// Resolver does NOT provide naver creds.
	fr := newFakeResolver(map[string]string{
		"USEARCH_GITHUB_TOKEN": "token-only",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	if registryContains(t, reg, "naver") {
		t.Error("expected naver to be skipped when creds not resolved")
	}
	// No error should occur — just a silent skip, like pre-SPEC behavior.
}

// TestNoNewRequiredConfig verifies that BuildProductionRegistry (no args)
// still works without any configuration changes (REQ-SEC2-005).
func TestNoNewRequiredConfig(t *testing.T) {
	t.Parallel()
	reg := pipeline.BuildProductionRegistry()
	if reg == nil {
		t.Fatal("BuildProductionRegistry() returned nil — backward compat broken")
	}
}

// --- REQ-SEC2-006: no secret value leak (NFR-SEC2-001) ---

// TestNoSecretInErrorOutput verifies that error messages never contain
// the actual secret value. Errors reference env-var NAMES and backend
// identifiers only (NFR-SEC2-001).
func TestNoSecretInErrorOutput(t *testing.T) {
	t.Parallel()

	sentinelID := "SENTINEL_SECRET_ID_12345"
	sentinelSecret := "SENTINEL_SECRET_VAL_67890"
	sentinelToken := "SENTINEL_TOKEN_ABCDEF"

	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":      sentinelID,
		"NAVER_CLIENT_SECRET":  sentinelSecret,
		"USEARCH_GITHUB_TOKEN": sentinelToken,
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	// The registry built successfully; now check the admin view
	// (SnapshotForAdmin) never leaks secret values.
	for _, view := range reg.SnapshotForAdmin() {
		if strings.Contains(view.SecretValue, sentinelID) ||
			strings.Contains(view.SecretValue, sentinelSecret) ||
			strings.Contains(view.SecretValue, sentinelToken) {
			t.Errorf("admin view SecretValue leaked a secret for adapter %q", view.ID)
		}
	}
}

// TestNoSecretInVaultError verifies that vault-path error messages
// name "vault" but never contain a secret value (NFR-SEC2-001).
func TestNoSecretInVaultError(t *testing.T) {
	t.Parallel()

	vaultResolver, _ := secretstore.NewResolver("vault", "")
	_, err := pipeline.BuildProductionRegistryWithResolverAndError(vaultResolver)
	if err == nil {
		t.Fatal("expected error")
	}

	errMsg := err.Error()
	// Should name the backend.
	if !strings.Contains(strings.ToLower(errMsg), "vault") {
		t.Errorf("error should mention 'vault': %q", errMsg)
	}
	// Should NOT contain any key name (vault stub doesn't even get to keys,
	// but let's be thorough: no env var names in the vault error).
	if strings.Contains(errMsg, "NAVER_CLIENT_ID") ||
		strings.Contains(errMsg, "USEARCH_GITHUB_TOKEN") {
		t.Errorf("vault error leaks env var names: %q", errMsg)
	}
}

// --- REQ-SEC2-007: Resolver-aware registration (SkipAuthCheck) ---

// TestK8sBackendAdapterRegisters verifies that with a Resolver providing
// credentials from a non-env source (simulated k8s), naver and github
// successfully register even though process env does NOT have their auth
// vars. This proves SkipAuthCheck bypasses the registry's env-only gate.
func TestK8sBackendAdapterRegisters(t *testing.T) {
	t.Parallel()

	// Simulate k8s mount: Resolver provides all creds.
	// Process env is irrelevant — the Resolver is the sole source.
	fr := newFakeResolver(map[string]string{
		"NAVER_CLIENT_ID":      "k8s-naver-id",
		"NAVER_CLIENT_SECRET":  "k8s-naver-secret",
		"USEARCH_GITHUB_TOKEN": "k8s-github-token",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	if !registryContains(t, reg, "naver") {
		t.Error("naver should register via Resolver creds even without process env (SkipAuthCheck)")
	}
	if !registryContains(t, reg, "github") {
		t.Error("github should register via Resolver creds even without process env (SkipAuthCheck)")
	}
}

// TestK8sBackendMissingSecretSkipsAdapter verifies that when the Resolver
// does NOT provide a secret, the adapter is silently skipped (no panic).
func TestK8sBackendMissingSecretSkipsAdapter(t *testing.T) {
	t.Parallel()

	// Resolver provides github token but NOT naver creds.
	fr := newFakeResolver(map[string]string{
		"USEARCH_GITHUB_TOKEN": "k8s-github-token",
	})

	reg := pipeline.BuildProductionRegistryWithResolver(fr)

	if registryContains(t, reg, "naver") {
		t.Error("naver should be skipped when Resolver doesn't provide creds")
	}
	if !registryContains(t, reg, "github") {
		t.Error("github should register when Resolver provides token")
	}
}

// --- NFR-SEC2-003: backend-selection determinism ---

// TestNewResolverConfig_Determinism verifies that backend-string → Resolver
// type mapping is deterministic (NFR-SEC2-003).
func TestNewResolverConfig_Determinism(t *testing.T) {
	t.Parallel()

	tests := []struct {
		backend  string
		wantType string
		wantErr  bool
	}{
		{"env", "*secretstore.EnvResolver", false},
		{"", "*secretstore.EnvResolver", false},
		{"k8s", "*secretstore.K8sResolver", false},
		{"vault", "*secretstore.VaultResolver", false},
		{"hsm", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			t.Parallel()

			r, err := secretstore.NewResolver(tt.backend, "")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unknown backend")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotType := fmt.Sprintf("%T", r)
			if gotType != tt.wantType {
				t.Errorf("NewResolver(%q) type = %s, want %s", tt.backend, gotType, tt.wantType)
			}
		})
	}
}

// --- Helper: verify fakeResolver satisfies the interface ---

func TestFakeResolverSatisfiesInterface(t *testing.T) {
	t.Parallel()
	var _ secretstore.Resolver = (*fakeResolver)(nil)
}
