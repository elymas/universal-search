// Package naver — score constant for Naver adapter.
// REQ-ADP8-005: Naver does not expose an engagement metric comparable to
// Reddit upvotes; all docs receive a fixed neutral score of 0.5.
package naver

// defaultScore is the fixed NormalizedDoc.Score assigned to every Naver result.
// Naver search APIs return no engagement metric (no likes, shares, or view count
// suitable for cross-source normalization), so 0.5 (the neutral center of [0.0,1.0])
// is used for all verticals.
//
// @MX:NOTE: [AUTO] Score=0.5 constant for all Naver results. No engagement signal
// available from Naver search APIs. See SPEC-ADP-008 §5 for rationale.
// Open question: DataLab ratio could be used for trend-based scoring in a future SPEC.
// @MX:SPEC: SPEC-ADP-008
const defaultScore = 0.5
