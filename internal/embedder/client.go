// Package embedder — Go HTTP client for the BGE-M3 embedding sidecar.
//
// REQ-IDX-002-005: context timeout, exponential backoff retry (2 retries).
// REQ-IDX-002-006: Per-call observability (slog + prometheus counter + histogram + OTel span).
// Mirrors internal/synthesis/client.go pattern per SPEC-IDX-002 §5.3.
package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used -- non-crypto backoff jitter only (see #nosec G404 at usage site)
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/obs"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// retryBase is the base backoff delay for connection-level errors.
	retryBase = 500 * time.Millisecond
	// retryMult is the multiplier for each subsequent retry (500ms → 1500ms).
	retryMult = 3
	// retryJitter is the max jitter fraction (±10%).
	retryJitter = 0.1
	// maxRetries is the maximum number of retry attempts on connection errors.
	maxRetries = 2
)

// Client is the Go-side HTTP client for the BGE-M3 embedding sidecar.
//
// @MX:ANCHOR: [AUTO] Embedder client public API; callers: SPEC-IDX-001, tests, future SPEC-DEEP-*
// @MX:REASON: fan_in >= 3; all Go-side embed calls flow through Embed method.
type Client struct {
	httpClient *http.Client
	baseURL    string
	o          *obs.Obs
}

// New constructs a Client from cfg and an optional obs bundle.
// obs may be nil (tests / minimal deployments).
func New(cfg Config, o *obs.Obs) (*Client, error) {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.RequestTimeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		o:          o,
	}, nil
}

// Embed sends an embed request to the sidecar and returns the embedding response.
//
// REQ-IDX-002-005: context timeout, retry on connection errors, no retry on 4xx.
// REQ-IDX-002-006: single counter/histogram/span per top-level call (NOT per retry).
func (c *Client) Embed(ctx context.Context, req Request) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()

	mode := ModeLabel(req)
	outcome := "error_unreachable"
	started := time.Now()

	var span oteltrace.Span
	if c.o != nil && c.o.HasTracer() {
		ctx, span = c.o.Tracer("embedder").Start(ctx, "embedder.call")
		defer span.End()
	}

	// Observability emitted exactly once per top-level call (after resolution).
	defer func() {
		elapsed := time.Since(started)
		c.emitObs(span, outcome, mode, elapsed, req)
	}()

	body, err := json.Marshal(req)
	if err != nil {
		outcome = "error_invalid"
		return Response{}, fmt.Errorf("embedder: marshal request: %w", err)
	}

	var resp Response
	err = c.withRetry(ctx, func() error {
		return c.doOnce(ctx, body, &resp)
	})

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "error_timeout"
		return Response{}, fmt.Errorf("%w: %w", ErrTimeout, err)
	case errors.Is(err, ErrInvalidRequest):
		outcome = "error_invalid"
		return Response{}, err
	case errors.Is(err, ErrModelLoadFailed):
		outcome = "error_loading"
		return Response{}, err
	case errors.Is(err, ErrOutOfMemory):
		outcome = "error_oom"
		return Response{}, err
	case err != nil:
		outcome = "error_unreachable"
		return Response{}, err
	default:
		outcome = "success"
	}

	// Increment cache-hit counter on success (nil-safe).
	if resp.CacheHits > 0 && c.o != nil && c.o.Metrics != nil && c.o.Metrics.EmbedderCacheHits != nil {
		c.o.Metrics.EmbedderCacheHits.Add(float64(resp.CacheHits))
	}

	return resp, nil
}

// doOnce performs a single HTTP POST to the sidecar and decodes the response.
// Returns typed sentinel errors for 4xx / 5xx status codes.
func (c *Client) doOnce(ctx context.Context, body []byte, out *Response) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("embedder: http do: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	switch {
	case httpResp.StatusCode >= 400 && httpResp.StatusCode < 500:
		// Parse error payload to check for specific codes.
		var errBody struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(httpResp.Body).Decode(&errBody)
		return fmt.Errorf("%w: %s", ErrInvalidRequest, errBody.Detail)

	case httpResp.StatusCode == 503:
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(httpResp.Body).Decode(&errBody)
		if errBody.Error == "model_loading" {
			return ErrModelLoadFailed
		}
		return fmt.Errorf("embedder: service unavailable (503)")

	case httpResp.StatusCode == 500:
		var errBody struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(httpResp.Body).Decode(&errBody)
		if errBody.Error == "oom" {
			return ErrOutOfMemory
		}
		return fmt.Errorf("embedder: internal server error: %s", errBody.Detail)

	case httpResp.StatusCode >= 500:
		return fmt.Errorf("embedder: server error %d", httpResp.StatusCode)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(out); err != nil {
		return fmt.Errorf("embedder: decode response: %w", err)
	}
	return nil
}

// withRetry executes fn up to maxRetries+1 times, retrying only on connection-level errors.
// HTTP 4xx and 5xx responses pass through directly (no retry for 4xx; caller decides for 5xx).
func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with ±10% jitter.
			base := retryBase * time.Duration(retryMult*(attempt))
			jitter := time.Duration(float64(base) * retryJitter * (rand.Float64()*2 - 1)) // #nosec G404 -- non-cryptographic jitter for retry/backoff, not a security context
			delay := base + jitter
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		// Only retry on connection-level errors (not on typed HTTP errors).
		if !isConnectionError(err) {
			return err
		}
	}
	return lastErr
}

// isConnectionError reports whether err represents a connection-level failure
// that warrants a retry (net.OpError, url.Error wrapping connection errors).
func isConnectionError(err error) bool {
	var netErr *net.OpError
	var urlErr *url.Error
	return errors.As(err, &netErr) || errors.As(err, &urlErr)
}

// emitObs emits one slog record + one counter increment + one histogram observation
// per top-level Embed call. Nil-safe across obs, Metrics, and Logger.
func (c *Client) emitObs(
	span oteltrace.Span,
	outcome, mode string,
	elapsed time.Duration,
	req Request,
) {
	elapsedSec := elapsed.Seconds()

	// slog record.
	if c.o != nil && c.o.Logger != nil {
		c.o.Logger.Info("embedder.call",
			slog.String("outcome", outcome),
			slog.String("mode", mode),
			slog.Int("texts_count", len(req.Texts)),
			slog.Float64("latency_seconds", elapsedSec),
			slog.String("request_id", req.RequestID),
		)
	}

	// Prometheus counter.
	if c.o != nil && c.o.Metrics != nil && c.o.Metrics.EmbedderCalls != nil {
		c.o.Metrics.EmbedderCalls.WithLabelValues(outcome, mode).Inc()
	}

	// Prometheus histogram.
	if c.o != nil && c.o.Metrics != nil && c.o.Metrics.EmbedderLatency != nil {
		c.o.Metrics.EmbedderLatency.WithLabelValues(outcome, mode).Observe(elapsedSec)
	}

	// OTel span attributes.
	if span != nil {
		span.SetAttributes(
			attribute.String("request_id", req.RequestID),
			attribute.Int("texts_count", len(req.Texts)),
			attribute.String("mode", mode),
			attribute.String("outcome", outcome),
			attribute.Float64("latency_seconds", elapsedSec),
		)
	}
}
