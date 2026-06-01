package secretstore

import "fmt"

// Backend identifiers selected via security.yaml secrets.backend.
const (
	BackendEnv   = "env"
	BackendK8s   = "k8s"
	BackendVault = "vault"
)

// DefaultK8sMountPath mirrors the security.yaml secrets.k8s_mount_path default.
const DefaultK8sMountPath = "/var/run/secrets"

// NewResolver constructs a Resolver for the given backend. The vault backend
// returns a stub resolver (Get yields ErrNotImplemented) — it is constructed
// successfully so configuration validation passes, but resolution is deferred
// to a post-V1 implementation. An unknown backend is a configuration error.
//
// mountPath is used only by the k8s backend; pass DefaultK8sMountPath (or the
// security.yaml secrets.k8s_mount_path value) for env/vault.
func NewResolver(backend, mountPath string) (Resolver, error) {
	switch backend {
	case BackendEnv, "":
		return NewEnvResolver(), nil
	case BackendK8s:
		if mountPath == "" {
			mountPath = DefaultK8sMountPath
		}
		return NewK8sResolver(mountPath), nil
	case BackendVault:
		return NewVaultResolver(), nil
	default:
		return nil, fmt.Errorf("secretstore: unknown backend %q (want env|k8s|vault)", backend)
	}
}
