package idx5

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// backgroundCtx is used when ctx is nil.
var backgroundCtx = context.Background()

// RevalidateCitations checks citation URLs and strips those returning 4xx.
// REQ-IDX5-004: lazy default (no-op), eager_top_n (HEAD probe top-N),
// eager_all (HEAD probe all). 4xx -> strip, timeout/5xx -> keep.
//
// @MX:WARN: [AUTO] eager modes spawn parallel HTTP HEAD requests with per-probe timeout.
// @MX:REASON: Citation re-validation must not add >200ms to response latency.
// @MX:SPEC: SPEC-IDX-005
func RevalidateCitations(ctx context.Context, citations []Citation, cfg Config) ([]Citation, int) {
	if ctx == nil {
		ctx = backgroundCtx
	}
	if cfg.CitationRevalidationMode == "lazy" || len(citations) == 0 {
		return citations, 0
	}

	toProbe := citations
	if cfg.CitationRevalidationMode == "eager_top_n" && len(citations) > cfg.EagerTopN {
		toProbe = citations[:cfg.EagerTopN]
	}

	// Probe URLs in parallel
	type probeResult struct {
		index int
		is4xx bool
	}

	var wg sync.WaitGroup
	results := make([]probeResult, len(toProbe))
	sem := make(chan struct{}, 10) // limit concurrency

	for i, cit := range toProbe {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			probeCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()

			req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, url, nil)
			if err != nil {
				results[idx] = probeResult{index: idx, is4xx: false}
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// timeout or network error -> keep
				results[idx] = probeResult{index: idx, is4xx: false}
				return
			}
			defer resp.Body.Close()

			is4xx := resp.StatusCode >= 400 && resp.StatusCode < 500
			results[idx] = probeResult{index: idx, is4xx: is4xx}
		}(i, cit.URL)
	}
	wg.Wait()

	// Build stripped set
	strippedSet := make(map[int]bool)
	strippedCount := 0
	for _, r := range results {
		if r.is4xx {
			strippedSet[r.index] = true
			strippedCount++
		}
	}

	// Filter citations
	filtered := make([]Citation, 0, len(citations))
	for i, cit := range citations {
		if !strippedSet[i] {
			filtered = append(filtered, cit)
		}
	}

	return filtered, strippedCount
}
