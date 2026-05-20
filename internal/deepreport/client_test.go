package deepreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/goleak"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestMetrics creates a fresh Prometheus registry and registers the deepreport
// collectors so the client can record metrics during tests.
func newTestMetrics() (*prometheus.Registry, *prometheus.CounterVec, prometheus.Histogram) {
	pr := prometheus.NewRegistry()
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_deep_outcomes_total"},
		[]string{"outcome"},
	)
	latency := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "test_deep_latency_seconds",
			Buckets: []float64{5, 15, 30, 60, 120, 180, 240, 300},
		},
	)
	pr.MustRegister(outcomes, latency)
	return pr, outcomes, latency
}

// stubServer creates an httptest.Server that responds with the given status and body.
func stubServer(status int, body any) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/generate_report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(status)
		if body != nil {
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(body)
			_, _ = w.Write(b)
		}
	})
	return httptest.NewServer(mux)
}

// validReport returns a Report fixture for happy-path tests.
func validReport() *Report {
	return &Report{
		RequestID: "test-req-001",
		Title:     "Test Report",
		Sections: []Section{
			{
				SectionIndex: 0,
				Heading:      "Introduction",
				Level:        1,
				Text:         "This is an intro.",
				Sentences: []Sentence{
					{SentenceIndex: 0, Text: "This is an intro.", Markers: []int{1}},
				},
			},
		},
		Citations: []Citation{
			{Marker: 1, DocID: "doc-1", URL: "https://example.com", Title: "Source"},
		},
		Model:            "claude-sonnet-4-6",
		Provider:         "anthropic",
		CostUSD:          0.42,
		PromptTokens:     1000,
		CompletionTokens: 500,
		LatencyMS:        12000,
		SchemaVersion:    1,
	}
}

// ---------------------------------------------------------------------------
// RED: Error sentinel tests
// ---------------------------------------------------------------------------

func TestErrorSentinelsWorkWithErrorsIs(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	cases := []struct {
		name  string
		err   error
		sent  error
		match bool
	}{
		{"ErrInvalidRequest wraps", fmt.Errorf("wrap: %w", ErrInvalidRequest), ErrInvalidRequest, true},
		{"ErrSidecarUnreachable wraps", fmt.Errorf("wrap: %w", ErrSidecarUnreachable), ErrSidecarUnreachable, true},
		{"ErrTimeout wraps", fmt.Errorf("wrap: %w", ErrTimeout), ErrTimeout, true},
		{"ErrBudgetExceeded wraps", fmt.Errorf("wrap: %w", ErrBudgetExceeded), ErrBudgetExceeded, true},
		{"ErrDeadlineExceeded wraps", fmt.Errorf("wrap: %w", ErrDeadlineExceeded), ErrDeadlineExceeded, true},
		{"cross sentinel does not match", ErrInvalidRequest, ErrSidecarUnreachable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := errors.Is(tc.err, tc.sent)
			if got != tc.match {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tc.err, tc.sent, got, tc.match)
			}
		})
	}
}

func TestNewClientWithoutMetrics(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusOK, validReport())
	defer srv.Close()

	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClient(cfg)

	report, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.RequestID != "test-req-001" {
		t.Errorf("RequestID = %q, want test-req-001", report.RequestID)
	}
}

// ---------------------------------------------------------------------------
// RED: Config tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	cfg := DefaultConfig()
	if cfg.SidecarURL != "http://localhost:8001" {
		t.Errorf("SidecarURL = %q, want http://localhost:8001", cfg.SidecarURL)
	}
	if cfg.Timeout != 360*time.Second {
		t.Errorf("Timeout = %v, want 360s", cfg.Timeout)
	}
}

func TestNewConfigFromEnv_Defaults(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SidecarURL != "http://localhost:8001" {
		t.Errorf("SidecarURL = %q, want http://localhost:8001", cfg.SidecarURL)
	}
	if cfg.Timeout != 360*time.Second {
		t.Errorf("Timeout = %v, want 360s", cfg.Timeout)
	}
}

func TestNewConfigFromEnv_CustomURL(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	t.Setenv("STORM_SIDECAR_URL", "http://storm:9999")
	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SidecarURL != "http://storm:9999" {
		t.Errorf("SidecarURL = %q, want http://storm:9999", cfg.SidecarURL)
	}
}

func TestNewConfigFromEnv_CustomTimeout(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	t.Setenv("STORM_SIDECAR_TIMEOUT_SECONDS", "120")
	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
}

func TestNewConfigFromEnv_InvalidTimeout(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	t.Setenv("STORM_SIDECAR_TIMEOUT_SECONDS", "not-a-number")
	_, err := NewConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid timeout, got nil")
	}
}

// ---------------------------------------------------------------------------
// RED: Client.GenerateReport — happy path
// ---------------------------------------------------------------------------

func TestGenerateReport_Success(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusOK, validReport())
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	req := &Request{
		RequestID: "test-req-001",
		Query:     "quantum computing",
		Lang:      "en",
		Docs: []NormalizedDocPayload{
			{ID: "doc-1", Title: "Quantum 101", Body: "Body text"},
		},
	}

	report, err := client.GenerateReport(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.RequestID != "test-req-001" {
		t.Errorf("RequestID = %q, want test-req-001", report.RequestID)
	}
	if report.Title != "Test Report" {
		t.Errorf("Title = %q, want Test Report", report.Title)
	}
	if len(report.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(report.Sections))
	}
	if report.Sections[0].Heading != "Introduction" {
		t.Errorf("Heading = %q, want Introduction", report.Sections[0].Heading)
	}
	if len(report.Citations) != 1 {
		t.Fatalf("len(Citations) = %d, want 1", len(report.Citations))
	}
	if report.CostUSD != 0.42 {
		t.Errorf("CostUSD = %v, want 0.42", report.CostUSD)
	}
	if report.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", report.SchemaVersion)
	}
}

