// Package synthesis — Go-side HTTP client for the researcher sidecar.
//
// REQ-SYN-005: context timeout, exponential backoff retry (2 retries).
// REQ-SYN-006: Per-call observability (slog + prometheus counter + histogram + OTel span).
package synthesis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/pkg/types"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/oklog/ulid/v2"
)

const (
	// retryBase is the base backoff delay.
	retryBase = 500 * time.Millisecond
	// retryMult is the multiplier for each subsequent retry (500ms → 1500ms).
	retryMult = 3
	// retryJitter is the max jitter fraction (±10%).
	retryJitter = 0.1
)

// Client is the Go-side HTTP client for the researcher sidecar.
//
// @MX:ANCHOR: [AUTO] Synthesis client public API; callers: cmd/usearch, CLI, tests
// @MX:REASON: fan_in >= 3; all Go-side synthesis calls flow through Synthesize.
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
		baseURL:    cfg.BaseURL,
		o:          o,
	}, nil
}

// Synthesize sends a synthesis request to the sidecar and returns the result.
//
// REQ-SYN-005: context timeout applied via cfg.RequestTimeout.
// REQ-SYN-006: counter, histogram, OTel span emitted once per top-level call.
func (c *Client) Synthesize(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (Result, error) {
	// Apply wall-clock timeout.
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()

	// Start OTel span.
	reqID := ulid.Make().String()
	var span oteltrace.Span
	if c.o != nil {
		ctx, span = c.o.Tracer("synthesis").Start(ctx, "synthesis.call")
		defer span.End()
	}

	started := time.Now()
	outcome := "error_unreachable"

	defer func() {
		elapsed := time.Since(started)
		c.emitObs(ctx, span, outcome, elapsed, reqID, query, docs, Result{})
	}()

	payload := c.buildPayload(reqID, query, lang, docs)
	body, err := json.Marshal(payload)
	if err != nil {
		outcome = "error_invalid"
		return Result{}, fmt.Errorf("synthesis: marshal payload: %w", err)
	}

	var result Result

	// @MX:WARN: [AUTO] withRetry executes up to 3 HTTP calls; timeout applies across all retries.
	// @MX:REASON: Retry storm risk if server is consistently slow; bounded by ctx deadline.
	err = withRetry(ctx, 2, func() error {
		return c.doOnce(ctx, body, &result)
	})

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "error_timeout"
		return Result{}, fmt.Errorf("synthesis: %w: %w", ErrTimeout, err)
	case errors.Is(err, ErrInvalidRequest):
		outcome = "error_invalid"
		return Result{}, err
	case errors.Is(err, ErrSidecarUnreachable):
		outcome = "error_unreachable"
		return Result{}, err
	case err != nil:
		outcome = "error_unreachable"
		return Result{}, err
	case result.Degraded:
		outcome = "degraded"
	default:
		outcome = "success"
	}

	// Cost counter is only updated on success path (not degraded per NFR-SYN-004).
	if outcome == "success" && result.CostUSD > 0 && c.o != nil {
		if c.o.Metrics != nil && c.o.Metrics.SynthesisCost != nil {
			c.o.Metrics.SynthesisCost.Add(result.CostUSD)
		}
	}

	return result, nil
}

// doOnce performs a single HTTP POST to the sidecar.
func (c *Client) doOnce(ctx context.Context, body []byte, result *Result) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/synthesize", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("synthesis: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Classify connection-level errors for retry logic.
		var netErr *net.OpError
		var urlErr *url.Error
		if errors.As(err, &netErr) || errors.As(err, &urlErr) {
			return fmt.Errorf("synthesis: connection error: %w", ErrSidecarUnreachable)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("synthesis: http: %w", ErrSidecarUnreachable)
	}
	defer func() { _ = resp.Body.Close() }()

	// 4xx → ErrInvalidRequest (no retry).
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return ErrInvalidRequest
	}
	// 5xx → transient; caller (withRetry) will retry.
	if resp.StatusCode >= 500 {
		return fmt.Errorf("synthesis: server error %d: %w", resp.StatusCode, ErrSidecarUnreachable)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("synthesis: decode response: %w", err)
	}
	return nil
}

