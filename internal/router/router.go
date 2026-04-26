// Package router implements the Universal Search Intent Router (SPEC-IR-001).
//
// The Intent Router is a pure-library function: it accepts a RouterQuery and
// returns a RoutingDecision describing the predicted Category, the Confidence
// score, the language tag, the source of the classification, and the set of
// adapter names eligible to serve the query. The Router does NOT invoke any
// adapter; SPEC-FAN-001 (M3) consumes RoutingDecision.AdapterSet and
// dispatches in parallel.
//
// Pipeline:
//  1. Validate query (REQ-IR-005)
//  2. Apply deterministic rule-based scoring (REQ-IR-002, §2.3 formula)
//  3. If rule confidence < ConfidenceThreshold, escalate to LLM-fallback
//     (REQ-IR-002, REQ-IR-007 deadline, REQ-IR-003 circuit-breaker degrade)
//  4. Compute detected language (REQ-IR-004 honours caller override)
//  5. Select AdapterSet via Capabilities ∩ Category-eligible DocTypes
//     (REQ-IR-008)
//  6. Emit per-call observability (REQ-IR-006)
//
// The Router is concurrency-safe: the underlying Rules / capability cache /
// LLM client / obs bundle are all immutable post-construction.
package router

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// defaultLLMDeadline is the per-call deadline for the LLM-fallback step
// (REQ-IR-007). Two seconds leaves ~1 second of headroom under NFR-IR-002's
// 3-second p95 budget.
const defaultLLMDeadline = 2 * time.Second

// Options configures Router construction.
type Options struct {
	// Rules carries the keyword tables and scoring formula. nil → defaults.
	Rules *Rules
	// LLMClient is the LLM gateway used for fallback. nil → no LLM escalation
	// (rule-based result with degraded_confidence flag set if confidence is
	// below threshold).
	LLMClient llm.Client
	// Registry is the adapter registry. Required; New returns
	// ErrAdapterRegistryEmpty when this is nil or empty.
	Registry *adapters.Registry
	// Obs is the observability bundle. nil-safe — Router degrades to no-op
	// metrics/log/span emission per REQ-IR-006.
	Obs *obs.Obs
	// LLMModelOverride is the value applied to llm.Request.Override on
	// fallback calls. Empty string → use Class-tier routing (Haiku 4.5).
	LLMModelOverride string
	// LLMDeadline is the per-call deadline for the LLM fallback. Zero →
	// defaultLLMDeadline (2s).
	LLMDeadline time.Duration
	// ConfidenceThreshold is the rule-confidence floor above which the LLM
	// step is skipped. Zero → ConfidenceThreshold constant (0.85).
	ConfidenceThreshold float64
}

// Router classifies queries and selects the eligible adapter set.
//
// @MX:ANCHOR: [AUTO] Sole sanctioned classification entry point. Callers:
// FAN-001 fanout, CLI-001 CLI, SYN-001 synthesis, future debug tooling.
// @MX:REASON: fan_in >= 5 expected post-M2; all RoutingDecision values flow
// through Router.Classify. Behaviour change here cascades to every downstream.
// @MX:SPEC: SPEC-IR-001
type Router struct {
	rules               *Rules
	llmClient           llm.Client
	obs                 *obs.Obs
	caps                map[string]types.Capabilities
	adapterNames        []string
	confidenceThreshold float64
	llmModelOverride    string
	llmDeadline         time.Duration
}

// New constructs a Router from opts. Returns ErrAdapterRegistryEmpty when the
// registry has zero adapters.
func New(opts Options) (*Router, error) {
	if opts.Registry == nil {
		return nil, ErrAdapterRegistryEmpty
	}
	names := opts.Registry.List()
	if len(names) == 0 {
		return nil, ErrAdapterRegistryEmpty
	}

	caps := make(map[string]types.Capabilities, len(names))
	for _, n := range names {
		ad, ok := opts.Registry.Get(n)
		if !ok {
			continue
		}
		caps[n] = ad.Capabilities()
	}

	rules := opts.Rules
	if rules == nil {
		rules = NewDefaultRules()
	}
	threshold := opts.ConfidenceThreshold
	if threshold <= 0 {
		threshold = ConfidenceThreshold
	}
	deadline := opts.LLMDeadline
	if deadline <= 0 {
		deadline = defaultLLMDeadline
	}

	return &Router{
		rules:               rules,
		llmClient:           opts.LLMClient,
		obs:                 opts.Obs,
		caps:                caps,
		adapterNames:        append([]string(nil), names...),
		confidenceThreshold: threshold,
		llmModelOverride:    opts.LLMModelOverride,
		llmDeadline:         deadline,
	}, nil
}

