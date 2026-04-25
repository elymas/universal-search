// Package noop provides a reference Adapter implementation. The noop adapter
// satisfies types.Adapter with deterministic, side-effect-free behavior.
// Used as a compile-time interface assertion and a stable test fixture for
// SPEC-FAN-001, SPEC-IR-001, and SPEC-IDX-001 development.
package noop

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// Adapter is a no-op Adapter that returns nil docs and nil errors.
type Adapter struct{ name string }

// New constructs a noop Adapter with the given name.
func New(name string) *Adapter { return &Adapter{name: name} }

// Name returns the configured adapter name.
func (a *Adapter) Name() string { return a.name }

// Healthcheck always returns nil.
func (a *Adapter) Healthcheck(_ context.Context) error { return nil }

// Search returns (nil, nil) unless ctx is already cancelled.
func (a *Adapter) Search(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

// Capabilities returns a minimal descriptor.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          a.name,
		DisplayName:       "Noop",
		DocTypes:          []types.DocType{types.DocTypeOther},
		SupportedLangs:    []string{"en"},
		DefaultMaxResults: 10,
		Notes:             "Reference noop adapter; no external calls.",
	}
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
