package secretstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// K8sResolver resolves secrets from files mounted by a Kubernetes Secret
// volume. Each secret is a single file named after its key under mountPath
// (the default Helm layout: one file per secret entry). It is the default
// backend for Helm-deployed production.
type K8sResolver struct {
	mountPath string
}

// NewK8sResolver constructs a K8sResolver reading from mountPath (the
// security.yaml secrets.k8s_mount_path value, e.g. /var/run/secrets).
func NewK8sResolver(mountPath string) *K8sResolver {
	return &K8sResolver{mountPath: mountPath}
}

// Get reads the mounted file <mountPath>/<key> and returns its trimmed
// contents. A missing file or empty value returns an error. The key is
// path-sanitised so it cannot escape mountPath.
func (r *K8sResolver) Get(_ context.Context, key string) (string, error) {
	if strings.ContainsAny(key, "/\\") || key == "" {
		return "", fmt.Errorf("secretstore: invalid k8s secret key %q", key)
	}
	p := filepath.Join(r.mountPath, key)
	b, err := os.ReadFile(p) //nolint:gosec // path is rooted at mountPath and key is sanitised above
	if err != nil {
		return "", fmt.Errorf("secretstore: read k8s secret %q: %w", key, err)
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("secretstore: k8s secret %q is empty", key)
	}
	return v, nil
}
