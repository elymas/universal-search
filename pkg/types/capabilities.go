// Package types — Capabilities descriptor and DocType enum.
// REQ-CORE-002.
package types

// DocType enumerates the canonical document categories. Adapters return
// results tagged with one of these values; the Intent Router (SPEC-IR-001)
// uses the set per adapter to drive query routing.
type DocType string

// DocType constants. Stable: Reddit/X/Bluesky → DocTypePost or DocTypeSocial;
// HN/news → DocTypeArticle; arXiv → DocTypePaper; YouTube → DocTypeVideo;
// GitHub repo → DocTypeRepo; GitHub issue → DocTypeIssue; everything else →
// DocTypeOther.
const (
	DocTypeArticle DocType = "article"
	DocTypePost    DocType = "post"
	DocTypePaper   DocType = "paper"
	DocTypeVideo   DocType = "video"
	DocTypeRepo    DocType = "repo"
	DocTypeIssue   DocType = "issue"
	DocTypeSocial  DocType = "social"
	DocTypeOther   DocType = "other"
)

// Capabilities describes the static metadata an Adapter exposes to the
// orchestrator. Returned by Adapter.Capabilities() and consumed by the
// Intent Router (SPEC-IR-001) at startup. MUST be deterministic — calling
// Capabilities twice on the same adapter returns equal values.
//
// Field additions to this struct are non-breaking; field removals or type
// changes require a major-version bump of pkg/types per the SDK boundary
// commitment in structure.md.
//
// @MX:NOTE: [AUTO] Field additions are non-breaking; the Intent Router
// reads these fields at startup to build the routing table. Renaming or
// removing fields breaks every adapter implementation.
// @MX:SPEC: SPEC-CORE-001
type Capabilities struct {
	// SourceID is the stable adapter identifier (matches Adapter.Name()).
	SourceID string
	// DisplayName is a human-readable label (e.g., "Hacker News").
	DisplayName string
	// DocTypes lists the document categories this adapter produces.
	DocTypes []DocType
	// SupportedLangs lists BCP-47 language codes the adapter can serve;
	// empty slice means language-agnostic.
	SupportedLangs []string
	// SupportsSince reports whether the adapter accepts a time-range filter.
	SupportsSince bool
	// RequiresAuth reports whether the adapter needs environment-provided
	// credentials at registration time. AuthEnvVars enumerates them.
	RequiresAuth bool
	// AuthEnvVars lists the environment variables that MUST be set when
	// RequiresAuth is true. The registry validates this at registration time.
	AuthEnvVars []string
	// RateLimitPerMin is the upstream rate limit (calls/min); 0 means unknown.
	RateLimitPerMin int
	// DefaultMaxResults is the fallback when Query.MaxResults is zero.
	DefaultMaxResults int
	// Notes carries free-form metadata for operators.
	Notes string
}
