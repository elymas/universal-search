// Package router — LLM-fallback adjudicator.
// SPEC-IR-001: REQ-IR-002 (escalation), REQ-IR-003 (circuit-breaker degrade),
// REQ-IR-007 (timeout degrade).
package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/llm"
)

// classifySystemPrompt is the constant-string system prompt for the LLM
// classifier. It is intentionally padded with examples to comfortably exceed
// the 1024-token Anthropic prompt-cache minimum (research §2.5). The exact
// bytes MUST be deterministic across calls so LiteLLM forwards an identical
// `cache_control` block on every request.
//
// Edits to this prompt invalidate the prompt cache for ~5 minutes (default
// TTL). Treat changes as observable cost events.
const classifySystemPrompt = `You are a query intent classifier for the Universal Search research meta-search engine. Your sole task is to classify a single user query into ONE of six categories and return a JSON object. Output ONLY the JSON object, with no preamble, no trailing commentary, and no markdown code fences.

# Output schema

{
  "category": "<one of: web | social | academic | korean | mixed | unknown>",
  "confidence": <float in [0.0, 1.0]>,
  "rationale": "<one sentence, max 200 characters>"
}

# Category definitions

- web: generic web search; news articles, blog posts, general info pages, encyclopedic content.
- social: queries primarily targeting social platforms — Reddit, Hacker News, X (Twitter), Bluesky, YouTube, TikTok, Discord, Polymarket prediction markets.
- academic: scholarly queries — arXiv papers, GitHub repositories or issues, conference proceedings (NeurIPS, ICLR, ICML, ACL), preprints, theses, citations, mathematical content.
- korean: queries primarily targeting Korean-locale sources — Naver search, Daum portal, Korean RSS feeds, Korean news sites, queries written in Hangul that target Korean services.
- mixed: queries with multi-category intent — for example, asking for Korean ML papers (academic AND korean), or comparing Reddit and Hacker News opinions on a topic (social AND web).
- unknown: cannot be confidently placed in any of the five named categories. Use this when the query is gibberish, too short, or genuinely ambiguous beyond useful interpretation. Unknown is recoverable; downstream systems will dispatch a default web+social ensemble.

# Confidence guidance

- 0.95-1.00: textbook example of the category, no other category fits.
- 0.80-0.94: strong fit, minor ambiguity but the category clearly dominates.
- 0.60-0.79: lean toward the category, secondary signals point elsewhere.
- 0.40-0.59: weak signal, multiple categories plausible (consider "mixed").
- 0.00-0.39: very low confidence; "unknown" is usually the better choice in this band.

# Examples

Query: "transformer attention is all you need 2017 paper"
Output: {"category":"academic","confidence":0.97,"rationale":"References a specific arXiv paper title with year"}

Query: "what's the best Python web framework according to Reddit"
Output: {"category":"social","confidence":0.89,"rationale":"Explicitly asks for Reddit consensus on a topic"}

Query: "ChatGPT 사용법과 프롬프트 엔지니어링 팁"
Output: {"category":"korean","confidence":0.96,"rationale":"Predominantly Hangul, targets Korean-language tutorials"}

Query: "best Korean LLM 모델 추천"
Output: {"category":"mixed","confidence":0.82,"rationale":"Code-mixed Korean and English seeking model recommendation"}

Query: "asdf qwerty"
Output: {"category":"unknown","confidence":0.20,"rationale":"Query is gibberish with no extractable intent"}

Query: "latest news about climate change 2026"
Output: {"category":"web","confidence":0.86,"rationale":"Generic news lookup with date qualifier"}

Query: "Hacker News thread on Rust async runtime tradeoffs"
Output: {"category":"social","confidence":0.93,"rationale":"Explicitly references a specific Hacker News thread topic"}

Query: "naver 뉴스 오늘의 헤드라인"
Output: {"category":"korean","confidence":0.95,"rationale":"Targets Naver Korean news portal directly"}

Query: "arxiv 2024 diffusion 모델 한국어 평가"
Output: {"category":"mixed","confidence":0.85,"rationale":"Combines arXiv academic intent with Korean-language evaluation focus"}

Query: "tiktok viral cooking trends"
Output: {"category":"social","confidence":0.88,"rationale":"Explicitly references TikTok viral content"}

Query: "github repo for stable diffusion training pipeline"
Output: {"category":"academic","confidence":0.84,"rationale":"GitHub repository for a research pipeline; classified as academic"}

Query: "best wireless earbuds review 2026"
Output: {"category":"web","confidence":0.81,"rationale":"Generic product review query with year qualifier"}

# Output rules

- Output ONLY the JSON object on a single logical line; do not wrap in markdown.
- Always populate every field — category, confidence, rationale.
- The rationale MUST be a single sentence, no longer than 200 characters.
- If genuinely uncertain, use "unknown" with confidence ≤ 0.4 rather than guessing.
`

