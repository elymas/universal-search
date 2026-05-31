package secretstore

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by backends that are reserved for a future
// release (currently the Vault backend). Callers MAY treat this as a fatal
// configuration error rather than a missing-secret condition.
var ErrNotImplemented = errors.New("secretstore: backend not implemented")

// Resolver resolves a named secret to its plaintext value. Implementations
// MUST NOT log, print, or otherwise surface the resolved value; the value
// flows only to the immediate caller.
//
// @MX:ANCHOR: [AUTO] Runtime secret resolution boundary; callers: llm/config,
// adapters/naver, and future K8s/Vault-deployed secret sites.
// @MX:REASON: fan_in >= 3 once call-site refactors land; this is the single
// seam between deployment-specific secret backends and consuming code — a
// regression here either leaks a secret or breaks every credentialed adapter.
// @MX:SPEC: SPEC-SEC-001 (REQ-SEC-016)
type Resolver interface {
	// Get returns the plaintext secret for key, or an error if it cannot be
	// resolved. A missing secret returns a non-nil error with an empty string.
	Get(ctx context.Context, key string) (string, error)
}
