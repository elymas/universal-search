// Package router — sentinel errors for the Intent Router.
// SPEC-IR-001: REQ-IR-005, REQ-IR-007, REQ-IR-003, REQ-IR-002.
package router

import "errors"

// Sentinel errors returned by Router.New and Router.Classify.
//
// In the normal Classify path, ErrLLMTimeout and ErrLLMUnavailable are NOT
// surfaced to the caller — the router degrades to a rule-based result and
// records the situation via Metadata flags + an outcome counter. The two
// sentinels remain exported so internal helpers (and tests) can categorise
// LLM failure modes via errors.Is.
var (
	// ErrInvalidQuery is returned by Classify when RouterQuery.Text is empty
	// or contains only Unicode whitespace runes (REQ-IR-005).
	ErrInvalidQuery = errors.New("router: invalid query (empty or whitespace-only Text)")

	// ErrLLMTimeout signals that the LLM-fallback call exceeded the 2s deadline
	// (REQ-IR-007). Used internally; the public Classify returns nil and falls
	// back to rule-based instead of surfacing this error.
	ErrLLMTimeout = errors.New("router: LLM classification deadline exceeded")

	// ErrLLMUnavailable signals that all providers in the LLM classify chain
	// are unavailable (circuit-open) (REQ-IR-003). Used internally; Classify
	// returns nil and falls back to rule-based.
	ErrLLMUnavailable = errors.New("router: LLM unavailable (all providers failed)")

	// ErrAdapterRegistryEmpty is returned by router.New when the supplied
	// adapter registry has zero registered adapters at construction time.
	ErrAdapterRegistryEmpty = errors.New("router: adapter registry is empty")

	// ErrLLMParse signals that the LLM response failed JSON parsing or enum
	// validation. Used internally; Classify returns nil and falls back to
	// rule-based (REQ-IR-002 + S-17).
	ErrLLMParse = errors.New("router: LLM response failed to parse")
)

// errIs is a thin wrapper around errors.Is, used in router.go to avoid
// importing errors there (keeps router.go's import surface minimal).
func errIs(err, target error) bool { return errors.Is(err, target) }
