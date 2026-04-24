// Package llm is the Universal Search LLM gateway client. It targets the
// LiteLLM proxy (deploy/docker-compose.yml litellm service) via an
// OpenAI-compatible HTTP surface, adds provider priority routing, retry with
// exponential backoff and fallthrough, circuit breaking, cost tracking, and
// a per-request budget cap. All LLM I/O in the Go orchestration plane flows
// through this package.
//
// Domain packages MUST NOT import github.com/openai/openai-go directly.
//
// REQ-LLM-002: Client interface with Complete, Stream, Embed methods.
// REQ-LLM-004: Retry with exponential backoff and provider fallthrough.
// REQ-LLM-005: Auth via LITELLM_MASTER_KEY; key never logged.
// REQ-LLM-006: Cost extraction from x-litellm-response-cost header.
// NFR-LLM-002: Per-provider circuit breaker.
// NFR-LLM-003: Per-request budget cap (ErrBudgetExceeded).
package llm

import (
	"context"
	"errors"

	"github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
)

// ModelClass identifies a task tier for model selection.
// The router maps each class to a provider priority list.
type ModelClass string

const (
	// DeepResearch maps to claude-opus-4-7 (primary) → gpt-4o (fallback).
	DeepResearch ModelClass = "deep_research"
	// Summary maps to claude-sonnet-4-6 → gpt-4o-mini → ollama/llama3.1.
	Summary ModelClass = "summary"
	// Classify maps to claude-haiku-4-5 → gpt-4o-mini → ollama/llama3.1:8b.
	Classify ModelClass = "classify"
	// Embed maps to text-embedding-3-large (embeddings endpoint).
	Embed ModelClass = "embed"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    string // "user" | "assistant" | "system"
	Content string
}

// Request is a provider-agnostic LLM completion request.
type Request struct {
	Class       ModelClass
	System      string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	// Override routes this request to a specific model alias, bypassing Class routing.
	Override string
}

// Response is a provider-agnostic LLM completion response.
// Fields match SPEC-LLM-001 §5 acceptance criteria for REQ-LLM-002.
type Response struct {
	Text             string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMs        int64
	CostUSD          float64
	FinishReason     string
}

// Delta is a single streaming chunk from Client.Stream.
type Delta struct {
	Content      string
	FinishReason string
	// Err is populated on the final delta when the stream ended in error.
	Err error
}

// EmbedRequest is a provider-agnostic embedding request.
type EmbedRequest struct {
	Class ModelClass // must be Embed
	Input []string
}

// EmbedResponse is a provider-agnostic embedding response.
type EmbedResponse struct {
	Vectors       [][]float32
	Provider      string
	Model         string
	PromptTokens  int
	LatencyMs     int64
	CostUSD       float64
}

// Client is the primary interface for LLM operations.
// All three methods accept context.Context for cancellation.
//
// @MX:ANCHOR: [AUTO] Primary LLM interface; callers: cmd/usearch, SPEC-SYN-001, SPEC-DEEP-*, tests
// @MX:REASON: fan_in >= 3; all LLM I/O in Go plane flows through this interface
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan Delta, error)
	Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
	Close() error
}

// Sentinel errors returned by Client implementations.
var (
	// ErrBudgetExceeded is returned when response cost exceeds PerRequestCapUSD.
	// The Response is still returned alongside this error.
	ErrBudgetExceeded = errors.New("llm: per-request budget exceeded")

	// ErrStreamBackpressureTimeout is returned when a stream consumer stalls for >30s.
	ErrStreamBackpressureTimeout = errors.New("llm: stream consumer stalled")

	// ErrAllProvidersFailed is returned when all providers in the priority list are exhausted.
	ErrAllProvidersFailed = errors.New("llm: all providers in priority list exhausted")

	// ErrModelNotConfigured is returned when a ModelClass has no configured provider.
	ErrModelNotConfigured = errors.New("llm: model class has no configured provider")
)

// New constructs a Client wired to the LiteLLM proxy defined by cfg and
// emits telemetry through the provided obs bundle.
//
// @MX:ANCHOR: [AUTO] LLM client constructor; callers: cmd/usearch, cmd/usearch-api, cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 3; sole entry point for creating a production Client
func New(cfg config.Config, o *obs.Obs) (Client, error) {
	return newDefaultClient(cfg, o)
}
