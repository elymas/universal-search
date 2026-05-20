// Package access implements the 5-phase content-fetch cascade.
//
// The cascade is: Phase 1 (local index) → Phase 2 (HEAD + robots.txt) →
// Phase 3 (standard HTTP GET) → Phase 4 (TLS-aware GET) → Phase 5 (Playwright).
// Each phase escalates to the next only on specific error conditions.
//
// Entry point: (*Fetcher).Fetch(ctx, url, opts) — the ONLY public fetch method.
//
// SPEC-CACHE-001 | REQ-CACHE-001 | D1-D11
//
// @MX:NOTE: [AUTO] Package entry point for the 5-phase access fallback (SPEC-CACHE-001).
// Sequential cascade — distinct from FAN-001 (parallel fan-out) and IDX-001
// (3-way parallel). One Fetcher per process; call Close/Shutdown on exit.
// @MX:SPEC: SPEC-CACHE-001
package access

import (
	"context"
	"log/slog"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// Fetcher orchestrates the 5-phase content-fetch cascade.
//
// Immutable post-construction: all fields are set by New and never modified.
// Concurrent callers may call Fetch simultaneously; internal shared state
// (robotsCache, browserPool) uses goroutine-safe primitives.
type Fetcher struct {
	// pw is the Playwright runtime; nil when PlaywrightEnabled == false.
	pw *playwright.Playwright
	// browserPool is a buffered channel used as a bounded browser pool.
	// Capacity = Options.MaxBrowsers.
	browserPool chan playwright.Browser
	// robotsCache is the per-host robots.txt cache.
	robotsCache *robotsCache
	// opts holds the fetcher-level configuration (immutable post-New).
	opts Options
	// obs is the observability adapter; always non-nil (noopObs when Obs=nil).
	obs ObsAdapter
	// writeThroughWG tracks in-flight cache write-through goroutines.
	// Shutdown waits on this group before returning.
	writeThroughWG sync.WaitGroup
	// shutdownCh is closed by Shutdown to signal rejection of new Fetch calls.
	shutdownCh chan struct{}
	// shutdownOnce ensures Shutdown idempotency.
	shutdownOnce sync.Once
}

// New constructs a Fetcher from the provided Options.
//
// Zero-valued Option fields are replaced by documented defaults (REQ-CACHE-001 §6.6).
// When PlaywrightEnabled is true and AutoInstallPlaywright is false, playwright
// browsers must be pre-installed; otherwise New returns ErrPlaywrightUnavailable.
//
// Callers MUST call Close() or Shutdown() when done to release resources.
func New(opts Options) (*Fetcher, error) {
	opts.applyDefaults()

	obs := resolveObs(opts.Obs)

	f := &Fetcher{
		robotsCache: newRobotsCache(opts.RobotsTTL),
		opts:        opts,
		obs:         obs,
		shutdownCh:  make(chan struct{}),
	}

	if opts.PlaywrightEnabled {
		pw, pool, err := initPlaywright(opts)
		if err != nil {
			return nil, err
		}
		f.pw = pw
		f.browserPool = pool
	}

	return f, nil
}

// initPlaywright sets up the Playwright runtime and pre-populates the browser pool.
func initPlaywright(opts Options) (*playwright.Playwright, chan playwright.Browser, error) {
	if opts.AutoInstallPlaywright {
		if err := playwright.Install(); err != nil {
			return nil, nil, wrapPlaywrightErr(err)
		}
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, nil, wrapPlaywrightErr(err)
	}

	pool := make(chan playwright.Browser, opts.MaxBrowsers)
	// Pre-launch MaxBrowsers browsers into the pool.
	for range opts.MaxBrowsers {
		browser, err := launchBrowserWith(pw, opts)
		if err != nil {
			// On partial failure: close what we launched and stop Playwright.
			close(pool)
			for b := range pool {
				_ = b.Close()
			}
			_ = pw.Stop()
			return nil, nil, wrapPlaywrightErr(err)
		}
		pool <- browser
	}

	return pw, pool, nil
}

// wrapPlaywrightErr wraps a Playwright initialization error as ErrPlaywrightUnavailable.
func wrapPlaywrightErr(err error) error {
	return &FetchError{
		Category: CategoryUnavailable,
		Reason:   "playwright init failed",
		Cause:    ErrPlaywrightUnavailable,
	}
}

// launchBrowserWith launches a single browser of the configured type.
func launchBrowserWith(pw *playwright.Playwright, opts Options) (playwright.Browser, error) {
	return launchBrowserType(pw, opts.Playwright.BrowserType)
}

// launchBrowserType launches the named browser type (chromium/firefox/webkit).
func launchBrowserType(pw *playwright.Playwright, browserType string) (playwright.Browser, error) {
	headless := true
	switch browserType {
	case "firefox":
		return pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{Headless: &headless})
	case "webkit":
		return pw.WebKit.Launch(playwright.BrowserTypeLaunchOptions{Headless: &headless})
	default: // "chromium" or empty
		return pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: &headless})
	}
}

