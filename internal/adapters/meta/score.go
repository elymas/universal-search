// Package meta — score computation for Threads search results.
// REQ-ADP10-005: neutralScore returns constant 0.5 (no engagement signal).
package meta

// @MX:NOTE: [AUTO] neutralScore returns 0.5 (neutral) instead of ADP-006's Tanh
// normalization because the keyword_search response has no engagement counts
// (like_count, repost_count). Score==0.0 means unscored in the NormalizedDoc
// type contract, so 0.5 is the explicit "scored, neutral" value.
// @MX:SPEC: SPEC-ADP-010
func neutralScore() float64 { return 0.5 }