// ClassifySystemPrompt is exported so tests can verify byte-identity (which
// is what the prompt cache requires).
func ClassifySystemPrompt() string {
	return classifySystemPrompt
}

// llmCompleter is the minimal subset of llm.Client this package consumes.
// Defined as an interface so tests can inject mocks without spinning up the
// real openai-go-backed client.
type llmCompleter interface {
	Complete(ctx context.Context, req llm.Request) (llm.Response, error)
}

// llmResponseShape is the expected JSON shape returned by the model.
type llmResponseShape struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

// llmClassify performs one LLM-fallback call. Returns:
//
//   - (cat, conf, rationale, nil) on success
//   - (_, _, _, ErrLLMTimeout) when the deadline is exceeded
//   - (_, _, _, ErrLLMUnavailable) when all providers are unavailable
//   - (_, _, _, ErrLLMParse) on parse / enum failure
//
// The deadline is the SHORTER of (deadline, ctx.Deadline) — caller's parent
// ctx wins per REQ-IR-007.
//
// @MX:WARN: [AUTO] Timeout-and-fall-through: callers must inspect the
// returned error and check Metadata flags. Silent degradation is by design
// (REQ-IR-003 + REQ-IR-007) but a foot-gun if the caller forgets to surface
// the flag.
// @MX:REASON: silent degradation hides LLM failure from naive callers;
// router.go::Classify is the canonical caller and handles flags correctly.
// @MX:SPEC: SPEC-IR-001
func llmClassify(ctx context.Context, client llmCompleter, query, modelOverride string, deadline time.Duration) (Category, float64, string, error) {
	if client == nil {
		return CategoryUnknown, 0, "", ErrLLMUnavailable
	}

	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	req := llm.Request{
		Class:       llm.Classify,
		System:      classifySystemPrompt,
		Messages:    []llm.Message{{Role: "user", Content: fmt.Sprintf("Classify this query: %q\nReturn ONLY the JSON object.", query)}},
		MaxTokens:   100,
		Temperature: 0,
	}
	if modelOverride != "" {
		req.Override = modelOverride
	}

	resp, err := client.Complete(callCtx, req)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			if ctx.Err() == nil {
				// Inner deadline fired before the parent ctx — treat as our timeout.
				return CategoryUnknown, 0, "", ErrLLMTimeout
			}
			return CategoryUnknown, 0, "", ErrLLMTimeout
		case errors.Is(err, llm.ErrAllProvidersFailed), errors.Is(err, llm.ErrModelNotConfigured):
			return CategoryUnknown, 0, "", ErrLLMUnavailable
		}
		return CategoryUnknown, 0, "", fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}

	cat, conf, rationale, parseErr := parseLLMResponse(resp.Text)
	if parseErr != nil {
		return CategoryUnknown, 0, "", parseErr
	}
	return cat, conf, rationale, nil
}

// parseLLMResponse extracts (Category, confidence, rationale) from the raw
// LLM text. Strips a leading ```json … ``` fence if present, json.Unmarshals,
// validates the enum, clamps confidence, and truncates rationale.
func parseLLMResponse(raw string) (Category, float64, string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return CategoryUnknown, 0, "", ErrLLMParse
	}
	body = stripCodeFence(body)

	var shape llmResponseShape
	if err := json.Unmarshal([]byte(body), &shape); err != nil {
		return CategoryUnknown, 0, "", fmt.Errorf("%w: %v", ErrLLMParse, err)
	}
	cat := Category(strings.ToLower(strings.TrimSpace(shape.Category)))
	if !cat.IsValid() {
		return CategoryUnknown, 0, "", fmt.Errorf("%w: invalid category %q", ErrLLMParse, shape.Category)
	}
	conf := clamp01(shape.Confidence)
	rationale := truncateString(shape.Rationale, 200)
	return cat, conf, rationale, nil
}

// stripCodeFence removes leading/trailing ```…``` markdown fences. Tolerant
// of `json` language hints and trailing newlines.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop opening fence including optional language hint.
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	} else {
		s = strings.TrimPrefix(s, "```")
	}
	// Drop closing fence.
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// truncateString truncates s to at most n bytes (not runes — sufficient for
// rationale length capping; non-UTF-8 boundary risk is acceptable here).
func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// LLMClassifyForTest is a thin shim exposing llmClassify to tests in the
// _test package. It is NOT part of the public API.
func LLMClassifyForTest(ctx context.Context, client llmCompleter, query, modelOverride string, deadline time.Duration) (Category, float64, string, error) {
	return llmClassify(ctx, client, query, modelOverride, deadline)
}
