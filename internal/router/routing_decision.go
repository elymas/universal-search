// Package router — RoutingDecision is the typed output of Router.Classify.
// SPEC-IR-001: REQ-IR-001, REQ-IR-006.
package router

import "encoding/json"

// RoutingDecision is the value returned by Router.Classify. It is the contract
// downstream M2 SPECs (FAN-001 fanout, CLI-001, SYN-001) consume.
//
// Metadata key allowlist (documented in SPEC-IR-001 research §10.4):
//
//	hangul_ratio          float64  always set
//	particle_density      float64  always set
//	rule_triggers         []string when Source==SourceRuleBased
//	rule_confidence       float64  when Source==SourceLLMFallback (debug aid)
//	llm_rationale         string   when Source==SourceLLMFallback (≤200 chars)
//	llm_unavailable       bool     when LLM was needed but circuit was open
//	llm_timeout           bool     when LLM exceeded the 2s deadline
//	degraded_confidence   bool     when confidence was rule-based after LLM failure
//	lang_override         bool     when RouterQuery.Lang was non-empty
//	adapter_set_fallback  bool     when capability intersection was empty
//	llm_parse_error       string   when LLM response failed to parse
type RoutingDecision struct {
	// Category is the classification outcome.
	Category Category `json:"category"`
	// Confidence is in [0.0, 1.0]; clamped before return.
	Confidence float64 `json:"confidence"`
	// AdapterSet is the lexicographically-sorted set of adapter names eligible
	// to serve this query.
	AdapterSet []string `json:"adapter_set"`
	// Lang is the BCP-47 detected (or caller-overridden) language tag.
	Lang string `json:"lang"`
	// Source records how the decision was reached.
	Source ClassificationSource `json:"source"`
	// Metadata carries diagnostic fields per the documented key allowlist.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON serialises a RoutingDecision with stable field ordering and
// guarantees AdapterSet is rendered as `[]` (not `null`) when empty.
func (d RoutingDecision) MarshalJSON() ([]byte, error) {
	type alias RoutingDecision
	out := alias(d)
	if out.AdapterSet == nil {
		out.AdapterSet = []string{}
	}
	return json.Marshal(out)
}
