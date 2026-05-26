package idx5

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CacheHeader represents the X-Cache response header value.
type CacheHeader string

const (
	CacheHeaderHIT      CacheHeader = "HIT"
	CacheHeaderSoftHit  CacheHeader = "SOFT-HIT"
	CacheHeaderMISS     CacheHeader = "MISS"
	CacheHeaderBypassed CacheHeader = "BYPASSED"
)

// ServeCached writes the cached answer to the HTTP response.
// REQ-IDX5-002: reconstruct SynthesizeResponse JSON + attach cache headers.
func ServeCached(w http.ResponseWriter, ca *CachedAnswer, staleness Staleness, extraHeaders map[string]string) {
	header := stalenessToCacheHeader(staleness)

	// For hard-stale, we serve MISS (triggers fanout in middleware)
	if staleness == HardStale {
		w.Header().Set("X-Cache", string(CacheHeaderMISS))
		return
	}

	w.Header().Set("X-Cache", string(header))
	w.Header().Set("X-Cache-Age-Seconds", fmt.Sprintf("%d", int(time.Since(ca.CreatedAt).Seconds())))
	w.Header().Set("X-Cache-Score", fmt.Sprintf("%.2f", ca.Similarity))

	// Apply extra headers (e.g., X-Cache-Citation-Stale)
	for k, v := range extraHeaders {
		w.Header().Set(k, v)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Write the cached response JSON directly
	_, _ = w.Write([]byte(ca.ResponseJSON))
}

// ServeMISS writes a MISS response (no cache headers with score details).
func ServeMISS(w http.ResponseWriter) {
	w.Header().Set("X-Cache", string(CacheHeaderMISS))
}

// ServeBypassed writes a BYPASSED response.
func ServeBypassed(w http.ResponseWriter) {
	w.Header().Set("X-Cache", string(CacheHeaderBypassed))
}

// CacheServeResult holds the result of a cache serve operation for observability.
type CacheServeResult struct {
	DocID   string
	Outcome LookupOutcome
	Age     time.Duration
	Score   float64
	Latency time.Duration
}

// MarshalJSONForResponse ensures the response body is valid JSON.
func MarshalResponse(v any) ([]byte, error) {
	return json.Marshal(v)
}

func stalenessToCacheHeader(s Staleness) CacheHeader {
	switch s {
	case Fresh:
		return CacheHeaderHIT
	case SoftStale:
		return CacheHeaderSoftHit
	default:
		return CacheHeaderMISS
	}
}
