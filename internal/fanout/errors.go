// Package fanout — sentinel errors for the multi-source fanout orchestrator.
// SPEC-FAN-001: ErrAdapterRegistryEmpty (New failures) and ErrEmptyAdapterSet (Dispatch failures).
package fanout

import "errors"

// ErrAdapterRegistryEmpty is returned by New when opts.Registry is nil or contains zero adapters.
// Mirrors internal/router.ErrAdapterRegistryEmpty naming convention.
var ErrAdapterRegistryEmpty = errors.New("fanout: registry has zero adapters")

// ErrEmptyAdapterSet is returned by Dispatch when decision.AdapterSet is empty.
// Distinct from ErrAdapterRegistryEmpty — this occurs at dispatch time, not construction.
var ErrEmptyAdapterSet = errors.New("fanout: empty adapter set")

// ErrAdapterNotFound is wrapped in worker errors when registry.Get(name) returns false.
var ErrAdapterNotFound = errors.New("fanout: adapter not found in registry")
