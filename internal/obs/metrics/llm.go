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

// llmCollectors bundles the three LLM metric vectors created per Registry.
// Stored as Registry.LLMCalls / LLMCost / LLMLatency fields. Per-Registry
// instances avoid the global-variable race that t.Parallel tests revealed
// when multiple Registry instances co-existed.
type llmCollectors struct {
	calls   *prometheus.CounterVec
	cost    *prometheus.CounterVec
	latency *prometheus.HistogramVec
}

// registerLLM creates the three LLM metric collectors, registers them on r,
// and returns them for storage on the owning Registry.
func registerLLM(r *prometheus.Registry) llmCollectors {
	calls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_llm_calls_total",
			Help: "Total LLM calls, partitioned by provider, model, and outcome.",
		},
		[]string{"provider", "model", "outcome"},
	)
	cost := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_llm_cost_usd_total",
			Help: "Cumulative LLM cost in USD, partitioned by provider and model.",
		},
		[]string{"provider", "model"},
	)
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_llm_latency_seconds",
			Help:    "LLM call latency distribution.",
			Buckets: llmCallBuckets,
		},
		[]string{"provider", "model"},
	)

	r.MustRegister(calls, cost, latency)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any real LLM calls are made.
	calls.WithLabelValues("anthropic", "claude-sonnet-4-6", "success").Add(0)
	cost.WithLabelValues("anthropic", "claude-sonnet-4-6").Add(0)
	latency.WithLabelValues("anthropic", "claude-sonnet-4-6").Observe(0)

	return llmCollectors{calls: calls, cost: cost, latency: latency}
}
