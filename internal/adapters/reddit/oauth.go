// Package reddit — OAuth2 app-only token acquisition and caching.
// REQ-ADP-001a-001/003/007: client_credentials grant, locked-refresh token cache.
package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultOAuthURL is the Reddit token endpoint for app-only OAuth.
	defaultOAuthURL = "https://www.reddit.com/api/v1/access_token"

	// tokenExpirySafetyMargin is subtracted from expires_in to refresh early
	// and avoid a search racing against nominal token expiry.
	tokenExpirySafetyMargin = 60 * time.Second
)

// tokenResponse is the JSON response from the Reddit token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// tokenCache guards a cached bearer token with mutex synchronization so that
// N concurrent callers share a single token POST (NFR-ADP-001a-001).
//
// @MX:ANCHOR: [AUTO] Shared mutable state guard; concurrency-correctness
// contract for all Search calls. A broken lock here causes token-POST stampede
// and/or data races across all fanout goroutines.
// @MX:REASON: all concurrent Search goroutines pass through get(); the mutex
// guarantees exactly one token POST under first-time or expired conditions.
// @MX:SPEC: SPEC-ADP-001a
type tokenCache struct {
	mu     sync.Mutex
	token  string
	expiry time.Time
}

// get returns a valid bearer token. If the cached token is missing or expired,
// it calls refreshFn under the lock to acquire a new one.
// The mutex guarantees EXACTLY ONE token POST under concurrent first-time callers
// (NFR-ADP-001a-001): the first caller to win the lock acquires the token;
// all others observe the freshly-cached token after the lock releases.
func (tc *tokenCache) get(ctx context.Context, refreshFn func(context.Context) (string, time.Time, error)) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Token is valid: return cached.
	if tc.token != "" && time.Now().Before(tc.expiry) {
		return tc.token, nil
	}

	// Acquire new token.
	tok, exp, err := refreshFn(ctx)
	if err != nil {
		return "", err
	}
	tc.token = tok
	tc.expiry = exp
	return tok, nil
}

// invalidate clears the cached token so the next get() triggers a refresh.
// Called when a search request returns 401 (REQ-ADP-001a-004).
func (tc *tokenCache) invalidate() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.token = ""
	tc.expiry = time.Time{}
}

// acquireToken POSTs to the Reddit OAuth token endpoint with HTTP Basic auth
// and grant_type=client_credentials. Returns the bearer token and its expiry.
//
// When credentials are absent (empty ClientID/ClientSecret), returns a dummy
// token "skip-auth" with a far-future expiry. This supports the SkipAuthCheck
// test seam: tests construct the adapter without real credentials and stub
// the search endpoint via Options.BaseURL.
//
// @MX:WARN: [AUTO] Outbound network call carrying secret credentials.
// @MX:REASON: leaking client_secret or token into error/log strings is a
// security incident; keep causes sentinel-only.
// @MX:SPEC: SPEC-ADP-001a
func (a *Adapter) acquireToken(ctx context.Context) (string, time.Time, error) {
	// No credentials: return a dummy token for the SkipAuthCheck test seam.
	// The search endpoint stub doesn't validate the token, so this is safe.
	if a.clientID == "" || a.clientSecret == "" {
		return "skip-auth", time.Now().Add(24 * time.Hour), nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.oauthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			HTTPStatus: 0,
			Cause:      fmt.Errorf("reddit: failed to create token request: %w", err),
		}
	}

	// REQ-ADP-001a-006a: custom UA + Accept on every outbound request.
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(a.clientID, a.clientSecret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			HTTPStatus: 0,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Token endpoint returned 401 or 403: bad credentials, not retryable.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryPermanent,
			HTTPStatus: resp.StatusCode,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	// 5xx or other server error: transient, retryable.
	if resp.StatusCode >= 500 {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			HTTPStatus: resp.StatusCode,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			HTTPStatus: resp.StatusCode,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	if tokenResp.AccessToken == "" {
		return "", time.Time{}, &types.SourceError{
			Adapter:    "reddit",
			Category:   types.CategoryUnavailable,
			Cause:      ErrTokenAcquisitionFailed,
		}
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - tokenExpirySafetyMargin)
	return tokenResp.AccessToken, expiry, nil
}