// Classify runs the classification pipeline for q and returns the resulting
// RoutingDecision. Errors:
//
//   - ErrInvalidQuery — RouterQuery.Text is empty / whitespace-only.
//
// LLM-fallback errors (timeout, circuit open, parse failure) are NOT returned
// to the caller; Classify degrades to a rule-based result and records the
// situation via Metadata flags (llm_timeout / llm_unavailable /
// degraded_confidence) and the error_* outcome counter.
//
// @MX:ANCHOR: [AUTO] Classification orchestrator. Public Router entry point
// fan_in >= 5 expected.
// @MX:REASON: every RoutingDecision flows through this method; nil-safety
// invariants for obs and LLMClient are load-bearing.
// @MX:SPEC: SPEC-IR-001
func (r *Router) Classify(ctx context.Context, q RouterQuery) (RoutingDecision, error) {
	tracer := r.tracer()
	spanCtx, span := tracer.Start(ctx, "router.classify",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
	)
	defer span.End()
	start := time.Now()

	if err := q.Validate(); err != nil {
		r.emit(spanCtx, RoutingDecision{}, err, time.Since(start))
		return RoutingDecision{}, err
	}

	metadata := make(map[string]any, 8)
	hangul, particles := KoreanSignals(q.Text)
	metadata["hangul_ratio"] = hangul
	metadata["particle_density"] = particles

	// Lang resolution: caller override wins (REQ-IR-004).
	lang := q.Lang
	if lang != "" {
		metadata["lang_override"] = true
	} else {
		lang = detectLang(hangul)
	}

	cat, conf, triggers := r.rules.Score(q)
	metadata["rule_triggers"] = triggers

	source := SourceRuleBased

	// Escalate to LLM when rule confidence is below threshold.
	if conf < r.confidenceThreshold {
		if llmCat, llmConf, llmRationale, llmErr := r.classifyByLLM(spanCtx, q.Text); llmErr == nil {
			metadata["rule_confidence"] = conf
			metadata["llm_rationale"] = llmRationale
			cat = llmCat
			conf = llmConf
			source = SourceLLMFallback
		} else {
			metadata["degraded_confidence"] = true
			r.recordLLMFailure(spanCtx, metadata, llmErr, time.Since(start))
			// Continue with rule-based result; do not surface llmErr.
		}
	}

	dec := RoutingDecision{
		Category:   cat,
		Confidence: conf,
		Lang:       lang,
		Source:     source,
		Metadata:   metadata,
	}
	dec.AdapterSet = r.selectAdapterSet(cat, lang, metadata)

	r.emit(spanCtx, dec, nil, time.Since(start))
	return dec, nil
}

// classifyByLLM is the LLM-fallback wrapper. It is a no-op when no LLM client
// was injected.
//
// @MX:ANCHOR: [AUTO] LLM-fallback path; sole place that talks to internal/llm.
// fan_in >= 3 (Classify + tests + future debug tooling).
// @MX:REASON: contract-level integration with SPEC-LLM-001; behaviour
// changes (timeout, circuit-breaker handling) ripple through the public
// degradation policy.
// @MX:SPEC: SPEC-IR-001
func (r *Router) classifyByLLM(ctx context.Context, query string) (Category, float64, string, error) {
	if r.llmClient == nil {
		return CategoryUnknown, 0, "", ErrLLMUnavailable
	}
	return llmClassify(ctx, r.llmClient, query, r.llmModelOverride, r.llmDeadline)
}

// recordLLMFailure attaches the appropriate Metadata flag for the kind of LLM
// failure observed and emits the matching error_* outcome counter.
func (r *Router) recordLLMFailure(ctx context.Context, metadata map[string]any, err error, elapsed time.Duration) {
	switch {
	case errorsIs(err, ErrLLMTimeout):
		metadata["llm_timeout"] = true
		r.recordOutcome(ctx, OutcomeErrorTimeout, elapsed)
	case errorsIs(err, ErrLLMUnavailable):
		metadata["llm_unavailable"] = true
		r.recordOutcome(ctx, OutcomeErrorBreakerOpen, elapsed)
	case errorsIs(err, ErrLLMParse):
		metadata["llm_parse_error"] = err.Error()
		r.recordOutcome(ctx, OutcomeErrorParse, elapsed)
	default:
		metadata["llm_unavailable"] = true
		r.recordOutcome(ctx, OutcomeErrorBreakerOpen, elapsed)
	}
}

// errorsIs is a thin alias avoiding `errors.Is` import collision in this file.
func errorsIs(err, target error) bool { return errIs(err, target) }

