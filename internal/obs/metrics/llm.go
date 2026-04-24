// Package metrics — LLM-specific metric collectors.
// Owned by SPEC-LLM-001; lives under internal/obs/metrics/ to preserve the
// import-boundary test in SPEC-OBS-001 (TestNoDirectPrometheusImportOutsideObs).
//
// REQ-LLM-003: usearch_llm_calls_total, usearch_llm_cost_usd_total, usearch_llm_latency_seconds
// REQ-LLM-007: Bounded label cardinality — provider ∈ {anthropic,openai,ollama},
//
//	model ∈ deploy/litellm/config.yaml aliases (≤15), outcome ∈ {success,failure,timeout}
package metrics

import "github.com/prometheus/client_golang/prometheus"

// llmCallBuckets covers the typical LLM latency range: fast local (Ollama) to
// large-model long requests (Opus), with fine resolution under 5 s.
var llmCallBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// LLM metric collectors — exported as package-level vars so internal/llm
// can call them without importing prometheus directly (SPEC-OBS-001 boundary).
//
// @MX:ANCHOR: [AUTO] LLM metric entry points; callers: internal/llm/cost.go, internal/llm/client.go, tests
// @MX:REASON: fan_in >= 3; these vars are the only Prometheus interface for LLM telemetry
var (
	// LLMCalls is total LLM calls, partitioned by provider, model, and outcome.
	// outcome ∈ {success, failure, timeout}
	LLMCalls *prometheus.CounterVec

	// LLMCost is the cumulative USD cost per provider/model pair.
	LLMCost *prometheus.CounterVec

	// LLMLatency is the LLM call latency distribution per provider/model pair.
	LLMLatency *prometheus.HistogramVec
)

// registerLLM creates and registers the three LLM metric collectors on r.
// Called from NewRegistry alongside the base collectors.
func registerLLM(r *prometheus.Registry) {
	LLMCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_llm_calls_total",
			Help: "Total LLM calls, partitioned by provider, model, and outcome.",
		},
		[]string{"provider", "model", "outcome"},
	)
	LLMCost = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_llm_cost_usd_total",
			Help: "Cumulative LLM cost in USD, partitioned by provider and model.",
		},
		[]string{"provider", "model"},
	)
	LLMLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_llm_latency_seconds",
			Help:    "LLM call latency distribution.",
			Buckets: llmCallBuckets,
		},
		[]string{"provider", "model"},
	)

	r.MustRegister(LLMCalls, LLMCost, LLMLatency)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any real LLM calls are made.
	LLMCalls.WithLabelValues("anthropic", "claude-sonnet-4-6", "success").Add(0)
	LLMCost.WithLabelValues("anthropic", "claude-sonnet-4-6").Add(0)
	LLMLatency.WithLabelValues("anthropic", "claude-sonnet-4-6").Observe(0)
}
