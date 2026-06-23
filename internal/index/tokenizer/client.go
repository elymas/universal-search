package tokenizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

// Client sends tokenization requests to the mecab-ko sidecar.
//
// # @MX:ANCHOR: [AUTO] Primary sidecar integration point; callers: index shard router, tests
// # @MX:REASON: fan_in >= 3; all Korean document indexing and query routing flows through here
// # @MX:SPEC: SPEC-IDX-003
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient creates a Client using the given Config.
// The underlying HTTP client uses the per-request Timeout as its transport timeout.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Tokenize sends text to POST /tokenize and returns the parsed Result.
//
// Retry policy: up to cfg.MaxRetries additional attempts on 5xx or network errors.
// Backoff: attempt 1 → RetryBaseDelay, attempt 2 → RetryBaseDelay*3, each ± 10% jitter.
// 400 responses are not retried (ErrInvalidInput is returned immediately).
//
// REQ-IDX-003-006: graceful degradation metadata set by caller when this returns an error.
func (c *Client) Tokenize(ctx context.Context, requestID, text string) (*Result, error) {
	body, err := json.Marshal(Request{RequestID: requestID, Text: text})
	if err != nil {
		return nil, fmt.Errorf("tokenizer: marshal request: %w", err)
	}

	var lastErr error
	delay := c.cfg.RetryBaseDelay

	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Apply jitter: ± 10% of the base delay.
			jitter := time.Duration(float64(delay) * (0.9 + 0.2*rand.Float64())) // #nosec G404 -- non-cryptographic jitter for retry/backoff, not a security context
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("tokenizer: %w", ErrTimeout)
			case <-time.After(jitter):
			}
			delay *= 3 // exponential: 100ms → 300ms
		}

		result, err := c.doRequest(ctx, body)
		if err == nil {
			return result, nil
		}
		// Do not retry invalid-input errors (400).
		if err == ErrInvalidInput { //nolint:errorlint
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("%w: %v", ErrSidecarUnreachable, lastErr)
}

// doRequest executes a single POST /tokenize request.
func (c *Client) doRequest(ctx context.Context, body []byte) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/tokenize", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tokenizer: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tokenizer: http do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var result Result
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("tokenizer: decode response: %w", err)
		}
		return &result, nil

	case http.StatusBadRequest:
		return nil, ErrInvalidInput

	default:
		return nil, fmt.Errorf("tokenizer: unexpected status %d", resp.StatusCode)
	}
}
