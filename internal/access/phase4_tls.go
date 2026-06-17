// Package access — Phase 4: TLS-aware HTTP GET.
//
// REQ-CACHE-005: Phase 4 retries with custom TLS config and a browser-shaped
// User-Agent. Detects JS-challenge responses and signals escalation to Phase 5.
package access

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// browserUserAgent mimics Chrome 130 on macOS to avoid basic bot detection.
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// jsChallengePatterns are body substrings that indicate a JS challenge page.
var jsChallengePatterns = []string{
	"cf-please-stand-by", // Cloudflare "Checking your browser"
	"<noscript>",         // Generic JS-required page
	"captcha-bypass",     // Generic captcha challenge
	"checking if the site connection is secure", // Cloudflare
}

// phase4TLS performs a TLS-tuned GET with a browser User-Agent.
//
// Escalation signals in the returned PhaseAttempt:
//   - isJSChallenge = true when a JS challenge body is detected.
func phase4TLS(
	ctx context.Context,
	rawURL string,
	fopts FetchOptions,
	opts Options,
) (*FetchedContent, *PhaseAttempt, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL", Cause: err}
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		NextProtos: []string{"h2", "http/1.1"},
		ServerName: u.Hostname(),
	}

	transport, err := buildTransport(ctx, u, opts, fopts, tlsCfg)
	if err != nil {
		a := &PhaseAttempt{Phase: 4, Outcome: "blocked"}
		return nil, a, err
	}

	maxHops := opts.RedirectMaxHops
	if maxHops == 0 {
		maxHops = defaultRedirectMaxHops
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxHops {
				return fmt.Errorf("too many redirects: %d", len(via))
			}
			return validateRedirect(req.URL, opts, fopts, len(via))
		},
	}

	ua := fopts.UserAgent
	if ua == "" {
		ua = browserUserAgent
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, &FetchError{Category: CategoryUnavailable, Reason: "request build failed", Cause: err}
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		attempt := &PhaseAttempt{Phase: 4}
		if isTLSError(err) {
			attempt.isTLSError = true
		}
		attempt.Outcome = "failure"
		return nil, attempt, &FetchError{Category: CategoryUnavailable, Reason: "TLS GET failed", Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	maxBytes := opts.MaxBodyBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, nil, &FetchError{Category: CategoryUnavailable, Reason: "body read failed", Cause: err}
	}

	// Detect JS challenge patterns in the response body.
	if (resp.StatusCode == 403 || resp.StatusCode == 503 || resp.StatusCode == 200) &&
		containsJSChallenge(body) {
		attempt := &PhaseAttempt{Phase: 4, isJSChallenge: true, Outcome: "failure"}
		return nil, attempt, &FetchError{
			Category:   CategoryUnavailable,
			Reason:     "js-challenge",
			HTTPStatus: resp.StatusCode,
		}
	}

	if resp.StatusCode == 200 {
		// SPEC-ACC-001: classify the Phase 4 body. Phase 4 may inherit a
		// top profile from Phase 3 (threaded via the cascade), but for the
		// direct-call path hit is nil — validatePage handles that.
		hits := detectProfiles(resp, body)
		topHit := topHitOrNil(hits)
		verdict := validatePage(resp, body, topHit)
		if verdict == VerdictChallenge || verdict == VerdictBlocked {
			attempt := &PhaseAttempt{
				Phase:   4,
				Outcome: "failure",
				verdict: verdict,
			}
			return nil, attempt, &FetchError{
				Category:   CategoryUnavailable,
				Reason:     "silent-200 challenge (phase 4)",
				HTTPStatus: 200,
				verdict:    verdict,
			}
		}
		return &FetchedContent{
			URL:         resp.Request.URL.String(),
			Body:        body,
			ContentType: resp.Header.Get("Content-Type"),
			StatusCode:  200,
			FetchedAt:   time.Now().UTC(),
		}, nil, nil
	}

	if resp.StatusCode == 429 {
		return nil, nil, &FetchError{Category: CategoryRateLimited, Reason: "rate limited", HTTPStatus: 429}
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, nil, &FetchError{
			Category:   CategoryPermanent,
			Reason:     fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}
	}

	return nil, nil, &FetchError{
		Category:   CategoryUnavailable,
		Reason:     fmt.Sprintf("HTTP %d", resp.StatusCode),
		HTTPStatus: resp.StatusCode,
	}
}

// containsJSChallenge checks whether the body matches any known JS challenge pattern.
func containsJSChallenge(body []byte) bool {
	bodyLower := bytes.ToLower(body)
	for _, pattern := range jsChallengePatterns {
		if bytes.Contains(bodyLower, []byte(strings.ToLower(pattern))) {
			return true
		}
	}
	return false
}
