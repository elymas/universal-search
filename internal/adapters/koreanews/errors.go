// Package koreanews — error sentinels for the composite Korean news adapter.
// SPEC-ADP-009: ErrInvalidQuery, ErrDaumDisabled, ErrKNCSidecarDown, ErrEmptyRSSFeedList.
package koreanews

import "errors"

// ErrInvalidQuery is returned when the query text is empty or contains only
// Unicode whitespace runes. Wrapped in *types.SourceError{CategoryPermanent}.
var ErrInvalidQuery = errors.New("koreanews: query text empty or whitespace-only")

// ErrDaumDisabled is returned by the Daum sub-source stub. The stub returns
// this error regardless of the USEARCH_ADP009_DAUM_ENABLED flag value because
// Daum's robots.txt explicitly forbids all crawlers:
//
//	https://search.daum.net/robots.txt → User-agent: * \n Disallow: /
//
// Enabling the Daum path requires a future SPEC-ADP-009-DAUM with explicit
// Kakao authorisation OR operator-attested compliance review.
//
// @MX:NOTE: [AUTO] Sentinel documents the legal posture. Required reading
// for any future SPEC author touching the daum sub-source.
// @MX:SPEC: SPEC-ADP-009
var ErrDaumDisabled = errors.New("koreanews: daum subsource is disabled in v0.1 per robots.txt; enable via future SPEC-ADP-009-DAUM with legal review")

// ErrKNCSidecarDown is returned when the KoreaNewsCrawler Python sidecar
// returns HTTP 503 (the default stub state) or is connection-refused.
// The sidecar at services/koreanews/ is a scaffold only in v0.1.
// Full implementation deferred to future SPEC-ADP-009-KNC.
var ErrKNCSidecarDown = errors.New("koreanews: knc sidecar unreachable (default port 8002)")

// ErrEmptyRSSFeedList is returned when RSS is enabled but no feed URLs are
// configured. Wrapped in *types.SourceError{CategoryPermanent}.
// Set USEARCH_ADP009_RSS_FEEDS to a JSON array or comma-list of feed URLs.
var ErrEmptyRSSFeedList = errors.New("koreanews: rss enabled but no feed URLs configured (set USEARCH_ADP009_RSS_FEEDS)")
