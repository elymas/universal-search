// Package metrics provides the Prometheus metrics registry and named collectors
// for Universal Search observability.
//
// REQ-OBS-003: Named collectors (Counter, Histogram, Gauge) for HTTP, fanout,
// adapter, and build-info.
// NFR-OBS-002: Static label allowlist; no unbounded cardinality.
package metrics

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// adapterCallBuckets is the custom bucket set for adapter call durations.
// Bimodal: fast API calls (~100ms), slow scraping (~5-30s).
var adapterCallBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// Registry bundles a custom prometheus.Registry with all named collectors.
// Using a non-default registry enables test isolation (each test gets its own).
// @MX:ANCHOR: [AUTO] Central metrics registry; callers: obs.Init, HTTPMiddleware, StartAdminServer, tests
// @MX:REASON: fan_in >= 3; registry is the single point of truth for all metric families
type Registry struct {
	// Prometheus is the underlying prometheus.Registry used for /metrics exposition.
	Prometheus *prometheus.Registry

	// HTTP metrics.
	HTTPRequests        *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Fanout concurrency metrics.
	FanoutInflight *prometheus.GaugeVec

	// Adapter reliability metrics.
	AdapterCalls        *prometheus.CounterVec
	AdapterCallDuration *prometheus.HistogramVec

	// Build metadata.
	BuildInfo *prometheus.GaugeVec

	// LLM metrics (SPEC-LLM-001). Per-Registry instances avoid the global-
	// variable race that surfaces under t.Parallel.
	LLMCalls   *prometheus.CounterVec
	LLMCost    *prometheus.CounterVec
	LLMLatency *prometheus.HistogramVec

	// Intent Router metrics (SPEC-IR-001). Reuse the existing `outcome`
	// label name; no new label is added to the cardinality allowlist.
	RouterClassifications        *prometheus.CounterVec
	RouterClassificationDuration *prometheus.HistogramVec

	// Synthesis metrics (SPEC-SYN-001). Reuse the existing `outcome` label;
	// SynthesisCost has no labels (no new cardinality introduced).
	SynthesisCalls   *prometheus.CounterVec
	SynthesisLatency *prometheus.HistogramVec
	SynthesisCost    prometheus.Counter

	// Embedder metrics (SPEC-IDX-002). New label `mode` added to allowlist.
	EmbedderCalls     *prometheus.CounterVec
	EmbedderLatency   *prometheus.HistogramVec
	EmbedderCacheHits prometheus.Counter

	// labelNames tracks all registered label names for cardinality validation.
	labelNames []string
}

