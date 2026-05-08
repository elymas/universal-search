// Package access — Phase 5: Playwright headless browser.
//
// REQ-CACHE-006: Phase 5 acquires a *Browser from the pool, navigates to the
// URL, extracts rendered HTML, and returns the browser to the pool.
//
// Build constraint: Phase 5 is always compiled (no build tag on the source).
// Integration tests are gated with //go:build integration in phase5_test.go.
package access

import (
	"context"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

// phase5Browser runs a Playwright headless browser to render the target URL.
//
// @MX:WARN: [AUTO] Browser pool acquire/release; Playwright child-process management.
// @MX:REASON: Removing defer browser.Close()/pool-return invalidates NFR-CACHE-005
// (zero goroutine leaks) and NFR-CACHE-006 (memory ceiling). The pool channel is
// the goroutine-safety boundary: one browser per concurrent Fetch in flight.
// @MX:SPEC: SPEC-CACHE-001
func (f *Fetcher) phase5Browser(
	ctx context.Context,
	rawURL string,
	fopts FetchOptions,
) (*FetchedContent, error) {
	if !f.opts.PlaywrightEnabled || f.pw == nil {
		return nil, &FetchError{
			Category: CategoryUnavailable,
			Reason:   "playwright not enabled",
		}
	}

	// Acquire a browser from the pool with per-phase budget.
	var browser playwright.Browser
	select {
	case browser = <-f.browserPool:
		// Got a pooled browser.
	case <-ctx.Done():
		return nil, &FetchError{
			Category: CategoryTimeout,
			Reason:   "browser pool exhausted or context cancelled",
			Cause:    ctx.Err(),
		}
	}

	// Return or discard the browser on exit.
	browserOK := false
	defer func() {
		if browserOK {
			// Return healthy browser to the pool (non-blocking — pool is buffered).
			select {
			case f.browserPool <- browser:
			default:
				// Pool full (race): close this browser.
				_ = browser.Close()
			}
		} else {
			// Browser had an error — close and don't return to pool.
			_ = browser.Close()
			// Refill the pool with a fresh browser if Playwright is still running.
			f.refillBrowserPool()
		}
	}()

	// Open a new page.
	page, err := browser.NewPage()
	if err != nil {
		return nil, &FetchError{
			Category: CategoryUnavailable,
			Reason:   fmt.Sprintf("browser.NewPage failed: %v", err),
			Cause:    err,
		}
	}
	defer func() { _ = page.Close() }()

	// Navigate with the per-phase timeout (already applied to ctx by cascade).
	timeout := float64(25 * time.Second / time.Millisecond) // 25s in ms
	_, err = page.Goto(rawURL, playwright.PageGotoOptions{
		Timeout: playwright.Float(timeout),
	})
	if err != nil {
		return nil, &FetchError{
			Category: CategoryUnavailable,
			Reason:   fmt.Sprintf("page.Goto failed: %v", err),
			Cause:    err,
		}
	}

	html, err := page.Content()
	if err != nil {
		return nil, &FetchError{
			Category: CategoryUnavailable,
			Reason:   fmt.Sprintf("page.Content failed: %v", err),
			Cause:    err,
		}
	}

	browserOK = true
	return &FetchedContent{
		URL:         rawURL,
		Body:        []byte(html),
		ContentType: "text/html",
		StatusCode:  200,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// refillBrowserPool launches a new browser and puts it into the pool
// to replace a discarded one. Called after a browser-level error.
func (f *Fetcher) refillBrowserPool() {
	if f.pw == nil {
		return
	}
	browser, err := f.launchBrowser()
	if err != nil {
		return // Best-effort refill; pool will be short until next launch.
	}
	select {
	case f.browserPool <- browser:
	default:
		// Pool is already full (concurrent refill race): discard.
		_ = browser.Close()
	}
}
