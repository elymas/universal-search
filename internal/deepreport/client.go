package deepreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// retryMaxAttempts is the maximum number of retries on 503 (total attempts = 3).
	retryMaxAttempts = 2
	// retryBaseDelay is the base delay for exponential backoff.
	retryBaseDelay = 500 * time.Millisecond
	// retryMaxDelay caps the backoff delay.
	retryMaxDelay = 3 * time.Second
)

// Client is the Go-side HTTP client for the STORM sidecar.
//
// @MX:ANCHOR: [AUTO] Deepreport client public API; callers: cmd/usearch, handlers, tests
// @MX:REASON: fan_in >= 3; all Go-side deep report calls flow through GenerateReport.
type Client struct {
	httpClient *http.Client
	cfg        Config
	outcomes   *prometheus.CounterVec
	latency    prometheus.Histogram
}

// NewClient constructs a Client from cfg without Prometheus metrics.
// Used for minimal deployments or when observability is not required.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		cfg:        cfg,
	}
}

// NewClientWithMetrics constructs a Client from cfg with Prometheus metric collectors.
// The outcomes CounterVec must have a single label "outcome".
func NewClientWithMetrics(cfg Config, outcomes *prometheus.CounterVec, latency prometheus.Histogram) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		cfg:        cfg,
		outcomes:   outcomes,
		latency:    latency,
	}
}

// GenerateReport sends a report generation request to the STORM sidecar and
// returns the parsed Report.
//
// REQ-DEEP1-001: POST /generate_report returns structured report JSON.
// REQ-DEEP1-004: HTTP status → error sentinel mapping.
func (c *Client) GenerateReport(ctx context.Context, req *Request) (*Report, error) {
	started := time.Now()
	outcome := "error_upstream"

	defer func() {
		elapsed := time.Since(started).Seconds()
		if c.outcomes != nil {
			c.outcomes.WithLabelValues(outcome).Inc()
		}
		if c.latency != nil {
			c.latency.Observe(elapsed)
		}
	}()

	body, err := json.Marshal(req)
	if err != nil {
		outcome = "error_invalid"
		return nil, fmt.Errorf("deepreport: marshal request: %w", err)
	}

	var report Report

	// @MX:WARN: [AUTO] Retry loop; up to 3 HTTP calls per GenerateReport invocation.
	// @MX:REASON: Bounded by ctx deadline; jitter avoids thundering-herd on restarts.
	err = c.withRetry(ctx, func() error {
		return c.doOnce(ctx, body, &report)
	})

	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		outcome = "deadline_exceeded"
		return nil, fmt.Errorf("deepreport: %w: %w", ErrTimeout, err)
	case errors.Is(err, ErrInvalidRequest):
		outcome = "error_invalid"
		return nil, err
	case errors.Is(err, ErrDeadlineExceeded):
		outcome = "deadline_exceeded"
		return nil, err
	case errors.Is(err, ErrBudgetExceeded):
		outcome = "budget_exceeded"
		return nil, err
	case errors.Is(err, ErrSidecarUnreachable):
		outcome = "error_upstream"
		return nil, err
	case err != nil:
		outcome = "error_upstream"
		return nil, err
	default:
		outcome = "success"
	}

	return &report, nil
}

// doOnce performs a single HTTP POST to the STORM sidecar.
func (c *Client) doOnce(ctx context.Context, body []byte, report *Report) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.SidecarURL+"/generate_report", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("deepreport: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Classify connection-level errors.
		var netErr *net.OpError
		var urlErr *url.Error
		if errors.As(err, &netErr) || errors.As(err, &urlErr) {
			return fmt.Errorf("deepreport: connection error: %w", ErrSidecarUnreachable)
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("deepreport: http: %w", ErrSidecarUnreachable)
	}
	defer func() { _ = resp.Body.Close() }()

	// Map status codes to error sentinels per REQ-DEEP1-004.
	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(report); err != nil {
			return fmt.Errorf("deepreport: decode response: %w", err)
		}
		return nil
	case http.StatusUnprocessableEntity:
		return ErrInvalidRequest
	case http.StatusGatewayTimeout:
		return ErrDeadlineExceeded
	case http.StatusPaymentRequired:
		return ErrBudgetExceeded
	default:
		// 503 is retryable; other 5xx are not retried but map to ErrSidecarUnreachable.
		if resp.StatusCode == http.StatusServiceUnavailable {
			// Return ErrSidecarUnreachable so retry logic picks it up.
			return fmt.Errorf("deepreport: service unavailable (503): %w", ErrSidecarUnreachable)
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("deepreport: server error %d: %w", resp.StatusCode, ErrSidecarUnreachable)
		}
		return fmt.Errorf("deepreport: unexpected status %d: %w", resp.StatusCode, ErrSidecarUnreachable)
	}
}

// withRetry executes fn up to retryMaxAttempts+1 times with exponential backoff.
// Retry is only attempted on ErrSidecarUnreachable errors (connection-level / 503).
// Non-retryable errors (ErrInvalidRequest, ErrDeadlineExceeded, ErrBudgetExceeded)
// and context cancellation are returned immediately.
func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= retryMaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		// Do not retry on non-retryable errors.
		if errors.Is(lastErr, ErrInvalidRequest) ||
			errors.Is(lastErr, ErrDeadlineExceeded) ||
			errors.Is(lastErr, ErrBudgetExceeded) ||
			errors.Is(lastErr, context.DeadlineExceeded) ||
			errors.Is(lastErr, context.Canceled) {
			return lastErr
		}
		// Do not retry after last attempt.
		if attempt == retryMaxAttempts {
			break
		}
		// Exponential backoff with jitter.
		delay := backoff(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// backoff computes an exponential backoff duration with jitter for the given attempt.
func backoff(attempt int) time.Duration {
	base := retryBaseDelay * time.Duration(1<<uint(attempt))
	if base > retryMaxDelay {
		base = retryMaxDelay
	}
	// Add ±10% jitter.
	jitter := time.Duration(float64(base) * 0.1 * (rand.Float64()*2 - 1)) // #nosec G404 -- non-cryptographic jitter for retry/backoff, not a security context
	delay := base + jitter
	if delay < 0 {
		delay = base
	}
	return delay
}