// buildPayload converts types.NormalizedDoc slice to the sidecar Request shape.
func (c *Client) buildPayload(reqID, query, lang string, docs []types.NormalizedDoc) Request {
	payloadDocs := make([]Doc, len(docs))
	for i, d := range docs {
		payloadDocs[i] = Doc{
			ID:          d.ID,
			SourceID:    d.SourceID,
			URL:         d.URL,
			Title:       d.Title,
			Body:        d.Body,
			Snippet:     d.Snippet,
			PublishedAt: d.PublishedAt.UTC().Format(time.RFC3339),
			RetrievedAt: d.RetrievedAt.UTC().Format(time.RFC3339),
			Author:      d.Author,
			Score:       d.Score,
			Lang:        d.Lang,
			DocType:     string(d.DocType),
			Citations:   d.Citations,
			Metadata:    d.Metadata,
			Hash:        d.Hash,
		}
	}
	return Request{
		RequestID: reqID,
		Query:     query,
		Lang:      lang,
		Docs:      payloadDocs,
	}
}

// emitObs fires the single set of observability signals for a Synthesize call.
// REQ-SYN-006: counter +1, histogram observation, OTel span attributes.
func (c *Client) emitObs(
	ctx context.Context,
	span oteltrace.Span,
	outcome string,
	elapsed time.Duration,
	reqID, query string,
	docs []types.NormalizedDoc,
	result Result,
) {
	elapsedSec := elapsed.Seconds()
	latencyMs := elapsed.Milliseconds()

	if c.o != nil {
		if c.o.Logger != nil {
			c.o.Logger.InfoContext(ctx, "synthesis.call",
				slog.String("request_id", reqID),
				slog.String("outcome", outcome),
				slog.Int("query_len", len(query)),
				slog.Int("docs_count", len(docs)),
				slog.Int64("latency_ms", latencyMs),
			)
		}
		if c.o.Metrics != nil {
			if c.o.Metrics.SynthesisCalls != nil {
				c.o.Metrics.SynthesisCalls.WithLabelValues(outcome).Inc()
			}
			if c.o.Metrics.SynthesisLatency != nil {
				c.o.Metrics.SynthesisLatency.WithLabelValues(outcome).Observe(elapsedSec)
			}
		}
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("request_id", reqID),
			attribute.Int("query_len", len(query)),
			attribute.Int("docs_count", len(docs)),
			attribute.String("outcome", outcome),
			attribute.Float64("latency_ms", float64(latencyMs)),
			attribute.Bool("degraded", result.Degraded),
			attribute.String("model", result.Model),
			attribute.Float64("cost_usd", result.CostUSD),
		)
	}
}

// withRetry executes fn up to maxRetries+1 times with exponential backoff.
// Retry is only attempted on ErrSidecarUnreachable errors (connection-level).
// 4xx errors (ErrInvalidRequest) and context cancellation are not retried.
//
// @MX:WARN: [AUTO] Retry loop; up to 3 HTTP calls per Synthesize invocation.
// @MX:REASON: Bounded by ctx deadline; jitter avoids thundering-herd on restarts.
func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		// Do not retry on client errors or deadline exceeded.
		if errors.Is(lastErr, ErrInvalidRequest) || errors.Is(lastErr, context.DeadlineExceeded) {
			return lastErr
		}
		// Do not retry after last attempt.
		if attempt == maxRetries {
			break
		}
		// Exponential backoff: 500ms, 1500ms ± 10% jitter.
		base := retryBase * time.Duration(1<<uint(attempt)*retryMult/retryMult)
		if attempt == 1 {
			base = 1500 * time.Millisecond
		}
		jitter := time.Duration(float64(base) * retryJitter * (rand.Float64()*2 - 1))
		delay := base + jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}
