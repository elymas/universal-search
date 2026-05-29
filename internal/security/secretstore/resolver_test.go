package secretstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvResolverReadsOSEnv(t *testing.T) {
	const key = "SECRETSTORE_TEST_KEY"
	t.Setenv(key, "s3cr3t-value")

	r := NewEnvResolver()
	got, err := r.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get(%q) returned error: %v", key, err)
	}
	if got != "s3cr3t-value" {
		t.Errorf("Get(%q) = %q, want %q", key, got, "s3cr3t-value")
	}
}

func TestEnvResolverUnsetReturnsError(t *testing.T) {
	r := NewEnvResolver()
	if _, err := r.Get(context.Background(), "SECRETSTORE_DEFINITELY_UNSET_XYZ"); err == nil {
		t.Fatal("Get on unset env var must return an error")
	}
}

func TestK8sResolverReadsMountedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DB_PASSWORD"), []byte("hunter2\n"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	r := NewK8sResolver(dir)
	got, err := r.Get(context.Background(), "DB_PASSWORD")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	// Trailing newline (common in mounted Secret files) must be trimmed.
	if got != "hunter2" {
		t.Errorf("Get = %q, want %q", got, "hunter2")
	}
}

func TestK8sResolverMissingFileReturnsError(t *testing.T) {
	r := NewK8sResolver(t.TempDir())
	if _, err := r.Get(context.Background(), "ABSENT"); err == nil {
		t.Fatal("Get on missing mounted file must return an error")
	}
}

func TestK8sResolverRejectsPathTraversal(t *testing.T) {
	r := NewK8sResolver(t.TempDir())
	for _, key := range []string{"../etc/passwd", "sub/key", ""} {
		if _, err := r.Get(context.Background(), key); err == nil {
			t.Errorf("Get(%q) must reject path-unsafe key", key)
		}
	}
}

func TestVaultResolverReturnsErrNotImplemented(t *testing.T) {
	r := NewVaultResolver()
	_, err := r.Get(context.Background(), "ANYTHING")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("VaultResolver.Get error = %v, want ErrNotImplemented", err)
	}
}

func TestResolverFactoryDispatch(t *testing.T) {
	tests := []struct {
		name      string
		backend   string
		wantType  string
		wantError bool
	}{
		{"env", "env", "*secretstore.EnvResolver", false},
		{"empty defaults to env", "", "*secretstore.EnvResolver", false},
		{"k8s", "k8s", "*secretstore.K8sResolver", false},
		{"vault", "vault", "*secretstore.VaultResolver", false},
		{"unknown", "consul", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewResolver(tt.backend, "")
			if tt.wantError {
				if err == nil {
					t.Fatalf("NewResolver(%q) expected error, got nil", tt.backend)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewResolver(%q) unexpected error: %v", tt.backend, err)
			}
			if got := typeName(r); got != tt.wantType {
				t.Errorf("NewResolver(%q) type = %s, want %s", tt.backend, got, tt.wantType)
			}
		})
	}
}

func TestResolverFactoryK8sUsesDefaultMountPath(t *testing.T) {
	r, err := NewResolver(BackendK8s, "")
	if err != nil {
		t.Fatalf("NewResolver(k8s) error: %v", err)
	}
	k8s, ok := r.(*K8sResolver)
	if !ok {
		t.Fatalf("expected *K8sResolver, got %T", r)
	}
	if k8s.mountPath != DefaultK8sMountPath {
		t.Errorf("default mountPath = %q, want %q", k8s.mountPath, DefaultK8sMountPath)
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *EnvResolver:
		return "*secretstore.EnvResolver"
	case *K8sResolver:
		return "*secretstore.K8sResolver"
	case *VaultResolver:
		return "*secretstore.VaultResolver"
	default:
		return "unknown"
	}
}
