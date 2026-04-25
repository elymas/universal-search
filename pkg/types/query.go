// Package types — Query value type for search invocations.
// REQ-CORE-002.
package types

import "time"

// Query is the normalized search request passed to Adapter.Search.
//
// Query is opaque to the registry and to fanout — adapters interpret Filters
// and Cursor in source-specific ways. The orchestrator's responsibility is to
// honour Deadline by deriving a context.WithDeadline upstream and to bound
// MaxResults so adapters do not need to enforce limits themselves.
//
// @MX:NOTE: [AUTO] Filters is opaque to the registry; each adapter interprets
// keys per its own SPEC. Cursor likewise is adapter-specific (Reddit uses
// "after", HN uses numericFilters, etc).
// @MX:SPEC: SPEC-CORE-001
type Query struct {
	// Text is the user's free-text query.
	Text string
	// Lang is the BCP-47 preferred result language (e.g., "ko", "en", "ja");
	// empty string means no preference.
	Lang string
	// MaxResults caps the number of results returned. Zero means
	// adapter-default (see Capabilities.DefaultMaxResults).
	MaxResults int
	// Filters carries adapter-specific constraints (e.g., date ranges).
	Filters []Filter
	// Cursor is an opaque adapter-specific pagination token. Empty for the
	// first page.
	Cursor string
	// Deadline is a soft deadline; the orchestrator SHOULD honour this via
	// context.WithDeadline before invoking Search.
	Deadline time.Time
}

// Filter is one adapter-specific constraint expressed as a Key/Value pair.
// The interpretation of Key is adapter-specific (e.g., "date_from",
// "subreddit", "lang") and is documented in each adapter SPEC.
type Filter struct {
	Key   string
	Value string
}