// launchBrowser is a convenience method on Fetcher for refillBrowserPool.
func (f *Fetcher) launchBrowser() (playwright.Browser, error) {
	if f.pw == nil {
		return nil, ErrPlaywrightUnavailable
	}
	return launchBrowserWith(f.pw, f.opts)
}

// Close performs an orderly shutdown: stops Playwright, closes all pooled
// browsers, and drains in-flight write-through goroutines.
// Returns the first non-nil error encountered.
func (f *Fetcher) Close() error {
	var firstErr error
	f.shutdownOnce.Do(func() {
		close(f.shutdownCh)
	})

	// Drain write-through goroutines (best-effort, no deadline).
	f.writeThroughWG.Wait()

	// Close pooled browsers.
	if f.browserPool != nil {
		// Drain without blocking (pool is buffered).
	drainLoop:
		for {
			select {
			case b := <-f.browserPool:
				if err := b.Close(); err != nil && firstErr == nil {
					firstErr = err
				}
			default:
				break drainLoop
			}
		}
	}

	// Stop Playwright runtime.
	if f.pw != nil {
		if err := f.pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Shutdown stops accepting new Fetch calls, drains in-flight write-through
// goroutines within ctx's deadline, then calls Close.
//
// REQ-CACHE-015: enables clean Playwright child-process termination.
// The host process calls Shutdown from its signal handler; Fetch returns
// ErrShuttingDown for any subsequent invocations.
func (f *Fetcher) Shutdown(ctx context.Context) error {
	// Signal rejection of new Fetch calls.
	f.shutdownOnce.Do(func() {
		close(f.shutdownCh)
	})

	// Drain in-flight write-through goroutines with ctx deadline.
	done := make(chan struct{})
	go func() {
		f.writeThroughWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		// Timeout: proceed with remaining cleanup anyway.
	}

	return f.closeResources()
}

// closeResources closes browsers and stops Playwright without touching shutdownCh.
func (f *Fetcher) closeResources() error {
	var firstErr error

	if f.browserPool != nil {
	drainLoop2:
		for {
			select {
			case b := <-f.browserPool:
				if err := b.Close(); err != nil && firstErr == nil {
					firstErr = err
				}
			default:
				break drainLoop2
			}
		}
	}

	if f.pw != nil {
		if err := f.pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// logger returns the slog.Logger from the obs adapter, or nil.
func (f *Fetcher) logger() *slog.Logger {
	if f.obs == nil {
		return nil
	}
	return f.obs.SlogLogger()
}

// resolveObs returns opts.Obs if non-nil; otherwise returns noopObs{}.
func resolveObs(o ObsAdapter) ObsAdapter {
	if o == nil {
		return noopObs{}
	}
	return o
}

