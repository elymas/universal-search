// Package access — Phase 2: HEAD probe + robots.txt check.
//
// REQ-CACHE-003: Phase 2 issues HEAD + fetches robots.txt in parallel,
// parses via temoto/robotstxt, and either allows or blocks escalation.
package access

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// phase2Probe runs the Phase 2 HEAD probe and robots.txt check.
//
// Returns (content, nil) on success (robots allow), where content carries
// metadata from the HEAD response.
// Returns (nil, *FetchError{CategoryBlocked}) when robots.txt disallows.
// Returns (nil, ErrPhaseNotApplicable) when both SkipHEADProbe and SkipRobotsTxt
// are set (handled by cascade as "skipped").
func phase2Probe(
	ctx context.Context,
	rawURL string,
	fopts FetchOptions,
	opts Options,
	rc *robotsCache,
) (*FetchedContent, error) {
	// When both skips are active, Phase 2 is entirely skipped.
	if fopts.SkipHEADProbe && fopts.SkipRobotsTxt {
		return nil, ErrPhaseNotApplicable
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL in phase2", Cause: err}
	}

	userAgent := fopts.UserAgent
	if userAgent == "" {
		userAgent = "usearch/1.0"
	}

	// Check robots.txt unless explicitly skipped.
	if !fopts.SkipRobotsTxt {
		allowed, err := rc.isAllowed(ctx, u.Scheme, u.Host, u.Path, userAgent)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, &FetchError{
				Category: CategoryBlocked,
				Reason:   "robots.txt disallow",
			}
		}
	}

	// HEAD probe (skippable for test servers).
	if fopts.SkipHEADProbe {
		return &FetchedContent{
			URL:       rawURL,
			FetchedAt: time.Now().UTC(),
		}, nil
	}

	content, err := doHEADProbe(ctx, rawURL, userAgent)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// doHEADProbe issues an HTTP HEAD request and captures metadata.
func doHEADProbe(ctx context.Context, rawURL, userAgent string) (*FetchedContent, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, &FetchError{Category: CategoryUnavailable, Reason: "HEAD request build failed", Cause: err}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Transient network error → escalate to Phase 3.
		return nil, &FetchError{Category: CategoryUnavailable, Reason: "HEAD probe failed", Cause: err}
	}
	defer resp.Body.Close()

	headers := map[string]string{}
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		headers["Cache-Control"] = cc
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		headers["Content-Type"] = ct
	}

	return &FetchedContent{
		URL:         rawURL,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
		FetchedAt:   time.Now().UTC(),
		Headers:     headers,
	}, nil
}
