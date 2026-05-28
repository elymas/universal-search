package secrets_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"fmt"

	"github.com/elymas/universal-search/internal/security/secrets"
)

func TestEnvResolverReadsOSEnv(t *testing.T) {
	t.Parallel()
	os.Setenv("TEST_API_KEY", "secret-value-12345")
	defer os.Unsetenv("TEST_API_KEY")

	r := &secrets.EnvResolver{}
	val, err := r.Get(context.Background(), "test.api.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret-value-12345" {
		t.Fatalf("expected secret-value-12345, got %q", val)
	}
}

func TestEnvResolverReturnsNotFound(t *testing.T) {
	t.Parallel()
	r := &secrets.EnvResolver{}
	_, err := r.Get(context.Background(), "nonexistent.secret.key.12345")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got: %v", err)
	}
}

func TestK8sResolverReadsMountedFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	secretContent := "my-k8s-secret-value"
	if err := os.WriteFile(filepath.Join(tmpDir, "db-password"), []byte(secretContent), 0600); err != nil {
		t.Fatal(err)
	}

	r := &secrets.K8sResolver{MountPath: tmpDir}
	val, err := r.Get(context.Background(), "db-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my-k8s-secret-value" {
		t.Fatalf("expected my-k8s-secret-value, got %q", val)
	}
}

func TestK8sResolverReturnsNotFound(t *testing.T) {
	t.Parallel()
	r := &secrets.K8sResolver{MountPath: t.TempDir()}
	_, err := r.Get(context.Background(), "nonexistent-key")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got: %v", err)
	}
}

func TestK8sResolverPathTraversalPrevention(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create a file outside the mount path
	outsideFile := filepath.Join(filepath.Dir(tmpDir), "outside-secret")
	os.WriteFile(outsideFile, []byte("outside"), 0600)
	defer os.Remove(outsideFile)

	r := &secrets.K8sResolver{MountPath: tmpDir}
	// Attempt path traversal
	_, err := r.Get(context.Background(), "../outside-secret")
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
}

func TestVaultResolverReturnsErrNotImplemented(t *testing.T) {
	t.Parallel()
	r := &secrets.VaultResolver{}
	_, err := r.Get(context.Background(), "any-key")
	if err == nil {
		t.Fatal("expected error from vault stub")
	}
	if !errors.Is(err, secrets.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestNewResolverDispatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		backend string
		want    string
	}{
		{"env", "*secrets.EnvResolver"},
		{"", "*secrets.EnvResolver"},
		{"k8s", "*secrets.K8sResolver"},
		{"vault", "*secrets.VaultResolver"},
	}
	for _, tt := range tests {
		r, err := secrets.NewResolver(tt.backend)
		if err != nil {
			t.Errorf("NewResolver(%q): unexpected error: %v", tt.backend, err)
			continue
		}
		got := fmt.Sprintf("%T", r)
		if got != tt.want {
			t.Errorf("NewResolver(%q) = %v, want %v", tt.backend, got, tt.want)
		}
	}
}

func TestNewResolverUnknownBackend(t *testing.T) {
	t.Parallel()
	_, err := secrets.NewResolver("unknown")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}
