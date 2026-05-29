// Package ratelimit provides per-tenant token-bucket rate limiting for
// SPEC-SEC-001 (REQ-SEC-014). V1 is alert-only by default.
package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Config controls limiter behaviour. Defaults mirror security.yaml ratelimit.*.
type Config struct {
	// PerTenantPerMinute is the sustained refill rate per tenant (default 60).
	PerTenantPerMinute int
	// Burst is the maximum number of tokens that can accumulate (default 60).
	Burst int
}

const (
	defaultPerMinute = 60
	defaultBurst     = 60
)

func (c Config) withDefaults() Config {
	if c.PerTenantPerMinute <= 0 {
		c.PerTenantPerMinute = defaultPerMinute
	}
	if c.Burst <= 0 {
		c.Burst = defaultBurst
	}
	return c
}

// Limiter holds one token bucket per tenant. It is safe for concurrent use.
type Limiter struct {
	cfg     Config
	perSec  rate.Limit
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
}

// New constructs a Limiter from cfg, applying defaults for unset fields.
func New(cfg Config) *Limiter {
	cfg = cfg.withDefaults()
	return &Limiter{
		cfg:     cfg,
		perSec:  rate.Limit(float64(cfg.PerTenantPerMinute) / 60.0),
		buckets: make(map[string]*rate.Limiter),
	}
}

// Allow reports whether a request from tenant may proceed, consuming one token
// from that tenant's bucket. It is the per-request hot path.
//
// @MX:WARN: [AUTO] Per-request hot path executed on every rate-limited request;
// holds a mutex while looking up/creating the per-tenant bucket.
// @MX:REASON: lock contention here directly adds latency to every request — the
// critical section must stay O(1) (map lookup + lazy bucket creation only).
// @MX:SPEC: SPEC-SEC-001 (REQ-SEC-014)
func (l *Limiter) Allow(tenant string) bool {
	return l.bucket(tenant).Allow()
}

// bucket returns the per-tenant rate.Limiter, lazily creating it on first use.
func (l *Limiter) bucket(tenant string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[tenant]
	if !ok {
		b = rate.NewLimiter(l.perSec, l.cfg.Burst)
		l.buckets[tenant] = b
	}
	return b
}
