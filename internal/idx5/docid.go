package idx5

// CacheDocID returns a deterministic doc_id for a cached answer.
// REQ-IDX5-007: format = "answer-cache:<queryHash>:<team_id>"
// The team_id suffix prevents cross-tenant doc_id collision.
//
// @MX:ANCHOR: [AUTO] doc_id construction includes team_id for security.
// @MX:REASON: Cross-tenant doc_id collision would allow cache leak (NFR-IDX5-004).
// @MX:SPEC: SPEC-IDX-005
func CacheDocID(queryHash, teamID string) string {
	return "answer-cache:" + queryHash + ":" + teamID
}
