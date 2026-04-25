// Package llm — priority-list router with per-provider circuit breaker.
// REQ-LLM-004: Provider priority list and fallthrough on exhaustion.
// NFR-LLM-002: Circuit breaker — opens at >=50% failure over 60s, 30s half-open.
package llm

import (
	"context"
	"sync"
	"time"
)

// BreakerState represents the circuit breaker state machine.
type BreakerState int

const (
	// BreakerClosed is the normal operating state.
	BreakerClosed BreakerState = iota
	// BreakerOpen rejects all requests.
	BreakerOpen
	// BreakerHalfOpen allows one probe request.
	BreakerHalfOpen
)

// breakerState is the internal alias for BreakerState.
type breakerState = BreakerState

// observation records a single success/failure with its timestamp.
type observation struct {
	t       time.Time
	success bool
}

// breaker is a per-provider circuit breaker with a rolling 60s window.
// @MX:WARN: [AUTO] Concurrent state machine; protected by mu
// @MX:REASON: Multiple goroutines may call Record/Allow concurrently
type breaker struct {
	mu           sync.Mutex
	state        breakerState
	observations []observation
	openedAt     time.Time

	// Configuration (exposed for testing via clock injection).
	window      time.Duration // rolling window (default 60s)
	openTimeout time.Duration // half-open delay (default 30s)
	minSamples  int           // minimum observations to open (default 10)
	threshold   float64       // failure ratio threshold (default 0.50)

	// nowFn allows clock injection in tests.
	nowFn func() time.Time
}

func newBreaker() *breaker {
	return &breaker{
		window:      60 * time.Second,
		openTimeout: 30 * time.Second,
		minSamples:  10,
		threshold:   0.50,
		nowFn:       time.Now,
	}
}

// Allow returns true if a request should be allowed through.
// For Half-Open state, only the first call returns true (probe).
func (b *breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.nowFn()

	switch b.state {
	case BreakerClosed:
		return true
	case BreakerOpen:
		if now.Sub(b.openedAt) >= b.openTimeout {
			b.state = BreakerHalfOpen
			return true
		}
		return false
	case BreakerHalfOpen:
		// Already in half-open; only the probe request is allowed.
		return false
	}
	return true
}

// Record records a call outcome and updates circuit state.
func (b *breaker) Record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.nowFn()

	// In half-open: probe result determines next state.
	if b.state == BreakerHalfOpen {
		if success {
			b.state = BreakerClosed
			b.observations = nil
		} else {
			b.state = BreakerOpen
			b.openedAt = now
		}
		return
	}

	// Append observation and prune expired ones.
	b.observations = append(b.observations, observation{t: now, success: success})
	cutoff := now.Add(-b.window)
	start := 0
	for start < len(b.observations) && b.observations[start].t.Before(cutoff) {
		start++
	}
	b.observations = b.observations[start:]

	// Check open threshold.
	if b.state == BreakerClosed && len(b.observations) >= b.minSamples {
		failures := 0
		for _, o := range b.observations {
			if !o.success {
				failures++
			}
		}
		ratio := float64(failures) / float64(len(b.observations))
		if ratio >= b.threshold {
			b.state = BreakerOpen
			b.openedAt = now
			b.observations = nil
		}
	}
}

// State returns the current breaker state (for testing / inspection).
func (b *breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// ForceOpenAt forces the breaker into Open state with the given open timestamp.
// Used only in tests to simulate time-elapsed open state.
func (b *breaker) ForceOpenAt(t time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = BreakerOpen
	b.openedAt = t
	b.observations = nil
}

// Router selects providers for a ModelClass and tracks per-provider circuit state.
// @MX:ANCHOR: [AUTO] Provider selection + circuit breaker; callers: client.go, router_test.go, tests
// @MX:REASON: fan_in >= 3; all retry/fallthrough logic flows through Route
type Router struct {
	priorities map[ModelClass][]ProviderRef
	breakers   map[string]*breaker // keyed by provider name
	mu         sync.RWMutex
}

// NewRouter creates a Router with the given provider priority map.
func NewRouter(priorities map[ModelClass][]ProviderRef) *Router {
	breakers := make(map[string]*breaker)
	for _, refs := range priorities {
		for _, ref := range refs {
			if _, ok := breakers[ref.Provider]; !ok {
				breakers[ref.Provider] = newBreaker()
			}
		}
	}
	return &Router{
		priorities: priorities,
		breakers:   breakers,
	}
}

// Route returns the ordered list of available providers for class,
// skipping providers whose circuit is Open.
// Returns ErrAllProvidersFailed if no provider is available.
func (r *Router) Route(_ context.Context, class ModelClass) ([]ProviderRef, error) {
	refs, ok := r.priorities[class]
	if !ok || len(refs) == 0 {
		return nil, ErrModelNotConfigured
	}

	var available []ProviderRef
	r.mu.RLock()
	breakers := r.breakers
	r.mu.RUnlock()

	for _, ref := range refs {
		b, exists := breakers[ref.Provider]
		if !exists || b.Allow() {
			available = append(available, ref)
		}
	}

	if len(available) == 0 {
		return nil, ErrAllProvidersFailed
	}
	return available, nil
}

// Record records a call outcome for the named provider's circuit breaker.
func (r *Router) Record(provider string, success bool) {
	r.mu.RLock()
	b := r.breakers[provider]
	r.mu.RUnlock()
	if b != nil {
		b.Record(success)
	}
}

// BreakerFor returns the breaker for a provider (for testing).
func (r *Router) BreakerFor(provider string) *breaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.breakers[provider]
}
