// Package access — Phase 3: standard HTTP GET.
//
// REQ-CACHE-004: Standard GET via stdlib http.Client with pinnedIPDialer,
// MaxBodyBytes cap, redirect validation, and WAF-pattern escalation detection.
package access

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// wafHeaders are the response header prefixes/names that indicate a WAF
// is active. Their presence (combined with 403/503) triggers escalation to Phase 4.
var wafHeaders = []string{
	"cf-ray",       // Cloudflare
	"x-akamai-",    // Akamai
	"x-served-by",  // Fastly
}

// phase3Get performs a standard HTTP GET and returns FetchedContent on success.
//
// Escalation signals embedded in PhaseAttempt:
//   - isTLSError = true when a TLS handshake error occurs
//   - isWAF = true when a 403/503 with WAF header is received
func phase3Get(
	ctx context.Context,
	rawURL string,
	fopts FetchOptions,
	opts Options,
) (*FetchedContent, *PhaseAttempt, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL", Cause: err}
	}

	userAgent := fopts.UserAgent
	if userAgent == "" {
		userAgent = "usearch/1.0"
	}

	transport, err := buildTransport(ctx, u, opts, fopts, nil)
	if err != nil {
		a := &PhaseAttempt{Phase: 3, Outcome: "blocked"}
		return nil, a, err
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.RedirectMaxHops {
				return fmt.Errorf("too many redirects: %d hops exceeded limit of %d",
					len(via), opts.RedirectMaxHops)
			}
			if err := validateRedirect(req.URL, opts, fopts, len(via)); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, &FetchError{Category: CategoryUnavailable, Reason: "request build failed", Cause: err}
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		attempt := &PhaseAttempt{Phase: 3}
		if isTLSError(err) {
			attempt.isTLSError = true
			attempt.Outcome = "failure"
		} else {
			attempt.Outcome = "failure"
		}
		return nil, attempt, &FetchError{Category: CategoryUnavailable, Reason: "GET failed", Cause: err}
	}
	defer resp.Body.Close()

	// Detect WAF patterns that should trigger Phase 4 escalation.
	if isWAFResponse(resp) {
		attempt := &PhaseAttempt{Phase: 3, isWAF: true, Outcome: "failure"}
		return nil, attempt, &FetchError{
			Category:   CategoryUnavailable,
			Reason:     "WAF-gated response",
			HTTPStatus: resp.StatusCode,
		}
	}

	switch {
	case resp.StatusCode == 200:
		body, err := readBody(resp, opts.MaxBodyBytes)
		if err != nil {
			return nil, nil, &FetchError{Category: CategoryUnavailable, Reason: "body read failed", Cause: err}
		}
		return &FetchedContent{
			URL:         resp.Request.URL.String(),
			Body:        body,
			ContentType: resp.Header.Get("Content-Type"),
			StatusCode:  200,
			FetchedAt:   time.Now().UTC(),
		}, nil, nil

	case resp.StatusCode == 429:
		return nil, nil, &FetchError{Category: CategoryRateLimited, Reason: "rate limited", HTTPStatus: 429}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, nil, &FetchError{
			Category:   CategoryPermanent,
			Reason:     fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}

	default: // 5xx and other non-200 non-4xx
		attempt := &PhaseAttempt{Phase: 3, isTLSError: false, Outcome: "failure"}
		return nil, attempt, &FetchError{
			Category:   CategoryUnavailable,
			Reason:     fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}
	}
}

// buildTransport creates an http.Transport using the pinnedIPDialer when
// private networks are not allowed. tlsCfg overrides the TLS configuration
// (nil → use Go's default TLS config).
func buildTransport(
	ctx context.Context,
	u *url.URL,
	opts Options,
	fopts FetchOptions,
	tlsCfg *tls.Config,
) (*http.Transport, error) {
	if opts.AllowPrivateNetworks || fopts.AllowPrivateNetworks {
		// Test mode: use default dialer, no IP pinning.
		t := http.DefaultTransport.(*http.Transport).Clone()
		if tlsCfg != nil {
			t.TLSClientConfig = tlsCfg
		}
		return t, nil
	}

	// Resolve and pin the IP for DNS-rebind mitigation.
	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(resolveCtx, u.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   "DNS lookup failed",
			Cause:    err,
		}
	}

	pinnedIP := ips[0].IP
	if isPrivateOrLoopback(pinnedIP) {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   "private/loopback IP resolved",
		}
	}

	dialFn := dialContextWithPinnedIP(pinnedIP.String())
	t := &http.Transport{
		DialContext:           dialFn,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   10,
	}
	if tlsCfg != nil {
		t.TLSClientConfig = tlsCfg
	}
	return t, nil
}

// isWAFResponse returns true when the response looks like it is WAF-gated:
// status 403 or 503 AND at least one WAF header is present.
func isWAFResponse(resp *http.Response) bool {
	if resp.StatusCode != 403 && resp.StatusCode != 503 {
		return false
	}
	for _, hdr := range wafHeaders {
		for k := range resp.Header {
			if strings.HasPrefix(strings.ToLower(k), hdr) {
				return true
			}
		}
	}
	return false
}

// isTLSError returns true when the error originates from a TLS handshake failure.
func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "tls:") ||
		strings.Contains(errStr, "TLS") ||
		strings.Contains(errStr, "handshake") ||
		strings.Contains(errStr, "x509") ||
		strings.Contains(errStr, "certificate")
}

// readBody reads at most maxBytes from resp.Body.
func readBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBodyBytes
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}
