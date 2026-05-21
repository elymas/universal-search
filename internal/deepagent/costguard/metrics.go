package costguard

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus collectors for the costguard package.
// SPEC-OBS-001 naming convention: usearch_deep_*.
type Metrics struct {
	// CacheHits tracks cache hit events by tier.
	CacheHits *prometheus.CounterVec
	// CacheAttempts tracks total cache lookup attempts by tier.
	CacheAttempts *prometheus.CounterVec
	// Calls tracks /deep call outcomes by tenant and status.
	Calls *prometheus.CounterVec
	// Cost tracks cumulative USD cost by tenant and model.
	Cost *prometheus.CounterVec
	// HaikuScreenScore tracks the distribution of Haiku pre-screen scores.
	HaikuScreenScore prometheus.Histogram
	// HaikuScreenBreakerState tracks the circuit breaker state.
	HaikuScreenBreakerState *prometheus.GaugeVec
	// CapCheckDuration tracks the latency of cap-check evaluation.
	CapCheckDuration prometheus.Histogram
}

// Tier constants for cache metrics.
const (
	TierHaikuScreen = "haiku_screen"
	TierResearcher  = "researcher"
	TierReviewer    = "reviewer"
	TierWriter      = "writer"
	TierVerifier    = "verifier"
)

// Status constants for call metrics.
const (
	StatusAllowed          = "allowed"
	StatusCapped           = "capped"
	StatusDegraded         = "degraded"
	StatusRejectedByScreen = "rejected_by_screen"
	StatusSuggestedBasic   = "suggested_basic"
	StatusError            = "error"
)

// RegisterMetrics creates and registers all costguard Prometheus collectors
// on the given registry. Returns a Metrics struct for use by the package.
func RegisterMetrics(reg *prometheus.Registry) *Metrics {
	cacheHits := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_cache_hits_total",
			Help: "Total cache hit events, partitioned by tier.",
		},
		[]string{"tier"},
	)
	cacheAttempts := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_cache_attempts_total",
			Help: "Total cache lookup attempts, partitioned by tier.",
		},
		[]string{"tier"},
	)
	calls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_calls_total",
			Help: "Total /deep call outcomes, partitioned by tenant and status.",
		},
		[]string{"tenant", "status"},
	)
	cost := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_cost_usd_total",
			Help: "Cumulative USD cost, partitioned by tenant and model.",
		},
		[]string{"tenant", "model"},
	)
	haikuScore := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_haiku_screen_score",
			Help:    "Distribution of Haiku pre-screen scores.",
			Buckets: []float64{0, 2, 4, 6, 8, 10},
		},
	)
	breakerState := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_deep_haiku_screen_breaker_state",
			Help: "Circuit breaker state: 1=closed, 0.5=half_open, 0=open.",
		},
		[]string{"state"},
	)
	capCheckDur := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_cap_check_duration_seconds",
			Help:    "Cap-check evaluation latency.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1},
		},
	)

	reg.MustRegister(
		cacheHits,
		cacheAttempts,
		calls,
		cost,
		haikuScore,
		breakerState,
		capCheckDur,
	)

	// Pre-initialize label values per SPEC-OBS-001 convention.
	tiers := []string{TierHaikuScreen, TierResearcher, TierReviewer, TierWriter, TierVerifier}
	for _, tier := range tiers {
		cacheHits.WithLabelValues(tier).Add(0)
		cacheAttempts.WithLabelValues(tier).Add(0)
	}

	breakerState.WithLabelValues("closed").Set(0)
	breakerState.WithLabelValues("half_open").Set(0)
	breakerState.WithLabelValues("open").Set(0)

	return &Metrics{
		CacheHits:              cacheHits,
		CacheAttempts:          cacheAttempts,
		Calls:                  calls,
		Cost:                   cost,
		HaikuScreenScore:       haikuScore,
		HaikuScreenBreakerState: breakerState,
		CapCheckDuration:       capCheckDur,
	}
}
