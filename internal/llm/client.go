// Package llm — default Client implementation backed by openai-go targeting LiteLLM proxy.
// REQ-LLM-002: Complete, Stream, Embed methods.
// REQ-LLM-003: Per-call observability (slog + counter + histogram + OTel span).
// REQ-LLM-004: Retry with exponential backoff; provider fallthrough.
// REQ-LLM-005: Bearer auth; key never logged.
// REQ-LLM-006: Cost header extraction.
// NFR-LLM-002: Circuit breaker via Router.
// NFR-LLM-003: Budget cap check.
package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/obs/reqid"
)

// defaultClient implements Client via openai-go → LiteLLM proxy.
//
// @MX:ANCHOR: [AUTO] Default LLM client implementation; callers: llm.New, cmd/usearch, tests
// @MX:REASON: fan_in >= 3; all production LLM calls flow through this struct
type defaultClient struct {
	cfg    config.Config
	obs    *obs.Obs
	oc     openai.Client
	router *Router
}

// newDefaultClient constructs the default client implementation.
func newDefaultClient(cfg config.Config, o *obs.Obs) (Client, error) {
	if o == nil {
		return nil, errors.New("llm: obs bundle must not be nil")
	}

	oc := openai.NewClient(
		option.WithBaseURL(cfg.BaseURL+"/v1"),
		option.WithAPIKey(cfg.MasterKey),
		option.WithHTTPClient(&http.Client{
			Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: reqid.NewTransport(http.DefaultTransport),
		}),
	)

	router := NewRouter(defaultPriorities)

	return &defaultClient{
		cfg:    cfg,
		obs:    o,
		oc:     oc,
		router: router,
	}, nil
}

// Complete sends a chat completion request with retry and fallthrough.
// REQ-LLM-002, REQ-LLM-003, REQ-LLM-004, REQ-LLM-005, REQ-LLM-006, NFR-LLM-003
func (c *defaultClient) Complete(ctx context.Context, req Request) (Response, error) {
	providers, err := c.router.Route(ctx, req.Class)
	if err != nil {
		return Response{}, err
	}

	var lastErr error
	for _, ref := range providers {
		resp, err := c.completeWithProvider(ctx, ref, req)
		if err != nil {
			// Budget exceeded: return response AND error immediately (NFR-LLM-003 HARD rule).
			// Response must not be discarded even when cap is exceeded.
			if errors.Is(err, ErrBudgetExceeded) {
				c.router.Record(ref.Provider, true)
				return resp, err
			}
			if isNonRetryable(err) {
				return Response{}, err
			}
			c.router.Record(ref.Provider, false)
			lastErr = err
			continue
		}
		c.router.Record(ref.Provider, true)
		return resp, nil
	}

	if lastErr != nil {
		return Response{}, lastErr
	}
	return Response{}, ErrAllProvidersFailed
}

// completeWithProvider sends one request to a specific provider with retries.
func (c *defaultClient) completeWithProvider(ctx context.Context, ref ProviderRef, req Request) (Response, error) {
	var result Response
	var resultErr error

	model := ref.Model
	if req.Override != "" {
		model = req.Override
	}

	retryErr := withRetry(ctx, func() error {
		resp, err := c.doComplete(ctx, ref.Provider, model, req)
		if err != nil {
			// Budget exceeded is not retried; carry through result.
			if errors.Is(err, ErrBudgetExceeded) {
				result = resp
				resultErr = err
				return nil // stop retry, return success from withRetry
			}
			return err
		}
		result = resp
		return nil
	})

	if retryErr != nil {
		return Response{}, retryErr
	}
	return result, resultErr
}

// doComplete executes a single HTTP chat completion request.
func (c *defaultClient) doComplete(ctx context.Context, provider, model string, req Request) (Response, error) {
	start := time.Now()

	// Cost capture via cost-aware RoundTripper.
	var cost float64
	costRT := newCostMiddlewareRoundTripper(http.DefaultTransport, &cost, c.obs.Logger)
	oc := openai.NewClient(
		option.WithBaseURL(c.cfg.BaseURL+"/v1"),
		option.WithAPIKey(c.cfg.MasterKey),
		option.WithHTTPClient(&http.Client{
			Timeout:   time.Duration(c.cfg.TimeoutSeconds) * time.Second,
			Transport: reqid.NewTransport(costRT),
		}),
	)

	// Build messages.
	msgs := buildMessages(req)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}

	// Start OTel span.
	tracer := c.obs.Tracer("llm")
	spanCtx, span := tracer.Start(ctx, "llm.call",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	completion, err := oc.Chat.Completions.New(spanCtx, params)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		wrapped := redactKey(err.Error(), c.cfg.MasterKey)
		span.SetStatus(codes.Error, wrapped)
		c.emitObservability(spanCtx, provider, model, "failure", 0, 0, latencyMs, 0)
		statusCode := extractHTTPStatus(err)
		return Response{}, &httpStatusError{code: statusCode, msg: wrapped}
	}

	text := ""
	finishReason := ""
	promptTokens := int(completion.Usage.PromptTokens)
	completionTokens := int(completion.Usage.CompletionTokens)
	if len(completion.Choices) > 0 {
		text = completion.Choices[0].Message.Content
		finishReason = string(completion.Choices[0].FinishReason)
	}

	// Set OTel span attributes (SPEC §5.1).
	span.SetAttributes(
		attribute.String("llm.provider", provider),
		attribute.String("llm.model", model),
		attribute.Int("llm.prompt_tokens", promptTokens),
		attribute.Int("llm.completion_tokens", completionTokens),
		attribute.Float64("llm.cost_usd", cost),
	)

	c.emitObservability(spanCtx, provider, model, "success", promptTokens, completionTokens, latencyMs, cost)

	resp := Response{
		Text:             text,
		Provider:         provider,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		LatencyMs:        latencyMs,
		CostUSD:          cost,
		FinishReason:     finishReason,
	}

	// Post-flight budget cap (NFR-LLM-003).
	if budgetErr := checkBudget(cost, c.cfg.PerRequestCapUSD); budgetErr != nil {
		rid := reqid.FromContext(spanCtx)
		c.obs.Logger.WarnContext(spanCtx, "llm budget cap exceeded",
			slog.String("request_id", rid),
			slog.String("provider", provider),
			slog.String("model", model),
			slog.Float64("cost_usd", cost),
			slog.Float64("cap_usd", c.cfg.PerRequestCapUSD),
		)
		return resp, budgetErr
	}

	return resp, nil
}

