package auth

import (
	"github.com/prometheus/client_golang/prometheus"
)

// AuthMetrics holds Prometheus collectors for the auth subsystem.
// NFR-AUTH1-006: all label values are bounded enums — no PII or unbounded strings.
type AuthMetrics struct {
	Attempts          *prometheus.CounterVec
	Failures          *prometheus.CounterVec
	TokenRevoked      *prometheus.CounterVec
	ValidationSeconds *prometheus.HistogramVec
	JWKSRefresh       *prometheus.CounterVec
	ModeGauge         *prometheus.GaugeVec
}

// NewAuthMetrics creates and registers auth collectors on the given registry.
func NewAuthMetrics(reg prometheus.Registerer) *AuthMetrics {
	m := &AuthMetrics{
		Attempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_auth_attempts_total",
				Help: "Total JWT validation attempts, partitioned by outcome.",
			},
			[]string{"outcome"},
		),
		Failures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_auth_failures_total",
				Help: "Total JWT validation failures, partitioned by reason.",
			},
			[]string{"reason"},
		),
		TokenRevoked: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_auth_token_revoked_total",
				Help: "Total tokens explicitly revoked, partitioned by trigger.",
			},
			[]string{"trigger"},
		),
		ValidationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "usearch_auth_validation_duration_seconds",
				Help:    "JWT validation latency distribution.",
				Buckets: []float64{0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
			[]string{"outcome"},
		),
		JWKSRefresh: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_auth_jwks_refresh_total",
				Help: "JWKS refresh attempts, partitioned by outcome.",
			},
			[]string{"outcome"},
		),
		ModeGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "usearch_auth_mode",
				Help: "Auth enforcement mode (1 = active).",
			},
			[]string{"mode"},
		),
	}

	reg.MustRegister(
		m.Attempts,
		m.Failures,
		m.TokenRevoked,
		m.ValidationSeconds,
		m.JWKSRefresh,
		m.ModeGauge,
	)

	// Pre-initialize label values so metrics appear in /metrics output
	for _, outcome := range []string{"success", "anonymous_fallback"} {
		m.Attempts.WithLabelValues(outcome).Add(0)
	}
	for _, reason := range []string{
		string(ReasonExpired),
		string(ReasonInvalidSignature),
		string(ReasonInvalidAudience),
		string(ReasonInvalidIssuer),
		string(ReasonInvalidNbf),
		string(ReasonMalformed),
		string(ReasonRevoked),
		string(ReasonMissingToken),
		string(ReasonRevocationUnavailable),
	} {
		m.Failures.WithLabelValues(reason).Add(0)
	}
	m.TokenRevoked.WithLabelValues("explicit_logout").Add(0)
	m.ValidationSeconds.WithLabelValues("success").Observe(0)
	for _, outcome := range []string{
		string(JWKSRefreshScheduled),
		string(JWKSRefreshUnknownKID),
		string(JWKSRefreshParseError),
		string(JWKSRefreshNetworkError),
	} {
		m.JWKSRefresh.WithLabelValues(outcome).Add(0)
	}
	for _, mode := range []string{string(ModeStrict), string(ModePermissive), string(ModeDisabled)} {
		m.ModeGauge.WithLabelValues(mode).Set(0)
	}

	return m
}
