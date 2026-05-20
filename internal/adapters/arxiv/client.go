package arxiv

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// allowedRedirectHosts is the set of hostnames that may appear in redirect
// Location headers. Cross-domain redirects to any other host are blocked.
var allowedRedirectHosts = map[string]bool{
	"export.arxiv.org": true,
	"arxiv.org":        true,
}

// redirectAllowlist returns a function suitable for use as a host-allowlist
// checker. It returns an error containing "cross-domain redirect" for blocked
// hosts, so callers can detect this specific failure category.
func redirectAllowlist() func(host string) error {
	return func(host string) error {
		if allowedRedirectHosts[host] {
			return nil
		}
		return fmt.Errorf("arxiv: cross-domain redirect to host %q denied", host)
	}
}

// newDefaultClient constructs the default *http.Client used by the adapter.
// It applies a 10-second timeout, wraps the transport with reqid propagation,
// and installs the redirect allowlist (max 3 hops).
func newDefaultClient() *http.Client {
	checkHost := redirectAllowlist()
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: reqid.NewTransport(http.DefaultTransport),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("arxiv: too many redirects (max 3)")
			}
			return checkHost(req.URL.Hostname())
		},
	}
}

// doRequest executes an HTTP GET request with the required headers.
func doRequest(ctx context.Context, client *http.Client, rawURL, userAgent string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/atom+xml")
	return client.Do(req)
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. For network errors (status==0), Category is Unavailable.
func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
	var cat types.Category
	switch {
	case status == 0:
		cat = types.CategoryUnavailable
	case status == http.StatusTooManyRequests:
		cat = types.CategoryRateLimited
	case status >= 400 && status < 500:
		cat = types.CategoryPermanent
	case status >= 500:
		cat = types.CategoryUnavailable
	default:
		cat = types.CategoryUnknown
	}
	return &types.SourceError{
		Adapter:    "arxiv",
		Category:   cat,
		HTTPStatus: status,
		Cause:      cause,
		RetryAfter: retryAfter,
	}
}