// emitObservability emits slog, counter, and histogram for one LLM call.
// REQ-LLM-003
func (c *defaultClient) emitObservability(ctx context.Context, provider, model, outcome string, prompt, completion int, latencyMs int64, cost float64) {
	rid := reqid.FromContext(ctx)

	c.obs.Logger.InfoContext(ctx, "llm call",
		slog.String("request_id", rid),
		slog.String("provider", provider),
		slog.String("model", model),
		slog.Int("prompt_tokens", prompt),
		slog.Int("completion_tokens", completion),
		slog.Int64("latency_ms", latencyMs),
		slog.Float64("cost_usd", cost),
	)

	if metrics.LLMCalls != nil {
		metrics.LLMCalls.WithLabelValues(provider, model, outcome).Inc()
	}
	if metrics.LLMLatency != nil {
		latencySec := float64(latencyMs) / 1000.0
		metrics.LLMLatency.WithLabelValues(provider, model).Observe(latencySec)
	}
	emitCostMetric(ctx, provider, model, cost)
}

// Stream sends a streaming chat completion request.
// REQ-LLM-008
func (c *defaultClient) Stream(ctx context.Context, req Request) (<-chan Delta, error) {
	providers, err := c.router.Route(ctx, req.Class)
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, ErrAllProvidersFailed
	}

	ref := providers[0]
	model := ref.Model
	if req.Override != "" {
		model = req.Override
	}

	msgs := buildMessages(req)
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stream := c.oc.Chat.Completions.NewStreaming(streamCtx, params)

	ch := make(chan Delta, streamChannelCap)
	go runStream(streamCtx, cancel, stream, ch)

	return ch, nil
}

// Embed sends an embedding request.
// REQ-LLM-002
func (c *defaultClient) Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error) {
	providers, err := c.router.Route(ctx, req.Class)
	if err != nil {
		return EmbedResponse{}, err
	}
	if len(providers) == 0 {
		return EmbedResponse{}, ErrAllProvidersFailed
	}

	ref := providers[0]
	start := time.Now()

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(ref.Model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: req.Input,
		},
	}

	result, err := c.oc.Embeddings.New(ctx, params)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return EmbedResponse{}, fmt.Errorf("embed: %w", err)
	}

	vectors := make([][]float32, len(result.Data))
	for i, emb := range result.Data {
		floats := make([]float32, len(emb.Embedding))
		for j, v := range emb.Embedding {
			floats[j] = float32(v)
		}
		vectors[i] = floats
	}

	return EmbedResponse{
		Vectors:      vectors,
		Provider:     ref.Provider,
		Model:        ref.Model,
		PromptTokens: int(result.Usage.PromptTokens),
		LatencyMs:    latencyMs,
	}, nil
}

// Close releases resources held by the client.
func (c *defaultClient) Close() error {
	return nil
}

// buildMessages converts Request to openai-go message params.
func buildMessages(req Request) []openai.ChatCompletionMessageParamUnion {
	var msgs []openai.ChatCompletionMessageParamUnion
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, openai.UserMessage(m.Content))
		case "assistant":
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		case "system":
			msgs = append(msgs, openai.SystemMessage(m.Content))
		}
	}
	return msgs
}

// extractHTTPStatus reads an HTTP status code from an openai-go error.
func extractHTTPStatus(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	for _, code := range []int{400, 401, 403, 404, 408, 429, 500, 502, 503, 504} {
		if strings.Contains(msg, fmt.Sprintf("%d", code)) {
			return code
		}
	}
	return 0
}

// redactKey removes all occurrences of key from s (secret redaction).
// REQ-LLM-005
func redactKey(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[REDACTED]")
}
