// Package redditrss — test-only hooks exported to the external test package.
// Compiled only under `go test`; never shipped in the production binary.
package redditrss

import "time"

// SetRetryParamsForTest overrides the 429 cooldown-retry parameters so external
// tests can use a tiny cooldown instead of the production default (5s).
func (a *Adapter) SetRetryParamsForTest(cooldown time.Duration, maxAttempts int) {
	a.cooldown = cooldown
	a.maxAttempts = maxAttempts
}
