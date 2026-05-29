package secretstore

import "context"

// VaultResolver is a V1 stub. A real HashiCorp Vault client is reserved for a
// post-V1 release; until then every Get returns ErrNotImplemented so that
// selecting the vault backend fails loudly rather than silently returning an
// empty secret.
type VaultResolver struct{}

// NewVaultResolver constructs the stub VaultResolver.
func NewVaultResolver() *VaultResolver { return &VaultResolver{} }

// Get always returns ErrNotImplemented (V1 stub).
func (r *VaultResolver) Get(_ context.Context, _ string) (string, error) {
	return "", ErrNotImplemented
}
