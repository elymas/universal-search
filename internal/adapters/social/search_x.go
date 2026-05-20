// Package social — X (Twitter) stub search path.
// REQ-ADP6-008: X is disabled in v0; returns sentinels based on env state.
package social

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// xEnabledEnvVar is the environment variable that gates X adapter activation.
const xEnabledEnvVar = "USEARCH_X_ENABLED"

// searchX implements the X stub: returns ErrXDisabled when the env var is unset
// or returns ErrXProviderNotConfigured when the env var is "true" but no provider
// is wired.
//
// REQ-ADP6-008: X is disabled in v0. No HTTP requests are made under any env state.
//
// @MX:NOTE: [AUTO] X stub: env lookup uses a.envLookup (injected via XOptions.EnvLookup)
// for goroutine-safe tests under -race. Never use t.Setenv or os.Getenv in tests.
// @MX:SPEC: SPEC-ADP-006
func (a *Adapter) searchX(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	val := a.envLookup(xEnabledEnvVar)
	if val != "true" {
		// Env var not set or not exactly "true" — X is disabled.
		return nil, ErrXDisabled
	}
	// Env var is "true" but no provider wired in v0.
	return nil, ErrXProviderNotConfigured
}
