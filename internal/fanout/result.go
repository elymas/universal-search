// Package fanout — Result and Stats types for Dispatch output.
// SPEC-FAN-001 §2.6, §6.1 item b.
package fanout

import "github.com/elymas/universal-search/pkg/types"

// Result is the typed return value of Dispatch. It is JSON-marshalable for
// diagnostic dumps. The production CLI formatter consumes Docs; AdapterErrors
// and Stats are diagnostic.
type Result struct {
	// Docs is the deduped, sorted merged slice of NormalizedDoc from all
	// successful adapters. Never nil (may be empty slice).
	Docs []types.NormalizedDoc `json:"docs"`
	// AdapterErrors is nil when Stats.ErrorCount == 0.
	// When Stats.ErrorCount >= 1, it is a non-nil map with exactly ErrorCount entries.
	// Keys are adapter names (matching Adapter.Name()); values are the errors returned
	// by that adapter's Search call (or context errors for cancelled/timed-out adapters).
	AdapterErrors map[string]error `json:"adapter_errors,omitempty"`
	// Stats carries aggregate dispatch statistics.
	Stats Stats `json:"stats"`
}

// Stats captures aggregate metrics for a single Dispatch call.
// Invariant (structural, not runtime assertion):
//
//	Stats.AdapterCount = len(decision.AdapterSet)
//	Stats.SuccessCount + Stats.ErrorCount = Stats.AdapterCount
type Stats struct {
	// AdapterCount is the total number of adapters in the decision.AdapterSet.
	AdapterCount int `json:"adapter_count"`
	// SuccessCount is the number of adapters that returned nil error.
	SuccessCount int `json:"success_count"`
	// ErrorCount is the number of adapters that returned a non-nil error.
	ErrorCount int `json:"error_count"`
	// DedupDropped is the number of docs removed during deduplication.
	DedupDropped int `json:"dedup_dropped"`
	// ElapsedSeconds is the wall-clock time from Dispatch entry to sort completion.
	ElapsedSeconds float64 `json:"elapsed_seconds"`
}