// selectAdapterSet returns the lexicographically-sorted set of adapter names
// eligible to serve a decision in (cat, lang) per REQ-IR-008.
//
// Algorithm:
//  1. DocType filter: adapter.DocTypes ∩ CategoryEligibleDocTypes(cat) ≠ ∅
//  2. Lang filter: adapter.SupportedLangs contains lang OR is empty
//  3. If both pass, include the adapter
//  4. If the resulting set is empty, fall back to the lang-agnostic web set
//     and set Metadata.adapter_set_fallback = true
//  5. Sort lexicographically
func (r *Router) selectAdapterSet(cat Category, lang string, metadata map[string]any) []string {
	eligible := CategoryEligibleDocTypes(cat)
	eligibleSet := make(map[types.DocType]bool, len(eligible))
	for _, dt := range eligible {
		eligibleSet[dt] = true
	}

	matched := make([]string, 0, len(r.adapterNames))
	for _, name := range r.adapterNames {
		caps := r.caps[name]
		if !docTypeIntersects(caps.DocTypes, eligibleSet) {
			continue
		}
		if !langCompatible(caps.SupportedLangs, lang) {
			continue
		}
		matched = append(matched, name)
	}

	if len(matched) == 0 {
		// Fallback: lang-agnostic web set (adapters with empty SupportedLangs
		// AND DocType containing article or other).
		for _, name := range r.adapterNames {
			caps := r.caps[name]
			if len(caps.SupportedLangs) != 0 {
				continue
			}
			if hasAnyDocType(caps.DocTypes, types.DocTypeArticle, types.DocTypeOther) {
				matched = append(matched, name)
			}
		}
		metadata["adapter_set_fallback"] = true
	}

	sort.Strings(matched)
	return matched
}

func docTypeIntersects(have []types.DocType, eligible map[types.DocType]bool) bool {
	for _, dt := range have {
		if eligible[dt] {
			return true
		}
	}
	return false
}

func langCompatible(supported []string, lang string) bool {
	if len(supported) == 0 {
		return true
	}
	for _, s := range supported {
		if strings.EqualFold(s, lang) {
			return true
		}
	}
	return false
}

func hasAnyDocType(have []types.DocType, want ...types.DocType) bool {
	for _, h := range have {
		for _, w := range want {
			if h == w {
				return true
			}
		}
	}
	return false
}

// detectLang is a lightweight Hangul-ratio → BCP-47 mapper. Returns "ko" when
// the ratio is at or above RatioHigh; "en" otherwise.
func detectLang(hangul float64) string {
	if hangul >= RatioHigh {
		return "ko"
	}
	return "en"
}

// emit records the per-call observability tuple (counter, histogram, span,
// slog) for the given decision/error pair. Mirrors
// internal/llm/client.go::emitObservability and
// internal/adapters/registry.go::wrappedAdapter.emit.
func (r *Router) emit(ctx context.Context, dec RoutingDecision, err error, elapsed time.Duration) {
	span := oteltrace.SpanFromContext(ctx)
	outcome := OutcomeFromDecision(dec, err)
	span.SetAttributes(
		attribute.String("router.category", string(dec.Category)),
		attribute.String("router.source", string(dec.Source)),
		attribute.String("router.lang", dec.Lang),
		attribute.Int("router.adapter_count", len(dec.AdapterSet)),
		attribute.Float64("router.confidence", dec.Confidence),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, outcome)
	}

	r.recordOutcome(ctx, outcome, elapsed)

	if r.obs == nil || r.obs.Logger == nil {
		return
	}
	rid := reqid.FromContext(ctx)
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelWarn
	}
	llmUsed := dec.Source == SourceLLMFallback
	hangulRatio, _ := dec.Metadata["hangul_ratio"].(float64)
	attrs := []slog.Attr{
		slog.String("request_id", rid),
		slog.String("category", string(dec.Category)),
		slog.String("source", string(dec.Source)),
		slog.Float64("confidence", dec.Confidence),
		slog.String("lang", dec.Lang),
		slog.Int("adapter_count", len(dec.AdapterSet)),
		slog.Float64("hangul_ratio", hangulRatio),
		slog.Bool("llm_used", llmUsed),
		slog.String("outcome", outcome),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	r.obs.Logger.LogAttrs(ctx, level, "router classify", attrs...)
}

// recordOutcome increments the counter and observes the histogram at the
// given outcome label. Nil-safe per REQ-IR-006.
func (r *Router) recordOutcome(_ context.Context, outcome string, elapsed time.Duration) {
	if r.obs == nil {
		return
	}
	reg := r.obs.Metrics
	if reg == nil {
		return
	}
	if reg.RouterClassifications != nil {
		reg.RouterClassifications.WithLabelValues(outcome).Inc()
	}
	if reg.RouterClassificationDuration != nil {
		reg.RouterClassificationDuration.WithLabelValues(outcome).Observe(elapsed.Seconds())
	}
}

// tracer returns the OTel tracer for this Router. Falls back to the global
// no-op provider when obs is nil so spans still get created.
func (r *Router) tracer() oteltrace.Tracer {
	if r.obs == nil {
		return otel.Tracer("router")
	}
	return r.obs.Tracer("router")
}