// ---------------------------------------------------------------------------
// RED: Client.GenerateReport — HTTP status error mapping
// ---------------------------------------------------------------------------

func TestGenerateReport_422_InvalidRequest(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusUnprocessableEntity, nil)
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

func TestGenerateReport_504_DeadlineExceeded(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusGatewayTimeout, nil)
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("err = %v, want ErrDeadlineExceeded", err)
	}
}

func TestGenerateReport_402_BudgetExceeded(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusPaymentRequired, nil)
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("err = %v, want ErrBudgetExceeded", err)
	}
}

func TestGenerateReport_502_SidecarUnreachable(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusBadGateway, nil)
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrSidecarUnreachable) {
		t.Errorf("err = %v, want ErrSidecarUnreachable", err)
	}
}

func TestGenerateReport_ConnectionError_SidecarUnreachable(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	// Use a server that's already closed to trigger a connection error.
	srv := httptest.NewServer(http.NewServeMux())
	srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 5 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrSidecarUnreachable) {
		t.Errorf("err = %v, want ErrSidecarUnreachable", err)
	}
}

// ---------------------------------------------------------------------------
// RED: Client.GenerateReport — context cancellation
// ---------------------------------------------------------------------------

func TestGenerateReport_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// Create a context that is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: "http://127.0.0.1:0", Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(ctx, &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want ErrTimeout", err)
	}
}

// ---------------------------------------------------------------------------
// RED: Client.GenerateReport — retry on 503
// ---------------------------------------------------------------------------

func TestGenerateReport_Retry503_ThenSuccess(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			// First two attempts: 503
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "temporarily unavailable")
			return
		}
		// Third attempt: success
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(validReport())
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	report, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.RequestID != "test-req-001" {
		t.Errorf("RequestID = %q, want test-req-001", report.RequestID)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestGenerateReport_Retry503_Exhausted(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusServiceUnavailable, nil)
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if !errors.Is(err, ErrSidecarUnreachable) {
		t.Errorf("err = %v, want ErrSidecarUnreachable", err)
	}
}

// ---------------------------------------------------------------------------
// RED: Metric emission
// ---------------------------------------------------------------------------

func TestGenerateReport_MetricsEmitted(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusOK, validReport())
	defer srv.Close()

	pr, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the outcome counter was incremented.
	mfs, err := pr.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "test_deep_outcomes_total" {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "outcome" && lp.GetValue() == "success" && m.GetCounter().GetValue() > 0 {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("outcome counter not incremented for success")
	}

	// Verify latency histogram has observations.
	foundHist := false
	for _, mf := range mfs {
		if mf.GetName() == "test_deep_latency_seconds" {
			for _, m := range mf.GetMetric() {
				if m.GetHistogram() != nil && m.GetHistogram().GetSampleCount() > 0 {
					foundHist = true
				}
			}
		}
	}
	if !foundHist {
		t.Error("latency histogram has no observations")
	}
}

func TestGenerateReport_MetricsEmittedOnError(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	srv := stubServer(http.StatusUnprocessableEntity, nil)
	defer srv.Close()

	pr, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	_, _ = client.GenerateReport(context.Background(), &Request{
		RequestID: "r1", Query: "q",
	})

	mfs, _ := pr.Gather()
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "test_deep_outcomes_total" {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "outcome" && lp.GetValue() == "error_invalid" && m.GetCounter().GetValue() > 0 {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("outcome counter not incremented for error_invalid")
	}
}

// ---------------------------------------------------------------------------
// RED: Request marshalling edge cases
// ---------------------------------------------------------------------------

func TestGenerateReport_OptionalOverrides(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(validReport())
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	_, outcomes, latency := newTestMetrics()
	cfg := Config{SidecarURL: srv.URL, Timeout: 30 * time.Second}
	client := NewClientWithMetrics(cfg, outcomes, latency)

	maxTokens := float64(4000)
	maxCost := float64(1.50)
	maxPersp := float64(3)
	maxTurns := float64(2)
	maxLat := float64(180000)

	_, err := client.GenerateReport(context.Background(), &Request{
		RequestID:       "r1",
		Query:           "test query",
		MaxTokens:       &maxTokens,
		MaxCostUSD:      &maxCost,
		MaxPerspectives: &maxPersp,
		MaxConvTurns:    &maxTurns,
		MaxLatencyMS:    &maxLat,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the optional fields were serialized.
	var parsed map[string]any
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if v := parsed["max_tokens"]; v == nil {
		t.Error("max_tokens should be present, got nil")
	}
	if v := parsed["max_cost_usd"]; v == nil {
		t.Error("max_cost_usd should be present, got nil")
	}
	if v := parsed["max_perspectives"]; v == nil {
		t.Error("max_perspectives should be present, got nil")
	}
	if v := parsed["max_conv_turns"]; v == nil {
		t.Error("max_conv_turns should be present, got nil")
	}
	if v := parsed["max_latency_ms"]; v == nil {
		t.Error("max_latency_ms should be present, got nil")
	}
}
