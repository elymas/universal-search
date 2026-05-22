package orchestrator

import "errors"

// Sentinel errors for the shared search pipeline.
var (
	// ErrNoAdaptersMatched indicates the router + source filter produced an empty set.
	ErrNoAdaptersMatched = errors.New("orchestrator: no adapters matched")

	// ErrAllAdaptersFailed indicates all adapters in the effective set returned errors.
	ErrAllAdaptersFailed = errors.New("orchestrator: all adapters failed")
)