// NewRegistry creates and registers all named metric collectors on a fresh
// prometheus.Registry. Use one Registry per process (or per test for isolation).
func NewRegistry() *Registry {
	pr := prometheus.NewRegistry()

	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_http_requests_total",
			Help: "Total HTTP requests received, partitioned by method, route, and status class.",
		},
		[]string{"method", "route", "status_class"},
	)

	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_http_request_duration_seconds",
			Help:    "HTTP request latency distribution.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	fanoutInflight := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_fanout_goroutines_inflight",
			Help: "Number of fanout goroutines currently active.",
		},
		[]string{"adapter_class"},
	)

	adapterCalls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_adapter_calls_total",
			Help: "Total adapter calls, partitioned by adapter and outcome.",
		},
		[]string{"adapter", "outcome"},
	)

	adapterDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_adapter_call_duration_seconds",
			Help:    "Adapter call latency distribution.",
			Buckets: adapterCallBuckets,
		},
		[]string{"adapter"},
	)

	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_build_info",
			Help: "Build metadata. Always set to 1.",
		},
		[]string{"version", "commit", "go_version"},
	)

	// Register all collectors; panic on error (programming mistake, not runtime error).
	pr.MustRegister(
		httpRequests,
		httpDuration,
		fanoutInflight,
		adapterCalls,
		adapterDuration,
		buildInfo,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// Pre-initialise each Vec with placeholder label values so that the metric
	// family appears in /metrics output even before any real observations are
	// recorded. This satisfies REQ-OBS-004 TestMetricsIncludesAllFamilies.
	httpRequests.WithLabelValues("GET", "/", "2xx").Add(0)
	httpDuration.WithLabelValues("GET", "/").Observe(0)
	fanoutInflight.WithLabelValues("web").Add(0)
	adapterCalls.WithLabelValues("", "success").Add(0)
	adapterDuration.WithLabelValues("").Observe(0)
	buildInfo.WithLabelValues("", "", "").Set(0)

	// Register LLM metrics (SPEC-LLM-001).
	llm := registerLLM(pr)

	// Register Intent Router metrics (SPEC-IR-001).
	router := registerRouter(pr)

	// Register Synthesis metrics (SPEC-SYN-001).
	synth := registerSynthesis(pr)

	// Register Embedder metrics (SPEC-IDX-002).
	embedder := registerEmbedder(pr)

	return &Registry{
		Prometheus:                   pr,
		HTTPRequests:                 httpRequests,
		HTTPRequestDuration:          httpDuration,
		FanoutInflight:               fanoutInflight,
		AdapterCalls:                 adapterCalls,
		AdapterCallDuration:          adapterDuration,
		BuildInfo:                    buildInfo,
		LLMCalls:                     llm.calls,
		LLMCost:                      llm.cost,
		LLMLatency:                   llm.latency,
		RouterClassifications:        router.classifications,
		RouterClassificationDuration: router.duration,
		SynthesisCalls:               synth.calls,
		SynthesisLatency:             synth.latency,
		SynthesisCost:                synth.cost,
		EmbedderCalls:                embedder.calls,
		EmbedderLatency:              embedder.latency,
		EmbedderCacheHits:            embedder.cacheHits,
		labelNames: []string{
			"method", "route", "status_class",
			"adapter_class",
			"adapter", "outcome",
			"version", "commit", "go_version",
			// LLM labels (SPEC-LLM-001 REQ-LLM-007)
			"provider", "model",
			// Embedder labels (SPEC-IDX-002) — `mode` is the new label name
			"mode",
		},
	}
}

// AllLabelNames returns the complete list of label names used across all
// registered Vec collectors. Used by NFR-OBS-002 cardinality guard tests.
func (r *Registry) AllLabelNames() []string {
	return r.labelNames
}

// Handler returns an http.Handler that serves Prometheus metrics from this
// registry in text exposition format.
func Handler(r *Registry) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r.Prometheus, promhttp.HandlerOpts{
		Registry: r.Prometheus,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// StartAdminServer starts an HTTP admin server on the given address (e.g.,
// "127.0.0.1:9090") and returns:
//   - addr: the actual listening address (useful when address was "127.0.0.1:0")
//   - shutdown: a function to gracefully stop the server
//   - err: any error during listener creation
//
// The server is stopped when ctx is cancelled OR when shutdown() is called.
// @MX:ANCHOR: [AUTO] Admin server lifecycle; callers: obs.Init, cmd/usearch, tests
// @MX:REASON: fan_in >= 3; localhost binding is a security requirement (NFR)
// @MX:WARN: [AUTO] Goroutine launched; context-cancellable via serverCtx
// @MX:REASON: goroutine is bounded by ctx lifetime; errgroup pattern used implicitly via server.Shutdown
func StartAdminServer(ctx context.Context, addr string, reg *Registry) (string, func(context.Context) error, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}

	srv := &http.Server{
		Handler:           Handler(reg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = srv.Serve(ln)
	}()

	// Stop server when context is cancelled.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	return ln.Addr().String(), srv.Shutdown, nil
}

// statusClass maps an HTTP status code to its class string (2xx, 3xx, 4xx, 5xx).
func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// responseWriter wraps http.ResponseWriter to capture the written status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware wraps an http.Handler with request counter + duration histogram
// recording. route is the route template (e.g., "/v1/query"), not the raw path.
func HTTPMiddleware(reg *Registry, route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		elapsed := time.Since(start).Seconds()
		sc := statusClass(rw.status)

		reg.HTTPRequests.WithLabelValues(r.Method, route, sc).Inc()
		reg.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(elapsed)
	})
}
