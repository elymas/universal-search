// Package access — per-host robots.txt cache.
//
// REQ-CACHE-003: Phase 2 fetches robots.txt with a 5s timeout, parses via
// temoto/robotstxt, and caches per-host for Options.RobotsTTL (default 24h).
// RFC 9309 semantics: 4xx → allow all; 5xx or network error → disallow all.
package access

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// robotsCacheEntry holds a parsed *robotstxt.RobotsData with its expiry.
type robotsCacheEntry struct {
	data      *robotstxt.RobotsData
	fetchedAt time.Time
	ttl       time.Duration
}

// expired reports whether the cache entry has passed its TTL.
func (e *robotsCacheEntry) expired() bool {
	return time.Since(e.fetchedAt) > e.ttl
}

// robotsCache is a concurrent-safe per-host cache of parsed robots.txt data.
//
// @MX:WARN: [AUTO] sync.Map is used for concurrent robots.txt caching;
// TTL eviction happens on read (lazy). Under million-host load, memory is
// unbounded — future SPEC-CACHE-001a will add an LRU cap.
// @MX:REASON: sync.Map goroutine-safety is required by REQ-CACHE-012.
type robotsCache struct {
	mu    sync.RWMutex
	store map[string]*robotsCacheEntry
	ttl   time.Duration
}

// newRobotsCache creates a robotsCache with the given per-host TTL.
func newRobotsCache(ttl time.Duration) *robotsCache {
	if ttl == 0 {
		ttl = defaultRobotsTTL
	}
	return &robotsCache{
		store: make(map[string]*robotsCacheEntry),
		ttl:   ttl,
	}
}

// get returns the cached data for host, reporting whether it was found and
// is still within its TTL.
func (c *robotsCache) get(host string) (*robotstxt.RobotsData, bool) {
	c.mu.RLock()
	entry, ok := c.store[host]
	c.mu.RUnlock()
	if !ok || entry.expired() {
		return nil, false
	}
	return entry.data, true
}

// put stores parsed robots.txt data for host.
func (c *robotsCache) put(host string, data *robotstxt.RobotsData) {
	c.mu.Lock()
	c.store[host] = &robotsCacheEntry{
		data:      data,
		fetchedAt: time.Now(),
		ttl:       c.ttl,
	}
	c.mu.Unlock()
}

// isAllowed checks whether the given URL path is allowed for the given
// user agent, fetching and caching robots.txt if necessary.
//
// Returns (true, nil) when allowed.
// Returns (false, *FetchError{CategoryBlocked}) when disallowed.
// Returns (false, *FetchError{...}) on infrastructure error.
func (c *robotsCache) isAllowed(ctx context.Context, scheme, host, path, userAgent string) (bool, error) {
	data, ok := c.get(host)
	if !ok {
		var err error
		data, err = fetchRobotsTxt(ctx, scheme, host)
		if err != nil {
			return false, err
		}
		c.put(host, data)
	}

	group := data.FindGroup(userAgent)
	if group == nil || group.Test(path) {
		return true, nil
	}
	return false, &FetchError{
		Category: CategoryBlocked,
		Reason:   "robots.txt disallow",
	}
}

// fetchRobotsTxt retrieves and parses the robots.txt file for a given host.
// RFC 9309 §2.3.1 semantics:
//   - HTTP 4xx (not 429): allow all
//   - HTTP 5xx or network error: disallow all
//   - HTTP 429 (too many requests): treat as unavailable → disallow all
func fetchRobotsTxt(ctx context.Context, scheme, host string) (*robotstxt.RobotsData, error) {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", scheme, host)

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, robotsURL, nil)
	if err != nil {
		// Disallow all on construction failure.
		return disallowAll(), nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Network error → disallow all per RFC 9309 §2.3.1.
		return disallowAll(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429:
		// 4xx (not 429): allow all per RFC 9309 §2.3.1.
		return allowAll(), nil
	case resp.StatusCode >= 500, resp.StatusCode == 429:
		// 5xx or 429: disallow all.
		return disallowAll(), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return disallowAll(), nil
	}

	data, err := robotstxt.FromBytes(body)
	if err != nil {
		// Parse error → allow all (liberal interpretation).
		return allowAll(), nil
	}
	return data, nil
}

// allowAll returns a *robotstxt.RobotsData that allows all crawlers everywhere.
func allowAll() *robotstxt.RobotsData {
	data, _ := robotstxt.FromBytes([]byte("User-agent: *\nAllow: /\n"))
	return data
}

// disallowAll returns a *robotstxt.RobotsData that disallows all crawlers.
func disallowAll() *robotstxt.RobotsData {
	data, _ := robotstxt.FromBytes([]byte("User-agent: *\nDisallow: /\n"))
	return data
}
